"""Лента уровня 2: список сообщений источника, детали, прочитано, ручной уровень."""
from fastapi import APIRouter, Depends, HTTPException, Query, status
from sqlalchemy.orm import Session

from backend.db import get_db
from backend.events import bus
from backend.models import Message, OverrideLog, User
from backend.queries import apply_filters, user_messages
from backend.schemas import LevelUpdate, MessageList, MessageOut
from backend.security import current_user
from backend.serializers import message_out

router = APIRouter(prefix="/api/messages", tags=["messages"])


def _get_owned(db: Session, user: User, message_id: int) -> Message:
    message = db.get(Message, message_id)
    if message is None or message.connection.user_id != user.id:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Сообщение не найдено")
    return message


@router.get("", response_model=MessageList)
def list_messages(
    source: str | None = Query(default=None, pattern="^(gmail|telegram)$"),
    level: str = Query(default="all", pattern="^(all|CRITICAL|HIGH|NORMAL|LOW)$"),
    status_filter: str = Query(default="all", alias="status", pattern="^(all|unread|read|done)$"),
    reply: str = Query(default="all", pattern="^(all|yes|no)$"),
    action: str = Query(default="all", pattern="^(all|yes|no)$"),
    period: str = Query(default="all", pattern="^(all|today|week|month)$"),
    q: str | None = None,
    tz_offset: int = Query(default=0, description="смещение часового пояса клиента, минуты"),
    user: User = Depends(current_user),
    db: Session = Depends(get_db),
) -> MessageList:
    stmt = apply_filters(
        user_messages(user),
        source=source,
        level=level,
        status=status_filter,
        reply=reply,
        action=action,
        period=period,
        q=q,
        tz_offset=tz_offset,
    )
    messages = db.execute(
        stmt.order_by(Message.received_at.desc(), Message.id.desc())
    ).scalars().all()
    unread = sum(1 for message in messages if not message.is_read)
    return MessageList(
        items=[message_out(message) for message in messages],
        total=len(messages),
        unread=unread,
    )


@router.get("/{message_id}", response_model=MessageOut)
def get_message(
    message_id: int,
    user: User = Depends(current_user),
    db: Session = Depends(get_db),
) -> MessageOut:
    return message_out(_get_owned(db, user, message_id))


@router.post("/{message_id}/read", response_model=MessageOut)
def mark_read(
    message_id: int,
    user: User = Depends(current_user),
    db: Session = Depends(get_db),
) -> MessageOut:
    """Помечается при открытии деталей. Обратной операции в MVP нет (решение №12)."""
    message = _get_owned(db, user, message_id)
    if not message.is_read:
        message.is_read = True
        db.commit()
        db.refresh(message)
    return message_out(message)


@router.post("/{message_id}/level", response_model=MessageOut)
def set_level(
    message_id: int,
    payload: LevelUpdate,
    user: User = Depends(current_user),
    db: Session = Depends(get_db),
) -> MessageOut:
    """Ручное исправление уровня — единственная точка правки оценки."""
    message = _get_owned(db, user, message_id)
    previous = message.effective_level
    if payload.level != previous:
        db.add(
            OverrideLog(message_id=message.id, from_level=previous, to_level=payload.level)
        )
    message.level_override = payload.level
    db.commit()
    db.refresh(message)
    bus.publish(user.id, "message.analyzed", message_out(message).model_dump(mode="json"))
    return message_out(message)
