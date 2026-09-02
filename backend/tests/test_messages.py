"""Лента уровня 2: фильтры, поиск, прочитано, ручное исправление уровня."""
from datetime import timedelta

from backend.models import Message, OverrideLog, utcnow


def test_list_returns_newest_first(auth_client, gmail_connection, make_message):
    old = make_message(gmail_connection, received_at=utcnow() - timedelta(days=2))
    new = make_message(gmail_connection, received_at=utcnow())
    ids = [item["id"] for item in auth_client.get("/api/messages").json()["items"]]
    assert ids == [new.id, old.id]


def test_list_counts_unread(auth_client, gmail_connection, make_message):
    make_message(gmail_connection, is_read=False)
    make_message(gmail_connection, is_read=True)
    body = auth_client.get("/api/messages").json()
    assert (body["total"], body["unread"]) == (2, 1)


def test_filter_by_source(auth_client, gmail_connection, telegram_connection, make_message):
    make_message(gmail_connection)
    make_message(telegram_connection)
    body = auth_client.get("/api/messages", params={"source": "telegram"}).json()
    assert body["total"] == 1
    assert body["items"][0]["source"] == "telegram"


def test_filter_by_level_uses_override(auth_client, gmail_connection, make_message):
    """Ручное исправление важнее оценки модели — фильтр обязан это учитывать."""
    make_message(gmail_connection, level="LOW", level_override="CRITICAL")
    make_message(gmail_connection, level="CRITICAL", level_override="LOW")
    body = auth_client.get("/api/messages", params={"level": "CRITICAL"}).json()
    assert body["total"] == 1
    assert body["items"][0]["level"] == "CRITICAL"


def test_filter_by_status(auth_client, gmail_connection, make_message):
    make_message(gmail_connection, is_read=False, status="PROCESSING")
    make_message(gmail_connection, is_read=True, status="DONE")
    assert auth_client.get("/api/messages", params={"status": "unread"}).json()["total"] == 1
    assert auth_client.get("/api/messages", params={"status": "read"}).json()["total"] == 1
    # «Обработано» = прошло оценку моделью, а не действие пользователя.
    assert auth_client.get("/api/messages", params={"status": "done"}).json()["total"] == 1


def test_filter_by_reply_and_action(auth_client, gmail_connection, make_message):
    make_message(gmail_connection, needs_reply=True, needs_action=False)
    make_message(gmail_connection, needs_reply=False, needs_action=True)
    assert auth_client.get("/api/messages", params={"reply": "yes"}).json()["total"] == 1
    assert auth_client.get("/api/messages", params={"reply": "no"}).json()["total"] == 1
    assert auth_client.get("/api/messages", params={"action": "yes"}).json()["total"] == 1


def test_filter_by_period(auth_client, gmail_connection, make_message):
    make_message(gmail_connection, received_at=utcnow() - timedelta(hours=2))
    make_message(gmail_connection, received_at=utcnow() - timedelta(days=3))
    make_message(gmail_connection, received_at=utcnow() - timedelta(days=45))
    assert auth_client.get("/api/messages", params={"period": "all"}).json()["total"] == 3
    assert auth_client.get("/api/messages", params={"period": "week"}).json()["total"] == 2
    assert auth_client.get("/api/messages", params={"period": "month"}).json()["total"] == 2
    today = auth_client.get("/api/messages", params={"period": "today", "tz_offset": 0}).json()
    assert today["total"] <= 1


def test_search_covers_sender_subject_and_body(auth_client, gmail_connection, make_message):
    make_message(gmail_connection, sender_name="Анна Ковалёва", subject="Договор", body="текст")
    make_message(gmail_connection, sender_name="Пётр", subject="Отчёт", body="Про договор внутри")
    make_message(gmail_connection, sender_name="Игорь", subject="Обед", body="ничего")
    assert auth_client.get("/api/messages", params={"q": "договор"}).json()["total"] == 2
    assert auth_client.get("/api/messages", params={"q": "АННА"}).json()["total"] == 1
    assert auth_client.get("/api/messages", params={"q": "нет такого"}).json()["total"] == 0


