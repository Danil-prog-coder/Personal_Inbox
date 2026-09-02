"""Экран «Источники»: подключение, состояние, отключение."""
import json
import secrets

from fastapi import APIRouter, Depends, HTTPException, Request, status
from fastapi.responses import RedirectResponse
from sqlalchemy import select
from sqlalchemy.orm import Session

from backend import config
from backend.db import get_db
from backend.models import SOURCE_KINDS, Connection, User, utcnow
from backend.schemas import AuthUrl, ConnectionOut, TelegramConnect
from backend.security import current_user
from backend.sources import gmail, telegram

router = APIRouter(prefix="/api/connections", tags=["connections"])

OAUTH_STATE_KEY = "gmail_oauth_state"


def _get_or_create(db: Session, user: User, kind: str) -> Connection:
    connection = db.execute(
        select(Connection).where(Connection.user_id == user.id, Connection.kind == kind)
    ).scalar_one_or_none()
    if connection is None:
        connection = Connection(user_id=user.id, kind=kind, state="off", account="", credentials="")
        db.add(connection)
        db.commit()
        db.refresh(connection)
    return connection


@router.get("", response_model=list[ConnectionOut])
def list_connections(
    user: User = Depends(current_user),
    db: Session = Depends(get_db),
) -> list[ConnectionOut]:
    """Оба источника всегда в списке: неподключённый показывается как off."""
    existing = {
        connection.kind: connection
        for connection in db.execute(
            select(Connection).where(Connection.user_id == user.id)
        ).scalars()
    }
    result = []
    for kind in SOURCE_KINDS:
        connection = existing.get(kind)
        result.append(
            ConnectionOut(
                kind=kind,
                state=connection.state if connection else "off",
                account=connection.account if connection else "",
                last_sync_at=connection.last_sync_at if connection else None,
            )
        )
    return result


@router.post("/gmail/start", response_model=AuthUrl)
def gmail_start(request: Request, user: User = Depends(current_user)) -> AuthUrl:
    state = secrets.token_urlsafe(16)
    request.session[OAUTH_STATE_KEY] = state
    try:
        return AuthUrl(auth_url=gmail.auth_url(state))
    except gmail.GmailError as error:
        raise HTTPException(status_code=status.HTTP_503_SERVICE_UNAVAILABLE, detail=str(error))


@router.get("/gmail/callback")
def gmail_callback(
    request: Request,
    code: str | None = None,
    state: str | None = None,
    db: Session = Depends(get_db),
    user: User = Depends(current_user),
) -> RedirectResponse:
    expected = request.session.pop(OAUTH_STATE_KEY, None)
    if not code or not state or state != expected:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST, detail="Авторизация Google не подтверждена"
        )
    try:
        email, refresh_token = gmail.exchange_code(code)
    except gmail.GmailError as error:
        raise HTTPException(status_code=status.HTTP_502_BAD_GATEWAY, detail=str(error))

    connection = _get_or_create(db, user, "gmail")
    connection.account = email
    connection.credentials = json.dumps({"refresh_token": refresh_token})
    connection.state = "active"
    connection.sync_cursor = None  # новое подключение — импорт за 30 дней
    connection.last_sync_at = None
    db.commit()
    return RedirectResponse(url=f"{config.FRONTEND_ORIGIN}/connections?connected=gmail")


@router.post("/telegram", response_model=ConnectionOut)
def connect_telegram(
    payload: TelegramConnect,
    user: User = Depends(current_user),
    db: Session = Depends(get_db),
) -> ConnectionOut:
    """Токен проверяется через getMe — пользователю показываем имя бота."""
    try:
        bot_name = telegram.verify_token(payload.bot_token.strip())
    except telegram.TelegramError as error:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(error))

    connection = _get_or_create(db, user, "telegram")
    connection.account = bot_name
    connection.credentials = json.dumps({"bot_token": payload.bot_token.strip()})
    connection.state = "active"
    connection.last_sync_at = utcnow()
    db.commit()
    db.refresh(connection)
    return ConnectionOut(
        kind=connection.kind,
        state=connection.state,
        account=connection.account,
        last_sync_at=connection.last_sync_at,
    )


@router.delete("/{kind}", status_code=status.HTTP_204_NO_CONTENT)
def disconnect(
    kind: str,
    user: User = Depends(current_user),
    db: Session = Depends(get_db),
):
    """Отключение обнуляет доступы, но не удаляет сообщения — они остаются в ленте."""
    if kind not in SOURCE_KINDS:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Неизвестный источник")
    connection = db.execute(
        select(Connection).where(Connection.user_id == user.id, Connection.kind == kind)
    ).scalar_one_or_none()
    if connection is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Источник не подключён")
    connection.state = "off"
    connection.credentials = ""
    db.commit()
    return None
