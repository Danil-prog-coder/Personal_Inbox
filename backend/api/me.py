"""Профиль: критерии важности, тема, плотность, смена пароля."""
from fastapi import APIRouter, Depends, HTTPException, status
from sqlalchemy.orm import Session

from backend.analysis import queue_reanalysis
from backend.db import get_db
from backend.models import User
from backend.schemas import MeUpdate, MeUpdateResult, UserOut
from backend.security import current_user, hash_password, verify_password

router = APIRouter(prefix="/api", tags=["me"])


@router.get("/me", response_model=UserOut)
def get_me(user: User = Depends(current_user)) -> User:
    return user


@router.patch("/me", response_model=MeUpdateResult)
def update_me(
    payload: MeUpdate,
    user: User = Depends(current_user),
    db: Session = Depends(get_db),
) -> MeUpdateResult:
    if payload.new_password is not None:
        if not payload.current_password or not verify_password(
            payload.current_password, user.password_hash
        ):
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST, detail="Текущий пароль неверен"
            )
        user.password_hash = hash_password(payload.new_password)

    if payload.theme is not None:
        user.theme = payload.theme
    if payload.density is not None:
        user.density = payload.density

    # Сохранение новых критериев ставит все сообщения на переоценку.
    criteria_changed = payload.criteria is not None and payload.criteria != user.criteria
    if payload.criteria is not None:
        user.criteria = payload.criteria

    db.commit()
    db.refresh(user)

    queued = queue_reanalysis(db, user) if criteria_changed else 0
    return MeUpdateResult(user=UserOut.model_validate(user), reanalyze_queued=queued)
