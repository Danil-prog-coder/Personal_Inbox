"""Лента уровня 1: карточки подключённых источников."""
from fastapi import APIRouter, Depends
from sqlalchemy import select
from sqlalchemy.orm import Session

from backend.db import get_db
from backend.models import Connection, Message, User
from backend.queries import level_distribution
from backend.schemas import SourceCard
from backend.security import current_user
from backend.serializers import message_brief

router = APIRouter(prefix="/api", tags=["sources"])


@router.get("/sources", response_model=list[SourceCard])
def list_sources(
    user: User = Depends(current_user),
    db: Session = Depends(get_db),
) -> list[SourceCard]:
    """Отключённые источники в ленте не показываются вообще (решение №22)."""
    connections = db.execute(
        select(Connection)
        .where(Connection.user_id == user.id, Connection.state.in_(("active", "reauth")))
        .order_by(Connection.kind)
    ).scalars().all()

    cards: list[SourceCard] = []
    for connection in connections:
        stmt = select(Message).where(Message.connection_id == connection.id)
        messages = db.execute(
            stmt.order_by(Message.received_at.desc(), Message.id.desc())
        ).scalars().all()
        urgent = next((m for m in messages if m.effective_level == "CRITICAL"), None) or next(
            (m for m in messages if m.effective_level == "HIGH"), None
        )
        cards.append(
            SourceCard(
                kind=connection.kind,
                state=connection.state,
                account=connection.account,
                last_sync_at=connection.last_sync_at,
                total=len(messages),
                unread=sum(1 for m in messages if not m.is_read),
                distribution=level_distribution(db, stmt),
                urgent=message_brief(urgent) if urgent else None,
            )
        )
    return cards
