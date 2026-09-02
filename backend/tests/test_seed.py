"""Демо-данные: 19 сообщений из референса переносятся дословно."""
from datetime import datetime

from backend import seed_data
from backend.models import Connection, Message


def test_reference_set_is_complete():
    assert len(seed_data.MESSAGES) == 19
    assert sum(1 for item in seed_data.MESSAGES if item["src"] == "gmail") == 11
    assert sum(1 for item in seed_data.MESSAGES if item["src"] == "tg") == 8
    assert len(seed_data.LIVE_QUEUE) == 3


def test_seed_creates_user_connections_and_messages(db):
    assert seed_data.seed(db) == 19
    assert db.query(Message).count() == 19
    assert {c.kind: c.state for c in db.query(Connection)} == {
        "gmail": "active",
        "telegram": "reauth",
    }


def test_seed_is_idempotent(db):
    seed_data.seed(db)
    assert seed_data.seed(db) == 0
    assert db.query(Message).count() == 19


def test_seed_keeps_texts_verbatim(db):
    seed_data.seed(db)
    message = db.query(Message).filter_by(external_id="seed-1").one()
    assert message.subject == "Договор Northline — правки до конца дня"
    assert message.sender_name == "Анна Ковалёва"
    assert message.level == "CRITICAL"
    assert message.deadline_text == "Сегодня, 18:00"
    assert message.needs_reply and message.needs_action
    assert message.is_read is False


def test_group_chat_message_has_no_link(db):
    seed_data.seed(db)
    message = db.query(Message).filter_by(external_id="seed-12").one()
    assert message.sender_addr == "групповой чат, 9 участников"
    assert message.external_url == ""


def test_seeded_messages_are_analyzed(db):
    seed_data.seed(db)
    assert db.query(Message).filter_by(status="DONE").count() == 19


NOW = datetime(2026, 9, 2, 15, 0)  # среда


def test_parse_time_today():
    assert seed_data._parse_time("09:41", NOW) == datetime(2026, 9, 2, 9, 41)


def test_parse_time_yesterday():
    assert seed_data._parse_time("Вчера, 19:04", NOW) == datetime(2026, 9, 1, 19, 4)


def test_parse_time_weekday_looks_back():
    """«Пн» из среды — это позапрошедший понедельник той же недели."""
    assert seed_data._parse_time("Пн", NOW) == datetime(2026, 8, 31, 12, 0)
    assert seed_data._parse_time("Сб", NOW) == datetime(2026, 8, 29, 12, 0)


def test_parse_time_same_weekday_is_a_week_ago():
    assert seed_data._parse_time("Ср", NOW) == datetime(2026, 8, 26, 12, 0)


def test_parse_time_unknown_value():
    assert seed_data._parse_time("когда-то", NOW) == datetime(2026, 9, 2, 12, 0)


def test_demo_user_can_log_in(db, client):
    seed_data.seed(db)
    response = client.post(
        "/api/auth/login",
        json={"email": seed_data.DEMO_EMAIL, "password": seed_data.DEMO_PASSWORD},
    )
    assert response.status_code == 200
    assert client.get("/api/messages").json()["total"] == 19
