"""Пароли и текущий пользователь. Сессия — подписанная cookie (SessionMiddleware),
JWT сознательно не заводим (docs/03-data-model.md, п. 6)."""
import bcrypt
from fastapi import Depends, HTTPException, Request, status
from sqlalchemy.orm import Session

from backend.db import get_db
from backend.models import User

# bcrypt учитывает только первые 72 байта пароля и на более длинном падает,
# поэтому режем сами — одинаково при регистрации и при проверке.
_BCRYPT_MAX_BYTES = 72


def _prepare(password: str) -> bytes:
    return password.encode("utf-8")[:_BCRYPT_MAX_BYTES]


def hash_password(password: str) -> str:
    return bcrypt.hashpw(_prepare(password), bcrypt.gensalt()).decode("utf-8")


def verify_password(password: str, password_hash: str) -> bool:
    try:
        return bcrypt.checkpw(_prepare(password), password_hash.encode("utf-8"))
    except ValueError:
        # Битый или пустой хеш в базе — это не 500, это «пароль не подошёл».
        return False


def login_user(request: Request, user: User) -> None:
    request.session["user_id"] = user.id


def logout_user(request: Request) -> None:
    request.session.clear()


def current_user(request: Request, db: Session = Depends(get_db)) -> User:
    user_id = request.session.get("user_id")
    user = db.get(User, user_id) if user_id else None
    if user is None:
        # Сессия есть, а пользователя нет — чистим, иначе cookie будет вечно битой.
        request.session.clear()
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Требуется вход")
    return user
