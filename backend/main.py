"""Точка входа: uvicorn backend.main:app --reload"""
import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from starlette.middleware.sessions import SessionMiddleware

from backend import config
from backend.api import auth, connections, me, messages, sources, stream, summary
from backend.db import Base, engine

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s: %(message)s")


def init_db() -> None:
    """Схема создаётся при старте. Alembic нужен для последующих изменений схемы."""
    Base.metadata.create_all(bind=engine)


@asynccontextmanager
async def lifespan(_: FastAPI):
    init_db()
    if config.ENABLE_SCHEDULER:
        from backend import analysis, scheduler

        analysis.ensure_worker()
        scheduler.start()
    if config.DEMO_LIVE:
        import threading

        from backend import seed_data

        threading.Thread(target=seed_data.play_live_queue, daemon=True).start()
    yield
    if config.ENABLE_SCHEDULER:
        from backend import scheduler

        scheduler.shutdown()


app = FastAPI(title="Personal Inbox", lifespan=lifespan)

# Куки-сессия: httpOnly по умолчанию, SameSite=Lax (docs/03-data-model.md, п. 6).
app.add_middleware(
    SessionMiddleware,
    secret_key=config.SESSION_SECRET,
    session_cookie=config.SESSION_COOKIE,
    max_age=config.SESSION_MAX_AGE,
    same_site="lax",
    https_only=False,
)
# Фронт живёт на другом порту в разработке, поэтому куки ходят с credentials.
app.add_middleware(
    CORSMiddleware,
    allow_origins=[config.FRONTEND_ORIGIN],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

for router in (
    auth.router,
    me.router,
    connections.router,
    sources.router,
    messages.router,
    summary.router,
    stream.router,
):
    app.include_router(router)


@app.get("/api/health")
def health() -> dict[str, str]:
    return {"status": "ok"}
