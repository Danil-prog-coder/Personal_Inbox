"""Подключение к БД. Одна база, один процесс — ничего сложнее не нужно."""
from collections.abc import Iterator

from sqlalchemy import create_engine, event
from sqlalchemy.orm import DeclarativeBase, Session, sessionmaker

from backend import config

_connect_args = {"check_same_thread": False} if config.DATABASE_URL.startswith("sqlite") else {}
engine = create_engine(config.DATABASE_URL, connect_args=_connect_args, future=True)

if config.DATABASE_URL.startswith("sqlite"):

    @event.listens_for(engine, "connect")
    def _prepare_sqlite(dbapi_connection, _record):  # pragma: no cover - тривиально
        cursor = dbapi_connection.cursor()
        cursor.execute("PRAGMA foreign_keys=ON")
        cursor.close()
        # Встроенный lower() в SQLite умеет только латиницу, поэтому поиск по
        # русскому тексту без этой замены был бы регистрозависимым.
        dbapi_connection.create_function(
            "lower", 1, lambda value: value.lower() if isinstance(value, str) else value,
            deterministic=True,
        )


SessionLocal = sessionmaker(bind=engine, autoflush=False, expire_on_commit=False, future=True)


class Base(DeclarativeBase):
    pass


def get_db() -> Iterator[Session]:
    """Зависимость FastAPI: сессия на запрос."""
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close()
