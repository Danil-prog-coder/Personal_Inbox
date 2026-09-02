"""Общие фикстуры. Тесты работают на отдельной временной базе — переменная
окружения выставляется до импорта backend, иначе config подхватит рабочую БД."""
import os
import tempfile

_TMP_DIR = tempfile.mkdtemp(prefix="personal-inbox-tests-")
os.environ["DATABASE_URL"] = f"sqlite:///{_TMP_DIR}/test.db"
os.environ["ENABLE_SCHEDULER"] = "0"
os.environ["FRONTEND_ORIGIN"] = "http://localhost:5173"

from datetime import timedelta  # noqa: E402

import pytest  # noqa: E402
from fastapi.testclient import TestClient  # noqa: E402

from backend import analysis  # noqa: E402
from backend.db import Base, SessionLocal, engine  # noqa: E402
from backend.events import bus  # noqa: E402
from backend.main import app  # noqa: E402
from backend.models import Connection, Message, User, utcnow  # noqa: E402
from backend.security import hash_password  # noqa: E402


@pytest.fixture(autouse=True)
def clean_db():
    Base.metadata.drop_all(bind=engine)
    Base.metadata.create_all(bind=engine)
    bus.clear()
    yield
    Base.metadata.drop_all(bind=engine)


@pytest.fixture(autouse=True)
def queued(monkeypatch) -> list[int]:
    """Фоновый воркер оценки в тестах не запускаем: очередь просто копится в списке.
    Сама оценка проверяется отдельно, вызовом process_message напрямую."""
    ids: list[int] = []
    monkeypatch.setattr(analysis, "enqueue", ids.append)
    return ids


@pytest.fixture
def db():
    session = SessionLocal()
    try:
        yield session
    finally:
        session.close()


@pytest.fixture
def client():
    with TestClient(app) as test_client:
        yield test_client


@pytest.fixture
def user(db) -> User:
    user = User(
        email="max@northline.io",
        password_hash=hash_password("qwerty12345"),
        criteria="Важны договоры и сроки.",
    )
    db.add(user)
    db.commit()
    db.refresh(user)
    return user


@pytest.fixture
def auth_client(client, user) -> TestClient:
    response = client.post(
        "/api/auth/login", json={"email": user.email, "password": "qwerty12345"}
    )
    assert response.status_code == 200
    return client


@pytest.fixture
def gmail_connection(db, user) -> Connection:
    connection = Connection(
        user_id=user.id,
        kind="gmail",
        state="active",
        account="me@northline.io",
        credentials='{"refresh_token": "тест"}',
        last_sync_at=utcnow(),
    )
    db.add(connection)
    db.commit()
    db.refresh(connection)
    return connection


@pytest.fixture
def telegram_connection(db, user) -> Connection:
    connection = Connection(
        user_id=user.id,
        kind="telegram",
        state="active",
        account="@maxorlov",
        credentials='{"bot_token": "123:abc"}',
    )
    db.add(connection)
    db.commit()
    db.refresh(connection)
    return connection


@pytest.fixture
def make_message(db):
    """Фабрика сообщений: минимум обязательных полей, остальное — по умолчанию."""
    counter = {"n": 0}

    def _make(connection, **kwargs) -> Message:
        counter["n"] += 1
        defaults = dict(
            connection_id=connection.id,
            external_id=f"ext-{counter['n']}",
            sender_name=f"Отправитель {counter['n']}",
            sender_addr=f"user{counter['n']}@northline.io",
            subject=f"Тема {counter['n']}",
            body="Текст сообщения",
            received_at=utcnow() - timedelta(minutes=counter["n"]),
            is_read=False,
            status="DONE",
            level="NORMAL",
            category="Работа",
            deadline_text="",
            needs_reply=False,
            needs_action=False,
            summary="Краткое содержание",
        )
        defaults.update(kwargs)
        message = Message(**defaults)
        db.add(message)
        db.commit()
        db.refresh(message)
        return message

    return _make
