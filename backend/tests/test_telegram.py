"""Telegram Bot API: разбор апдейтов, курсор, реакция на ошибки. Сеть замокана."""
import json

import pytest

from backend.models import Message
from backend.sources import telegram


def _update(update_id: int, chat: dict, text: str = "Привет", message_id: int = 10) -> dict:
    return {
        "update_id": update_id,
        "message": {
            "message_id": message_id,
            "date": 1_756_000_000,
            "chat": chat,
            "text": text,
        },
    }


@pytest.fixture
def fake_call(monkeypatch):
    """Подменяет вызовы Bot API. calls хранит историю обращений."""
    calls: list[tuple[str, dict]] = []
    responses: dict[str, object] = {}

    def _call(_token, method, **params):
        calls.append((method, params))
        if method not in responses:
            raise telegram.TelegramError(f"нет заглушки для {method}")
        value = responses[method]
        if isinstance(value, Exception):
            raise value
        return value

    monkeypatch.setattr(telegram, "call", _call)
    return type("Fake", (), {"calls": calls, "responses": responses})()


def test_verify_token_returns_bot_username(fake_call):
    fake_call.responses["getMe"] = {"username": "inbox_bot", "first_name": "Inbox"}
    assert telegram.verify_token("123:abc") == "@inbox_bot"


def test_verify_token_falls_back_to_first_name(fake_call):
    fake_call.responses["getMe"] = {"first_name": "Inbox"}
    assert telegram.verify_token("123:abc") == "Inbox"


def test_sync_stores_private_message(db, telegram_connection, fake_call, queued):
    fake_call.responses["getUpdates"] = [
        _update(101, {"id": 55, "type": "private", "first_name": "Дима", "username": "dmitry_pm"})
    ]
    assert telegram.sync(db, telegram_connection) == 1

    message = db.query(Message).one()
    assert message.external_id == "55:10"
    assert message.sender_name == "Дима"
    assert message.sender_addr == "@dmitry_pm"
    assert message.external_url == "https://t.me/dmitry_pm/10"
    assert telegram_connection.sync_cursor == "101"
    assert telegram_connection.last_sync_at is not None


def test_group_chat_shows_member_count(db, telegram_connection, fake_call):
    fake_call.responses["getUpdates"] = [
        _update(1, {"id": -100, "type": "group", "title": "Инвесторы · Seed round"})
    ]
    fake_call.responses["getChatMemberCount"] = 9
    telegram.sync(db, telegram_connection)

    message = db.query(Message).one()
    assert message.sender_name == "Инвесторы · Seed round"
    assert message.sender_addr == "групповой чат, 9 участников"
    # У чата без username прямой ссылки нет — кнопка «Открыть» не показывается.
    assert message.external_url == ""


def test_group_without_member_count_degrades_gracefully(db, telegram_connection, fake_call):
    fake_call.responses["getUpdates"] = [_update(1, {"id": -100, "type": "group", "title": "Чат"})]
    fake_call.responses["getChatMemberCount"] = telegram.TelegramError("нет прав")
    telegram.sync(db, telegram_connection)
    assert db.query(Message).one().sender_addr == "групповой чат"


def test_subject_is_first_line(db, telegram_connection, fake_call):
    fake_call.responses["getUpdates"] = [
        _update(1, {"id": 5, "type": "private", "first_name": "Лена"}, text="Первая строка\nвторая")
    ]
    telegram.sync(db, telegram_connection)
    message = db.query(Message).one()
    assert message.subject == "Первая строка"
    assert message.body == "Первая строка\nвторая"


def test_message_without_text_is_skipped(db, telegram_connection, fake_call):
    update = _update(1, {"id": 5, "type": "private", "first_name": "Лена"})
    del update["message"]["text"]
    fake_call.responses["getUpdates"] = [update]
    assert telegram.sync(db, telegram_connection) == 0
    # Курсор всё равно сдвигается, иначе апдейт будет приходить вечно.
    assert telegram_connection.sync_cursor == "1"


def test_next_sync_asks_for_updates_after_cursor(db, telegram_connection, fake_call):
    telegram_connection.sync_cursor = "77"
    db.commit()
    fake_call.responses["getUpdates"] = []
    telegram.sync(db, telegram_connection)
    assert fake_call.calls[0][1]["offset"] == 78


def test_invalid_token_switches_to_reauth(db, telegram_connection, fake_call):
    fake_call.responses["getUpdates"] = telegram.TelegramError("Unauthorized")
    assert telegram.sync(db, telegram_connection) == 0
    assert telegram_connection.state == "reauth"


def test_network_error_keeps_connection_active(db, telegram_connection, fake_call):
    fake_call.responses["getUpdates"] = telegram.TelegramError("Telegram недоступен: таймаут")
    assert telegram.sync(db, telegram_connection) == 0
    assert telegram_connection.state == "active"


def test_missing_token_switches_to_reauth(db, telegram_connection):
    telegram_connection.credentials = json.dumps({})
    db.commit()
    assert telegram.sync(db, telegram_connection) == 0
    assert telegram_connection.state == "reauth"


def test_member_count_is_requested_once_per_chat(db, telegram_connection, fake_call):
    chat = {"id": -100, "type": "group", "title": "Чат"}
    fake_call.responses["getUpdates"] = [
        _update(1, chat, message_id=1),
        _update(2, chat, message_id=2),
    ]
    fake_call.responses["getChatMemberCount"] = 5
    assert telegram.sync(db, telegram_connection) == 2
    assert sum(1 for method, _ in fake_call.calls if method == "getChatMemberCount") == 1
