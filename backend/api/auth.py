"""Регистрация, вход, выход."""
from fastapi import APIRouter, Depends, HTTPException, Request, Response, status
from sqlalchemy import func, select
from sqlalchemy.orm import Session

from backend.db import get_db
from backend.models import User
from backend.schemas import Credentials, LoginCredentials, UserOut
from backend.security import hash_password, login_user, logout_user, verify_password

router = APIRouter(prefix="/api/auth", tags=["auth"])


def _find_by_email(db: Session, email: str) -> User | None:
    return db.execute(
        select(User).where(func.lower(User.email) == email.strip().lower())
    ).scalar_one_or_none()


@router.post("/register", response_model=UserOut, status_code=status.HTTP_201_CREATED)
def register(payload: Credentials, request: Request, db: Session = Depends(get_db)) -> User:
    if _find_by_email(db, payload.email):
        raise HTTPException(status_code=status.HTTP_409_CONFLICT, detail="Этот email уже занят")
    user = User(
        email=payload.email.strip().lower(),
        password_hash=hash_password(payload.password),
        criteria="",
    )
    db.add(user)
    db.commit()
    db.refresh(user)
    login_user(request, user)
    return user


@router.post("/login", response_model=UserOut)
def login(payload: LoginCredentials, request: Request, db: Session = Depends(get_db)) -> User:
    user = _find_by_email(db, payload.email)
    if user is None or not verify_password(payload.password, user.password_hash):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED, detail="Неверный email или пароль"
        )
    login_user(request, user)
    return user


@router.post("/logout", status_code=status.HTTP_204_NO_CONTENT)
def logout(request: Request) -> Response:
    logout_user(request)
    return Response(status_code=status.HTTP_204_NO_CONTENT)
