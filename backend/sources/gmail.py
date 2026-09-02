"""Gmail через Google OAuth 2.0 и Gmail API, доступ только на чтение.

Первый импорт — письма за 30 дней, не больше 200 (решение №16).
Дальше инкрементально по historyId. Истёкший или отозванный токен переводит
подключение в reauth, ранее полученные сообщения остаются в ленте."""
import base64
import json
import logging
from datetime import datetime, timedelta, timezone
from email.utils import parseaddr, parsedate_to_datetime

from sqlalchemy.orm import Session

from backend import config
from backend.ingest import IncomingMessage, store
from backend.models import Connection, utcnow

log = logging.getLogger(__name__)

SCOPES = ["https://www.googleapis.com/auth/gmail.readonly"]
MESSAGE_URL = "https://mail.google.com/mail/u/0/#inbox/{id}"
PAGE_SIZE = 100


class GmailError(RuntimeError):
    """Google не отвечает или отказал в доступе."""


def _client_config() -> dict:
    if not config.GOOGLE_CLIENT_ID or not config.GOOGLE_CLIENT_SECRET:
        raise GmailError("GOOGLE_CLIENT_ID и GOOGLE_CLIENT_SECRET не заданы")
    return {
        "web": {
            "client_id": config.GOOGLE_CLIENT_ID,
            "client_secret": config.GOOGLE_CLIENT_SECRET,
            "auth_uri": "https://accounts.google.com/o/oauth2/auth",
            "token_uri": "https://oauth2.googleapis.com/token",
            "redirect_uris": [config.GOOGLE_REDIRECT_URI],
        }
    }


def _flow(state: str | None = None):
    from google_auth_oauthlib.flow import Flow

    flow = Flow.from_client_config(_client_config(), scopes=SCOPES, state=state)
    flow.redirect_uri = config.GOOGLE_REDIRECT_URI
    return flow


def auth_url(state: str) -> str:
    """URL согласия Google. access_type=offline — нужен refresh_token."""
    url, _ = _flow(state).authorization_url(
        access_type="offline", include_granted_scopes="true", prompt="consent"
    )
    return url


def exchange_code(code: str) -> tuple[str, str]:
    """Обмен кода на токены. Возвращает (email аккаунта, refresh_token)."""
    flow = _flow()
    try:
        flow.fetch_token(code=code)
    except Exception as error:
        raise GmailError(f"Google отказал в обмене кода: {error}") from error
    credentials = flow.credentials
    if not credentials.refresh_token:
        raise GmailError("Google не выдал refresh_token — переподключите доступ")
    email = _profile_email(credentials)
    return email, credentials.refresh_token


def _credentials(refresh_token: str):
    from google.oauth2.credentials import Credentials

    return Credentials(
        token=None,
        refresh_token=refresh_token,
        token_uri="https://oauth2.googleapis.com/token",
        client_id=config.GOOGLE_CLIENT_ID,
        client_secret=config.GOOGLE_CLIENT_SECRET,
        scopes=SCOPES,
    )


def _service(credentials):
    from googleapiclient.discovery import build

    return build("gmail", "v1", credentials=credentials, cache_discovery=False)


def _profile_email(credentials) -> str:
    profile = _service(credentials).users().getProfile(userId="me").execute()
    return profile.get("emailAddress", "")


def _header(payload: dict, name: str) -> str:
    for header in payload.get("headers", []):
        if header.get("name", "").lower() == name.lower():
            return header.get("value", "")
    return ""


def _body_text(payload: dict) -> str:
    """Первый текстовый кусок письма. HTML-часть берём только если plain нет."""
    mime = payload.get("mimeType", "")
    data = payload.get("body", {}).get("data")
    if data and mime == "text/plain":
        return _decode(data)
    for part in payload.get("parts", []) or []:
        text = _body_text(part)
        if text:
            return text
    if data and mime == "text/html":
        return _strip_html(_decode(data))
    return ""


def _decode(data: str) -> str:
    try:
        return base64.urlsafe_b64decode(data.encode("utf-8")).decode("utf-8", errors="replace")
    except (ValueError, TypeError):
        return ""


def _strip_html(html: str) -> str:
    import re

    text = re.sub(r"<(script|style)[^>]*>.*?</\1>", " ", html, flags=re.S | re.I)
    text = re.sub(r"<[^>]+>", " ", text)
    return re.sub(r"\s+", " ", text).strip()


