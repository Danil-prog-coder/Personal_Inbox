"""Сводка за период: 24ч / Неделя / Месяц, по всем источникам сразу."""
from fastapi import APIRouter, Depends, Query
from sqlalchemy.orm import Session

from backend.db import get_db
from backend.models import Message, User
from backend.queries import level_distribution, summary_period_start, user_messages
from backend.schemas import SummaryOut
from backend.security import current_user
from backend.serializers import message_brief

router = APIRouter(prefix="/api", tags=["summary"])

TOP_LIMIT = 4  # «Главное за период» — до 4 сообщений


@router.get("/summary", response_model=SummaryOut)
def get_summary(
    period: str = Query(default="24h", pattern="^(24h|week|month)$"),
    user: User = Depends(current_user),
    db: Session = Depends(get_db),
) -> SummaryOut:
    stmt = user_messages(user).where(Message.received_at >= summary_period_start(period))
    messages = db.execute(
        stmt.order_by(Message.received_at.desc(), Message.id.desc())
    ).scalars().all()

    top = [m for m in messages if m.effective_level == "CRITICAL"]
    top += [m for m in messages if m.effective_level == "HIGH"]

    return SummaryOut(
        period=period,
        total=len(messages),
        distribution=level_distribution(db, stmt),
        needs_reply=sum(1 for m in messages if m.needs_reply),
        needs_action=sum(1 for m in messages if m.needs_action),
        top=[message_brief(m) for m in top[:TOP_LIMIT]],
    )
