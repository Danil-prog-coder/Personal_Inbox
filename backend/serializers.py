"""Перевод моделей в схемы ответа. Один формат сообщения на все ручки и на SSE."""
from backend.models import Message
from backend.schemas import MessageBrief, MessageOut


def message_out(message: Message) -> MessageOut:
    return MessageOut(
        id=message.id,
        source=message.connection.kind,
        external_id=message.external_id,
        sender_name=message.sender_name,
        sender_addr=message.sender_addr,
        subject=message.subject,
        body=message.body,
        received_at=message.received_at,
        is_read=message.is_read,
        status=message.status,
        level=message.effective_level,
        level_override=message.level_override,
        category=message.category,
        deadline_text=message.deadline_text,
        needs_reply=message.needs_reply,
        needs_action=message.needs_action,
        summary=message.summary,
        external_url=message.external_url,
        analyzed_at=message.analyzed_at,
        analysis_failed=message.analysis_failed,
    )


def message_brief(message: Message) -> MessageBrief:
    return MessageBrief(
        id=message.id,
        sender_name=message.sender_name,
        subject=message.subject,
        level=message.effective_level,
    )
