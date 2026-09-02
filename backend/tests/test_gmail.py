"""Gmail: разбор письма и реакция на ошибки. Сеть не задействована."""
import base64
from datetime import datetime, timezone

from backend.sources import gmail


def _b64(text: str) -> str:
    return base64.urlsafe_b64encode(text.encode("utf-8")).decode("ascii")


def _raw(**kwargs) -> dict:
    payload = {
        "mimeType": "text/plain",
        "headers": [
            {"name": "From", "value": "Анна Ковалёва <a.kovaleva@northline.io>"},
            {"name": "Subject", "value": "Договор Northline"},
            {"name": "Date", "value": "Tue, 02 Sep 2026 09:41:00 +0000"},
        ],
        "body": {"data": _b64("Юристы вернули версию с комментариями.")},
    }
    raw = {
        "id": "abc123",
        "internalDate": "1756802460000",
        "payload": payload,
        "snippet": "сниппет",
    }
    raw.update(kwargs)
    return raw


def test_parses_sender_subject_and_body():
    incoming = gmail._to_incoming(_raw())
    assert incoming.sender_name == "Анна Ковалёва"
    assert incoming.sender_addr == "a.kovaleva@northline.io"
    assert incoming.subject == "Договор Northline"
    assert incoming.body == "Юристы вернули версию с комментариями."
    assert incoming.external_url.endswith("abc123")


def test_uses_internal_date():
    incoming = gmail._to_incoming(_raw())
    expected = datetime.fromtimestamp(1756802460, tz=timezone.utc).replace(tzinfo=None)
    assert incoming.received_at == expected


def test_falls_back_to_date_header():
    raw = _raw()
    del raw["internalDate"]
    assert gmail._to_incoming(raw).received_at == datetime(2026, 9, 2, 9, 41)


def test_multipart_prefers_plain_text():
    raw = _raw(
        payload={
            "mimeType": "multipart/alternative",
            "headers": [{"name": "Subject", "value": "Тема"}],
            "parts": [
                {"mimeType": "text/plain", "body": {"data": _b64("простой текст")}},
                {"mimeType": "text/html", "body": {"data": _b64("<p>разметка</p>")}},
            ],
        }
    )
    assert gmail._to_incoming(raw).body == "простой текст"


def test_html_only_letter_is_stripped():
    raw = _raw(
        payload={
            "mimeType": "text/html",
            "headers": [{"name": "Subject", "value": "Тема"}],
            "body": {"data": _b64("<style>p{color:red}</style><p>Привет,  <b>мир</b></p>")},
        }
    )
    assert gmail._to_incoming(raw).body == "Привет, мир"


def test_letter_without_body_uses_snippet():
    raw = _raw(payload={"mimeType": "text/plain", "headers": [], "body": {}})
    incoming = gmail._to_incoming(raw)
    assert incoming.body == "сниппет"
    assert incoming.subject == "(без темы)"
    assert incoming.sender_name == "Без отправителя"


def test_broken_base64_does_not_crash():
    raw = _raw(
        payload={"mimeType": "text/plain", "headers": [], "body": {"data": "!!!не base64!!!"}}
    )
    assert gmail._to_incoming(raw).body == "сниппет"


def test_auth_errors_are_recognized():
    assert gmail._is_auth_error(Exception("invalid_grant: token expired"))
    assert gmail._is_auth_error(Exception("HttpError 401"))
    assert not gmail._is_auth_error(Exception("timed out"))


def test_sync_without_credentials_switches_to_reauth(db, gmail_connection):
    gmail_connection.credentials = "{}"
    db.commit()
    assert gmail.sync(db, gmail_connection) == 0
    assert gmail_connection.state == "reauth"


def test_expired_token_switches_to_reauth(db, gmail_connection, monkeypatch):
    def boom(_refresh_token):
        raise RuntimeError("invalid_grant")

    monkeypatch.setattr(gmail, "_credentials", boom)
    assert gmail.sync(db, gmail_connection) == 0
    assert gmail_connection.state == "reauth"


def test_network_error_keeps_connection_active(db, gmail_connection, monkeypatch):
    def boom(_refresh_token):
        raise RuntimeError("Connection reset by peer")

    monkeypatch.setattr(gmail, "_credentials", boom)
    assert gmail.sync(db, gmail_connection) == 0
    assert gmail_connection.state == "active"


def test_auth_url_requires_client_credentials(monkeypatch):
    import pytest

    monkeypatch.setattr(gmail.config, "GOOGLE_CLIENT_ID", "")
    with pytest.raises(gmail.GmailError):
        gmail.auth_url("state")


class _FakeService:
    """Минимальная заглушка Gmail API: отдаёт заранее заданные ответы."""

    def __init__(self, history_error=None, message_ids=("m1",)):
        self._history_error = history_error
        self._message_ids = list(message_ids)
        self.initial_import_calls = 0

    def users(self):
        return self

    def history(self):
        return self

    def messages(self):
        return self

    def list(self, **kwargs):
        if "startHistoryId" in kwargs:
            if self._history_error:
                raise self._history_error
            return _Execute({"history": [], "historyId": "999"})
        self.initial_import_calls += 1
        return _Execute({"messages": [{"id": mid} for mid in self._message_ids]})

    def getProfile(self, **_kwargs):
        return _Execute({"historyId": "1000", "emailAddress": "me@northline.io"})

    def get(self, **kwargs):
        return _Execute(_raw(id=kwargs["id"]))


class _Execute:
    def __init__(self, value):
        self._value = value

    def execute(self):
        return self._value


def test_stale_history_id_triggers_full_import(db, gmail_connection, monkeypatch, queued):
    """Google хранит историю ограниченное время: 404 — повод импортировать заново."""
    gmail_connection.sync_cursor = "старый"
    db.commit()
    service = _FakeService(history_error=Exception("HttpError 404 Requested entity was not found"))
    monkeypatch.setattr(gmail, "_credentials", lambda token: object())
    monkeypatch.setattr(gmail, "_service", lambda credentials: service)

    assert gmail.sync(db, gmail_connection) == 1
    assert service.initial_import_calls == 1
    assert gmail_connection.sync_cursor == "1000"
    assert gmail_connection.state == "active"


def test_incremental_sync_keeps_cursor(db, gmail_connection, monkeypatch):
    gmail_connection.sync_cursor = "500"
    db.commit()
    service = _FakeService()
    monkeypatch.setattr(gmail, "_credentials", lambda token: object())
    monkeypatch.setattr(gmail, "_service", lambda credentials: service)

    assert gmail.sync(db, gmail_connection) == 0
    assert service.initial_import_calls == 0
    assert gmail_connection.sync_cursor == "999"
