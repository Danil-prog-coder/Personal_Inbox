"""Telegram через Bot API: проверка токена и long polling getUpdates.

Только Bot API — личный аккаунт через MTProto не используем (решение №4).
Следствие: бот видит только те чаты, куда его добавили, и историю до добавления
не отдаёт (решение №16)."""
import json
import logging
from datetime import datetime, timezone

import httpx
from sqlalchemy.orm import Session

from backend.ingest import IncomingMessage, store
from backend.models import Connection, utcnow

log = logging.getLogger(__name__)

API = "https://api.telegram.org/bot{token}/{method}"
TIMEOUT = 20.0


class TelegramError(RuntimeError):
    """Bot API вернул ошибку или недоступен."""


def call(token: str, method: str, **params) -> dict:
    try:
        response = httpx.post(API.format(token=token, method=method), json=params, timeout=TIMEOUT)
        payload = response.json()
    except (httpx.HTTPError, ValueError) as error:
        raise TelegramError(f"Telegram недоступен: {error}") from error
    if not payload.get("ok"):
        raise TelegramError(payload.get("description", "Telegram вернул ошибку"))
    return payload.get("result", {})


def verify_token(token: str) -> str:
    """Проверка через getMe. Возвращает @username бота — его показываем в UI."""
    bot = call(token, "getMe")
    username = bot.get("username")
    return f"@{username}" if username else bot.get("first_name", "бот")


def _chat_title(token: str, chat: dict, cache: dict | None = None) -> tuple[str, str]:
    """(имя отправителя, адрес). Для групп адрес — «групповой чат, N участников».
    cache хранит уже запрошенные счётчики: в одном чате обычно приходит пачка
    сообщений, и спрашивать Telegram про каждое незачем."""
    chat_type = chat.get("type", "private")
    if chat_type in ("group", "supergroup", "channel"):
        name = chat.get("title", "Групповой чат")
        cache = cache if cache is not None else {}
        chat_id = chat.get("id")
        if chat_id not in cache:
            try:
                count = call(token, "getChatMemberCount", chat_id=chat_id)
                cache[chat_id] = (
                    f"групповой чат, {count} участников"
                    if isinstance(count, int)
                    else "групповой чат"
                )
            except TelegramError:
                cache[chat_id] = "групповой чат"
        return name, cache[chat_id]
    name = " ".join(filter(None, [chat.get("first_name"), chat.get("last_name")])) or "Без имени"
    username = chat.get("username")
    return name, f"@{username}" if username else "личный чат"


def _external_url(chat: dict, message_id: int) -> str:
    """Прямая ссылка есть только у чатов с username; иначе кнопка не показывается."""
    username = chat.get("username")
    return f"https://t.me/{username}/{message_id}" if username else ""


def sync(db: Session, connection: Connection) -> int:
    """Забрать новые сообщения. Возвращает число сохранённых."""
    credentials = json.loads(connection.credentials or "{}")
    token = credentials.get("bot_token")
    if not token:
        connection.state = "reauth"
        db.commit()
        return 0

    offset = int(connection.sync_cursor) + 1 if connection.sync_cursor else None
    try:
        updates = call(token, "getUpdates", offset=offset, timeout=0, allowed_updates=["message"])
    except TelegramError as error:
        # Неверный токен — просим переподключить; сетевой сбой — просто ждём следующего цикла.
        log.warning("Telegram sync: %s", error)
        if "unauthorized" in str(error).lower():
            connection.state = "reauth"
            db.commit()
        return 0

    saved = 0
    last_update_id = None
    member_counts: dict[int, str] = {}
    for update in updates:
        last_update_id = update.get("update_id", last_update_id)
        message = update.get("message")
        if not message:
            continue
        text = message.get("text") or message.get("caption") or ""
        if not text:
            # Медиа без подписи оценивать нечем — пропускаем.
            continue
        chat = message.get("chat", {})
        sender_name, sender_addr = _chat_title(token, chat, member_counts)
        received_at = datetime.fromtimestamp(message.get("date", 0), tz=timezone.utc).replace(
            tzinfo=None
        )
        stored = store(
            db,
            connection,
            IncomingMessage(
                external_id=f"{chat.get('id')}:{message.get('message_id')}",
                sender_name=sender_name,
                sender_addr=sender_addr,
                # Для Telegram тема — первая строка сообщения (docs/03-data-model.md).
                subject=text.strip().splitlines()[0][:200],
                body=text,
                received_at=received_at,
                external_url=_external_url(chat, message.get("message_id", 0)),
            ),
        )
        if stored is not None:
            saved += 1

    if last_update_id is not None:
        connection.sync_cursor = str(last_update_id)
    connection.last_sync_at = utcnow()
    connection.state = "active"
    db.commit()
    return saved
