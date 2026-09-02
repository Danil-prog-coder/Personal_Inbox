"""Схемы запросов и ответов HTTP API (docs/03-data-model.md, п. 6)."""
from datetime import datetime
from typing import Literal

from pydantic import BaseModel, ConfigDict, EmailStr, Field

Level = Literal["CRITICAL", "HIGH", "NORMAL", "LOW"]
SourceKind = Literal["gmail", "telegram"]
ConnState = Literal["off", "active", "reauth"]
MsgStatus = Literal["PROCESSING", "DONE"]
Theme = Literal["dark", "light"]
Density = Literal["spacious", "compact"]
Period = Literal["all", "today", "week", "month"]
SummaryPeriod = Literal["24h", "week", "month"]

PASSWORD_MIN_LENGTH = 8


class Credentials(BaseModel):
    email: EmailStr
    password: str = Field(min_length=PASSWORD_MIN_LENGTH)


class LoginCredentials(BaseModel):
    """На входе пароль не валидируется по длине: старый короткий пароль
    должен получать «неверный email или пароль», а не 422."""

    email: EmailStr
    password: str


class UserOut(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: int
    email: EmailStr
    criteria: str
    theme: Theme
    density: Density
    created_at: datetime


class MeUpdate(BaseModel):
    criteria: str | None = None
    theme: Theme | None = None
    density: Density | None = None
    current_password: str | None = None
    new_password: str | None = Field(default=None, min_length=PASSWORD_MIN_LENGTH)


class MeUpdateResult(BaseModel):
    user: UserOut
    # Сколько сообщений отправлено на переоценку после смены критериев.
    reanalyze_queued: int = 0


class ConnectionOut(BaseModel):
    kind: SourceKind
    state: ConnState
    account: str
    last_sync_at: datetime | None


class TelegramConnect(BaseModel):
    bot_token: str = Field(min_length=10)


class AuthUrl(BaseModel):
    auth_url: str


class MessageBrief(BaseModel):
    """Строка «Отправитель — Тема» с точкой уровня: карточка источника и сводка."""

    id: int
    sender_name: str
    subject: str
    level: Level


class SourceCard(BaseModel):
    kind: SourceKind
    state: ConnState
    account: str
    last_sync_at: datetime | None
    total: int
    unread: int
    distribution: dict[str, int]
    urgent: MessageBrief | None


class MessageOut(BaseModel):
    id: int
    source: SourceKind
    external_id: str
    sender_name: str
    sender_addr: str
    subject: str
    body: str
    received_at: datetime
    is_read: bool
    status: MsgStatus
    level: Level
    level_override: Level | None
    category: str
    deadline_text: str
    needs_reply: bool
    needs_action: bool
    summary: str
    external_url: str
    analyzed_at: datetime | None
    analysis_failed: bool


class MessageList(BaseModel):
    items: list[MessageOut]
    total: int
    unread: int


class LevelUpdate(BaseModel):
    level: Level


class SummaryOut(BaseModel):
    period: SummaryPeriod
    total: int
    distribution: dict[str, int]
    needs_reply: int
    needs_action: int
    top: list[MessageBrief]
