"""Приём сообщения из источника."""
from backend.events import bus
from backend.ingest import IncomingMessage, store
from backend.models import Message, utcnow


def _incoming(**kwargs) -> IncomingMessage:
    defaults = dict(
        external_id="msg-1",
        sender_name="Анна Ковалёва",
        sender_addr="a.kovaleva@northline.io",
        subject="Договор",
        body="Текст",
        received_at=utcnow(),
        external_url="https://mail.google.com/",
    )
    defaults.update(kwargs)
    return IncomingMessage(**defaults)


def test_store_creates_processing_message(db, gmail_connection, queued):
    message = store(db, gmail_connection, _incoming())
    assert message is not None
    assert message.status == "PROCESSING"
    assert message.is_read is False
    assert message.level is None
    assert queued == [message.id]


def test_store_publishes_created_event(db, gmail_connection):
    cursor = bus.cursor
    store(db, gmail_connection, _incoming())
    events, _ = bus.since(gmail_connection.user_id, cursor)
    assert [event.name for event in events] == ["message.created"]


def test_duplicate_external_id_is_skipped(db, gmail_connection):
    store(db, gmail_connection, _incoming())
    assert store(db, gmail_connection, _incoming()) is None
    assert db.query(Message).count() == 1


def test_same_external_id_in_other_connection_is_stored(
    db, gmail_connection, telegram_connection
):
    store(db, gmail_connection, _incoming())
    assert store(db, telegram_connection, _incoming()) is not None
    assert db.query(Message).count() == 2


def test_store_without_analysis(db, gmail_connection, queued):
    store(db, gmail_connection, _incoming(), analyze=False)
    assert queued == []
