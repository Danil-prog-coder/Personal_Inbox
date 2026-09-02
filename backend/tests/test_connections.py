"""Экран «Источники»: список, подключение Telegram, отключение."""
import json

from backend.models import Connection, Message
from backend.sources import telegram


def test_list_shows_both_sources_even_without_connections(auth_client):
    body = auth_client.get("/api/connections").json()
    assert [item["kind"] for item in body] == ["gmail", "telegram"]
    assert all(item["state"] == "off" for item in body)


def test_list_shows_state_and_account(auth_client, gmail_connection):
    body = auth_client.get("/api/connections").json()
    gmail = next(item for item in body if item["kind"] == "gmail")
    assert gmail["state"] == "active"
    assert gmail["account"] == "me@northline.io"


def test_connect_telegram_verifies_token(auth_client, db, monkeypatch):
    monkeypatch.setattr(telegram, "verify_token", lambda token: "@inbox_bot")
    response = auth_client.post("/api/connections/telegram", json={"bot_token": "123456:тест"})
    assert response.status_code == 200
    assert response.json()["account"] == "@inbox_bot"

    connection = db.query(Connection).filter_by(kind="telegram").one()
    assert connection.state == "active"
    assert json.loads(connection.credentials)["bot_token"] == "123456:тест"


def test_connect_telegram_with_bad_token(auth_client, monkeypatch):
    def boom(_token):
        raise telegram.TelegramError("Unauthorized")

    monkeypatch.setattr(telegram, "verify_token", boom)
    response = auth_client.post("/api/connections/telegram", json={"bot_token": "123456:плохой"})
    assert response.status_code == 400
    assert response.json()["detail"] == "Unauthorized"


def test_reconnect_updates_existing_row(auth_client, db, telegram_connection, monkeypatch):
    monkeypatch.setattr(telegram, "verify_token", lambda token: "@другой_бот")
    auth_client.post("/api/connections/telegram", json={"bot_token": "654321:новый"})
    assert db.query(Connection).filter_by(kind="telegram").count() == 1


def test_disconnect_clears_credentials_but_keeps_messages(
    auth_client, db, gmail_connection, make_message
):
    make_message(gmail_connection)
    assert auth_client.delete("/api/connections/gmail").status_code == 204

    db.expire_all()
    connection = db.get(Connection, gmail_connection.id)
    assert connection.state == "off"
    assert connection.credentials == ""
    assert db.query(Message).count() == 1


def test_disconnect_unknown_source(auth_client):
    assert auth_client.delete("/api/connections/slack").status_code == 404


def test_disconnect_missing_connection(auth_client):
    assert auth_client.delete("/api/connections/telegram").status_code == 404


def test_gmail_start_without_google_credentials(auth_client, monkeypatch):
    from backend.sources import gmail

    monkeypatch.setattr(gmail.config, "GOOGLE_CLIENT_ID", "")
    assert auth_client.post("/api/connections/gmail/start").status_code == 503


def test_gmail_callback_rejects_foreign_state(auth_client):
    response = auth_client.get(
        "/api/connections/gmail/callback", params={"code": "код", "state": "чужой"}
    )
    assert response.status_code == 400


def test_connections_require_auth(client):
    assert client.get("/api/connections").status_code == 401
