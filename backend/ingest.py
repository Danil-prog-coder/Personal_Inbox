"""Приём сообщения из источника: дедупликация, событие в ленту, очередь на оценку."""
import logging
from dataclasses import dataclass
from datetime import datetime

from sqlalchemy import select
from sqlalchemy.orm import Session

from backend import analysis
from backend.events import bus
from backend.models import Connection, Message
from backend.serializers import message_out

log = logging.getLogger(__name__)


@dataclass
class IncomingMessage:
    """То, что источник знает о сообщении до оценки моделью."""

    external_id: str
    sender_name: str
    sender_addr: str
    subject: str
    body: str
    received_at: datetime
    external_url: str = ""


def store(
    db: Session,
    connection: Connection,
    incoming: IncomingMessage,
    *,
    analyze: bool = True,
) -> Message | None:
    """Сохранить входящее сообщение. Дубль (тот же external_id) — вернуть None."""
    existing = db.execute(
        select(Message).where(
            Message.connection_id == connection.id,
            Message.external_id == incoming.external_id,
        )
    ).scalar_one_or_none()
    if existing is not None:
        return None

    message = Message(
        connection_id=connection.id,
        external_id=incoming.external_id,
        sender_name=incoming.sender_name,
        sender_addr=incoming.sender_addr,
        subject=incoming.subject,
        body=incoming.body,
        received_at=incoming.received_at,
        external_url=incoming.external_url,
        status="PROCESSING",
        is_read=False,
    )
    db.add(message)
    db.commit()
    db.refresh(message)

    # Карточка появляется в ленте сразу, с индикатором «Определяем важность…».
    bus.publish(
        connection.user_id, "message.created", message_out(message).model_dump(mode="json")
    )
    if analyze:
        analysis.enqueue(message.id)
    return message