def _received_at(raw: dict, payload: dict) -> datetime:
    internal = raw.get("internalDate")
    if internal:
        return datetime.fromtimestamp(int(internal) / 1000, tz=timezone.utc).replace(tzinfo=None)
    try:
        parsed = parsedate_to_datetime(_header(payload, "Date"))
        return parsed.astimezone(timezone.utc).replace(tzinfo=None) if parsed else utcnow()
    except (TypeError, ValueError):
        return utcnow()


def _to_incoming(raw: dict) -> IncomingMessage:
    payload = raw.get("payload", {})
    sender_name, sender_addr = parseaddr(_header(payload, "From"))
    return IncomingMessage(
        external_id=raw.get("id", ""),
        sender_name=sender_name or sender_addr or "Без отправителя",
        sender_addr=sender_addr,
        subject=_header(payload, "Subject") or "(без темы)",
        body=_body_text(payload) or raw.get("snippet", ""),
        received_at=_received_at(raw, payload),
        external_url=MESSAGE_URL.format(id=raw.get("id", "")),
    )


def sync(db: Session, connection: Connection) -> int:
    """Синхронизация одного подключения. Возвращает число новых сообщений."""
    credentials_data = json.loads(connection.credentials or "{}")
    refresh_token = credentials_data.get("refresh_token")
    if not refresh_token:
        connection.state = "reauth"
        db.commit()
        return 0

    try:
        credentials = _credentials(refresh_token)
        service = _service(credentials)
        ids, cursor = _collect_ids(service, connection.sync_cursor)
        saved = 0
        for message_id in ids:
            raw = (
                service.users()
                .messages()
                .get(userId="me", id=message_id, format="full")
                .execute()
            )
            if store(db, connection, _to_incoming(raw)) is not None:
                saved += 1
        if cursor is None:
            profile = service.users().getProfile(userId="me").execute()
            cursor = str(profile.get("historyId", ""))
        connection.sync_cursor = cursor or connection.sync_cursor
        connection.last_sync_at = utcnow()
        connection.state = "active"
        db.commit()
        return saved
    except Exception as error:
        log.warning("Gmail sync: %s", error)
        if _is_auth_error(error):
            connection.state = "reauth"
            db.commit()
        return 0


def _collect_ids(service, sync_cursor: str | None) -> tuple[list[str], str | None]:
    """Что забирать: инкремент по historyId либо первый импорт за 30 дней."""
    if not sync_cursor:
        return _initial_ids(service)
    try:
        return _incremental_ids(service, sync_cursor)
    except Exception as error:
        if not _is_stale_history(error):
            raise
        # Google хранит историю ограниченное время: протухший historyId —
        # не ошибка, а повод импортировать заново.
        log.info("historyId устарел, импортируем письма заново")
        return _initial_ids(service)


def _is_stale_history(error: Exception) -> bool:
    text = str(error).lower()
    return "404" in text or "not found" in text or "starthistoryid" in text


def _is_auth_error(error: Exception) -> bool:
    text = str(error).lower()
    return any(marker in text for marker in ("invalid_grant", "unauthorized", "401", "403"))


def _initial_ids(service) -> tuple[list[str], str | None]:
    """Первый импорт: письма за 30 дней, максимум 200, самые новые первыми."""
    since = datetime.now(timezone.utc) - timedelta(days=config.GMAIL_IMPORT_DAYS)
    after = since.strftime("%Y/%m/%d")
    ids: list[str] = []
    page_token = None
    while len(ids) < config.GMAIL_IMPORT_LIMIT:
        response = (
            service.users()
            .messages()
            .list(
                userId="me",
                q=f"after:{after}",
                maxResults=min(PAGE_SIZE, config.GMAIL_IMPORT_LIMIT - len(ids)),
                pageToken=page_token,
            )
            .execute()
        )
        ids += [item["id"] for item in response.get("messages", [])]
        page_token = response.get("nextPageToken")
        if not page_token:
            break
    return ids[: config.GMAIL_IMPORT_LIMIT], None


def _incremental_ids(service, start_history_id: str) -> tuple[list[str], str | None]:
    """Инкрементально по historyId. Протухший historyId — импорт с нуля."""
    ids: list[str] = []
    page_token = None
    cursor = start_history_id
    while True:
        response = (
            service.users()
            .history()
            .list(
                userId="me",
                startHistoryId=start_history_id,
                historyTypes=["messageAdded"],
                pageToken=page_token,
            )
            .execute()
        )
        for record in response.get("history", []):
            for added in record.get("messagesAdded", []):
                message_id = added.get("message", {}).get("id")
                if message_id and message_id not in ids:
                    ids.append(message_id)
        cursor = str(response.get("historyId", cursor))
        page_token = response.get("nextPageToken")
        if not page_token:
            break
    return ids, cursor
