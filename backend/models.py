"""Модель данных. Имена полей и значения перечислений — из docs/03-data-model.md,
менять их без правки документа нельзя."""
from datetime import datetime, timezone

from sqlalchemy import (
    Boolean,
    DateTime,
    ForeignKey,
    Index,
    Integer,
    String,
    Text,
    UniqueConstraint,
    func,
)
from sqlalchemy.ext.hybrid import hybrid_property
from sqlalchemy.orm import Mapped, mapped_column, relationship

from backend.db import Base

# --- Перечисления (хранятся строками) --------------------------------------
LEVELS = ("CRITICAL", "HIGH", "NORMAL", "LOW")
SOURCE_KINDS = ("gmail", "telegram")
CONN_STATES = ("off", "active", "reauth")
MSG_STATUSES = ("PROCESSING", "DONE")
THEMES = ("dark", "light")
DENSITIES = ("spacious", "compact")


def utcnow() -> datetime:
    return datetime.now(timezone.utc).replace(tzinfo=None)


class User(Base):
    __tablename__ = "user"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    email: Mapped[str] = mapped_column(String(320), unique=True, nullable=False)
    password_hash: Mapped[str] = mapped_column(String(128), nullable=False)
    criteria: Mapped[str] = mapped_column(Text, nullable=False, default="")
    theme: Mapped[str] = mapped_column(String(8), nullable=False, default="dark")
    density: Mapped[str] = mapped_column(String(16), nullable=False, default="spacious")
    created_at: Mapped[datetime] = mapped_column(DateTime, nullable=False, default=utcnow)

    connections: Mapped[list["Connection"]] = relationship(
        back_populates="user", cascade="all, delete-orphan"
    )


class Connection(Base):
    __tablename__ = "connection"
    __table_args__ = (UniqueConstraint("user_id", "kind", name="uq_connection_user_kind"),)

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    user_id: Mapped[int] = mapped_column(ForeignKey("user.id", ondelete="CASCADE"), nullable=False)
    kind: Mapped[str] = mapped_column(String(16), nullable=False)
    state: Mapped[str] = mapped_column(String(16), nullable=False, default="off")
    account: Mapped[str] = mapped_column(String(320), nullable=False, default="")
    # JSON-строка: refresh_token для Gmail, bot_token для Telegram.
    credentials: Mapped[str] = mapped_column(Text, nullable=False, default="")
    last_sync_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    sync_cursor: Mapped[str | None] = mapped_column(String(64), nullable=True)

    user: Mapped[User] = relationship(back_populates="connections")
    messages: Mapped[list["Message"]] = relationship(
        back_populates="connection", cascade="all, delete-orphan"
    )


class Message(Base):
    __tablename__ = "message"
    __table_args__ = (
        UniqueConstraint("connection_id", "external_id", name="uq_message_conn_external"),
        Index("ix_message_conn_received", "connection_id", "received_at"),
        Index("ix_message_conn_read", "connection_id", "is_read"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    connection_id: Mapped[int] = mapped_column(
        ForeignKey("connection.id", ondelete="CASCADE"), nullable=False
    )
    external_id: Mapped[str] = mapped_column(String(255), nullable=False)
    sender_name: Mapped[str] = mapped_column(String(255), nullable=False, default="")
    sender_addr: Mapped[str] = mapped_column(String(255), nullable=False, default="")
    subject: Mapped[str] = mapped_column(Text, nullable=False, default="")
    body: Mapped[str] = mapped_column(Text, nullable=False, default="")
    received_at: Mapped[datetime] = mapped_column(DateTime, nullable=False, default=utcnow)
    is_read: Mapped[bool] = mapped_column(Boolean, nullable=False, default=False)
    status: Mapped[str] = mapped_column(String(16), nullable=False, default="PROCESSING")
    level: Mapped[str | None] = mapped_column(String(16), nullable=True)
    level_override: Mapped[str | None] = mapped_column(String(16), nullable=True)
    category: Mapped[str] = mapped_column(String(255), nullable=False, default="")
    deadline_text: Mapped[str] = mapped_column(String(255), nullable=False, default="")
    needs_reply: Mapped[bool] = mapped_column(Boolean, nullable=False, default=False)
    needs_action: Mapped[bool] = mapped_column(Boolean, nullable=False, default=False)
    summary: Mapped[str] = mapped_column(Text, nullable=False, default="")
    external_url: Mapped[str] = mapped_column(Text, nullable=False, default="")
    analyzed_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    # Модель не ответила после трёх попыток — на карточке пометка «Оценка недоступна».
    analysis_failed: Mapped[bool] = mapped_column(Boolean, nullable=False, default=False)

    connection: Mapped[Connection] = relationship(back_populates="messages")
    overrides: Mapped[list["OverrideLog"]] = relationship(
        back_populates="message", cascade="all, delete-orphan"
    )

    @hybrid_property
    def effective_level(self) -> str:
        """Уровень, который видит пользователь: ручное исправление важнее оценки модели."""
        return self.level_override or self.level or "NORMAL"

    @effective_level.expression
    @classmethod
    def effective_level(cls):
        return func.coalesce(cls.level_override, cls.level, "NORMAL")


class OverrideLog(Base):
    __tablename__ = "override_log"

    id: Mapped[int] = mapped_column(Integer, primary_key=True)
    message_id: Mapped[int] = mapped_column(
        ForeignKey("message.id", ondelete="CASCADE"), nullable=False
    )
    from_level: Mapped[str | None] = mapped_column(String(16), nullable=True)
    to_level: Mapped[str] = mapped_column(String(16), nullable=False)
    created_at: Mapped[datetime] = mapped_column(DateTime, nullable=False, default=utcnow)

    message: Mapped[Message] = relationship(back_populates="overrides")
