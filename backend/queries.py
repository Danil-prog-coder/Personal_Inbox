"""Выборки сообщений: фильтры ленты, распределение по уровням, окна периодов.
Условия периодов — docs/03-data-model.md, пп. 4 и 5."""
from datetime import datetime, timedelta, timezone

from sqlalchemy import Select, func, or_, select
from sqlalchemy.orm import Session

from backend.models import LEVELS, Connection, Message, User

# Смещение часового пояса приходит с фронта в минутах (как -getTimezoneOffset()),
# иначе «Сегодня» на сервере в UTC не совпадёт с «сегодня» пользователя.
MAX_TZ_OFFSET_MINUTES = 14 * 60  # дальше реальных часовых поясов не бывает


def utcnow() -> datetime:
    return datetime.now(timezone.utc).replace(tzinfo=None)


def period_start(period: str, tz_offset: int = 0, now: datetime | None = None) -> datetime | None:
    """Начало окна фильтра «Период». None — «Всё время»."""
    now = now or utcnow()
    if period == "today":
        minutes = max(-MAX_TZ_OFFSET_MINUTES, min(MAX_TZ_OFFSET_MINUTES, tz_offset))
        offset = timedelta(minutes=minutes)
        local_midnight = (now + offset).replace(hour=0, minute=0, second=0, microsecond=0)
        return local_midnight - offset
    if period == "week":
        return now - timedelta(days=7)
    if period == "month":
        return now - timedelta(days=30)
    return None


def summary_period_start(period: str, now: datetime | None = None) -> datetime:
    """Окно сводки: 24ч / Неделя / Месяц."""
    now = now or utcnow()
    hours = {"24h": 24, "week": 24 * 7, "month": 24 * 30}.get(period, 24)
    return now - timedelta(hours=hours)


def user_messages(user: User) -> Select:
    """Базовая выборка: сообщения всех подключений пользователя."""
    return select(Message).join(Connection).where(Connection.user_id == user.id)


def apply_filters(
    stmt: Select,
    *,
    source: str | None = None,
    level: str | None = None,
    status: str | None = None,
    reply: str | None = None,
    action: str | None = None,
    period: str = "all",
    q: str | None = None,
    tz_offset: int = 0,
    now: datetime | None = None,
) -> Select:
    if source:
        stmt = stmt.where(Connection.kind == source)
    if level and level != "all":
        stmt = stmt.where(Message.effective_level == level)
    if status == "unread":
        stmt = stmt.where(Message.is_read.is_(False))
    elif status == "read":
        stmt = stmt.where(Message.is_read.is_(True))
    elif status == "done":
        stmt = stmt.where(Message.status == "DONE")
    if reply in ("yes", "no"):
        stmt = stmt.where(Message.needs_reply.is_(reply == "yes"))
    if action in ("yes", "no"):
        stmt = stmt.where(Message.needs_action.is_(action == "yes"))
    start = period_start(period, tz_offset, now)
    if start is not None:
        stmt = stmt.where(Message.received_at >= start)
    if q and q.strip():
        # Поиск по отправителю, теме и тексту, регистронезависимо.
        # % и _ внутри запроса — обычные символы, а не шаблон LIKE.
        escaped = q.strip().lower().replace("\\", "\\\\").replace("%", "\\%").replace("_", "\\_")
        pattern = f"%{escaped}%"
        stmt = stmt.where(
            or_(
                func.lower(Message.sender_name).like(pattern, escape="\\"),
                func.lower(Message.sender_addr).like(pattern, escape="\\"),
                func.lower(Message.subject).like(pattern, escape="\\"),
                func.lower(Message.body).like(pattern, escape="\\"),
            )
        )
    return stmt


def level_distribution(db: Session, stmt: Select) -> dict[str, int]:
    """Счётчики по четырём уровням — для полосы распределения. Нули включены."""
    counts = {level: 0 for level in LEVELS}
    subq = stmt.with_only_columns(Message.effective_level.label("lvl")).order_by(None).subquery()
    rows = db.execute(select(subq.c.lvl, func.count()).group_by(subq.c.lvl)).all()
    for level, count in rows:
        if level in counts:
            counts[level] = count
    return counts
