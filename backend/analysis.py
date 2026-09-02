"""Оценка сообщений моделью: очередь, повторы, фоновая переоценка.

Один рабочий поток на процесс — этого достаточно (docs/04-decisions.md, решение №2).
Сообщение никогда не пропадает из ленты из-за ошибки модели: после трёх неудач
оно закрывается как NORMAL с пометкой «Оценка недоступна»."""
import logging
import queue
import threading
import time

from sqlalchemy import select
from sqlalchemy.orm import Session

from backend import config
from backend.db import SessionLocal
from backend.events import bus
from backend.llm.provider import AnalysisRequest, LLMUnavailable, analyze
from backend.models import Connection, Message, OverrideLog, User, utcnow
from backend.serializers import message_out

log = logging.getLogger(__name__)

_queue: "queue.Queue[int]" = queue.Queue()
_worker: threading.Thread | None = None
_worker_lock = threading.Lock()


def enqueue(message_id: int) -> None:
    """Поставить сообщение в очередь на оценку."""
    _queue.put(message_id)
    ensure_worker()


def ensure_worker() -> None:
    global _worker
    with _worker_lock:
        if _worker is None or not _worker.is_alive():
            _worker = threading.Thread(target=_run, name="analysis-worker", daemon=True)
            _worker.start()


def _run() -> None:  # pragma: no cover - цикл потока, логика в process_message
    while True:
        message_id = _queue.get()
        try:
            with SessionLocal() as db:
                process_message(db, message_id)
        except Exception:
            log.exception("Не удалось оценить сообщение %s", message_id)
        finally:
            _queue.task_done()


def recent_overrides(db: Session, user: User, limit: int | None = None) -> list[tuple[str, str]]:
    """Последние ручные исправления пользователя — обратная связь для модели."""
    limit = limit or config.LLM_OVERRIDE_HISTORY
    rows = db.execute(
        select(Message.subject, OverrideLog.to_level)
        .join(Message, Message.id == OverrideLog.message_id)
        .join(Connection, Connection.id == Message.connection_id)
        .where(Connection.user_id == user.id)
        .order_by(OverrideLog.created_at.desc(), OverrideLog.id.desc())
        .limit(limit)
    ).all()
    return [(subject, level) for subject, level in rows]


def build_request(db: Session, message: Message) -> AnalysisRequest:
    user = message.connection.user
    sender = f"{message.sender_name} <{message.sender_addr}>".strip()
    return AnalysisRequest(
        criteria=user.criteria or "",
        sender=sender,
        subject=message.subject,
        body=message.body,
        source=message.connection.kind,
        overrides=recent_overrides(db, user),
    )


def process_message(db: Session, message_id: int, sleep=time.sleep) -> Message | None:
    """Оценить одно сообщение. Три повтора с задержками 2с / 8с / 30с."""
    message = db.get(Message, message_id)
    if message is None:
        return None

    request = build_request(db, message)
    result = None
    last_error: Exception | None = None
    for attempt, delay in enumerate(config.LLM_RETRY_DELAYS):
        try:
            result = analyze(request)
            break
        except LLMUnavailable as error:
            last_error = error
            log.warning("Модель не ответила (попытка %s): %s", attempt + 1, error)
            if attempt < len(config.LLM_RETRY_DELAYS) - 1:
                sleep(delay)

    if result is None:
        log.error("Оценка недоступна для сообщения %s: %s", message_id, last_error)
        # У сообщения, которое уже оценивалось, прежнюю оценку не стираем:
        # при переоценке потерять готовую карточку хуже, чем показать старую.
        if message.analyzed_at is None:
            message.level = "NORMAL"
            message.category = ""
            message.deadline_text = ""
            message.summary = ""
            message.needs_reply = False
            message.needs_action = False
        message.status = "DONE"
        message.analysis_failed = True
    else:
        message.status = "DONE"
        message.level = result.level
        message.category = result.category
        message.deadline_text = result.deadline
        message.needs_reply = result.needs_reply
        message.needs_action = result.needs_action
        message.summary = result.summary
        message.analysis_failed = False
    message.analyzed_at = utcnow()
    db.commit()
    db.refresh(message)
    bus.publish(
        message.connection.user_id,
        "message.analyzed",
        message_out(message).model_dump(mode="json"),
    )
    return message


def queue_reanalysis(db: Session, user: User) -> int:
    """Смена критериев: все сообщения пользователя уходят на переоценку.
    Ручные исправления (level_override) при этом не трогаем — решение №15."""
    messages = db.execute(
        select(Message)
        .join(Connection)
        .where(Connection.user_id == user.id)
        .order_by(Message.received_at.desc())
    ).scalars().all()
    for message in messages:
        message.status = "PROCESSING"
    db.commit()
    for message in messages:
        enqueue(message.id)
    return len(messages)