def test_filters_combine(auth_client, gmail_connection, make_message):
    make_message(gmail_connection, level="CRITICAL", needs_reply=True, subject="Договор")
    make_message(gmail_connection, level="CRITICAL", needs_reply=False, subject="Договор")
    body = auth_client.get(
        "/api/messages", params={"level": "CRITICAL", "reply": "yes", "q": "договор"}
    ).json()
    assert body["total"] == 1


def test_get_message_details(auth_client, gmail_connection, make_message):
    message = make_message(gmail_connection, body="Полный текст письма")
    body = auth_client.get(f"/api/messages/{message.id}").json()
    assert body["body"] == "Полный текст письма"
    assert body["source"] == "gmail"


def test_get_message_of_another_user_is_404(client, db, gmail_connection, make_message):
    """Чужое сообщение не должно отличаться от несуществующего."""
    from backend.models import User
    from backend.security import hash_password

    other = User(email="other@northline.io", password_hash=hash_password("qwerty12345"))
    db.add(other)
    db.commit()
    message = make_message(gmail_connection)
    client.post("/api/auth/login", json={"email": other.email, "password": "qwerty12345"})
    assert client.get(f"/api/messages/{message.id}").status_code == 404


def test_mark_read_is_idempotent(auth_client, gmail_connection, make_message):
    message = make_message(gmail_connection, is_read=False)
    assert auth_client.post(f"/api/messages/{message.id}/read").json()["is_read"] is True
    assert auth_client.post(f"/api/messages/{message.id}/read").json()["is_read"] is True


def test_set_level_writes_override_log(auth_client, db, gmail_connection, make_message):
    message = make_message(gmail_connection, level="NORMAL")
    body = auth_client.post(f"/api/messages/{message.id}/level", json={"level": "CRITICAL"}).json()
    assert body["level"] == "CRITICAL"
    assert body["level_override"] == "CRITICAL"

    logs = db.query(OverrideLog).filter_by(message_id=message.id).all()
    assert [(log.from_level, log.to_level) for log in logs] == [("NORMAL", "CRITICAL")]
    # Оценка модели остаётся нетронутой — переопределён только показываемый уровень.
    assert db.get(Message, message.id).level == "NORMAL"


def test_set_same_level_does_not_duplicate_log(auth_client, db, gmail_connection, make_message):
    message = make_message(gmail_connection, level="HIGH")
    auth_client.post(f"/api/messages/{message.id}/level", json={"level": "HIGH"})
    assert db.query(OverrideLog).count() == 0


def test_set_level_rejects_unknown_value(auth_client, gmail_connection, make_message):
    message = make_message(gmail_connection)
    assert auth_client.post(
        f"/api/messages/{message.id}/level", json={"level": "URGENT"}
    ).status_code == 422


def test_unknown_filter_value_is_rejected(auth_client):
    """Опечатка в фильтре должна давать ошибку, а не молча возвращать всё."""
    assert auth_client.get("/api/messages", params={"level": "КРИТИЧНО"}).status_code == 422
    assert auth_client.get("/api/messages", params={"status": "обработано"}).status_code == 422
    assert auth_client.get("/api/messages", params={"period": "квартал"}).status_code == 422
    assert auth_client.get("/api/messages", params={"reply": "может быть"}).status_code == 422


def test_search_treats_wildcards_as_text(auth_client, gmail_connection, make_message):
    """% и _ в запросе — обычные символы, а не шаблон LIKE."""
    make_message(gmail_connection, subject="Скидка 50%")
    make_message(gmail_connection, subject="Обычная тема")
    assert auth_client.get("/api/messages", params={"q": "50%"}).json()["total"] == 1
    assert auth_client.get("/api/messages", params={"q": "%"}).json()["total"] == 1
    assert auth_client.get("/api/messages", params={"q": "_"}).json()["total"] == 0


def test_blank_search_is_ignored(auth_client, gmail_connection, make_message):
    make_message(gmail_connection)
    assert auth_client.get("/api/messages", params={"q": "   "}).json()["total"] == 1
