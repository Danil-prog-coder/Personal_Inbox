"""Профиль: критерии, тема, плотность, смена пароля."""
from backend.models import Message


def test_update_theme_and_density(auth_client):
    body = auth_client.patch("/api/me", json={"theme": "light", "density": "compact"}).json()
    assert body["user"]["theme"] == "light"
    assert body["user"]["density"] == "compact"


def test_unknown_theme_is_rejected(auth_client):
    assert auth_client.patch("/api/me", json={"theme": "серая"}).status_code == 422


def test_criteria_change_queues_reanalysis(auth_client, db, gmail_connection, make_message):
    """Смена критериев ставит все сообщения в PROCESSING (решение №15 — override цел)."""
    message = make_message(gmail_connection, status="DONE", level="LOW", level_override="CRITICAL")
    body = auth_client.patch("/api/me", json={"criteria": "Новые критерии"}).json()
    assert body["reanalyze_queued"] == 1

    db.expire_all()
    stored = db.get(Message, message.id)
    assert stored.status == "PROCESSING"
    assert stored.level_override == "CRITICAL"


def test_same_criteria_does_not_queue_reanalysis(auth_client, user, gmail_connection, make_message):
    make_message(gmail_connection)
    body = auth_client.patch("/api/me", json={"criteria": user.criteria}).json()
    assert body["reanalyze_queued"] == 0


def test_password_change_requires_current_password(auth_client):
    response = auth_client.patch(
        "/api/me", json={"current_password": "неверный", "new_password": "новыйпароль1"}
    )
    assert response.status_code == 400
    assert response.json()["detail"] == "Текущий пароль неверен"


def test_password_change_works(auth_client, user):
    response = auth_client.patch(
        "/api/me", json={"current_password": "qwerty12345", "new_password": "новыйпароль1"}
    )
    assert response.status_code == 200
    auth_client.post("/api/auth/logout")
    assert auth_client.post(
        "/api/auth/login", json={"email": user.email, "password": "новыйпароль1"}
    ).status_code == 200


def test_short_new_password_is_rejected(auth_client):
    response = auth_client.patch(
        "/api/me", json={"current_password": "qwerty12345", "new_password": "1234567"}
    )
    assert response.status_code == 422
