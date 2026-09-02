"""Синхронизация источников раз в 5 минут. APScheduler в том же процессе —
отдельный воркер и брокер для pet-проекта избыточны (решение №2)."""
import logging

from apscheduler.schedulers.background import BackgroundScheduler
from sqlalchemy import select

from backend import config
from backend.db import SessionLocal
from backend.models import Connection
from backend.sources import gmail, telegram

log = logging.getLogger(__name__)

_scheduler: BackgroundScheduler | None = None

SYNCERS = {"gmail": gmail.sync, "telegram": telegram.sync}


def sync_all() -> int:
    """Обойти все активные подключения. Ошибка одного не мешает остальным."""
    saved = 0
    with SessionLocal() as db:
        connections = db.execute(
            select(Connection).where(Connection.state == "active")
        ).scalars().all()
        for connection in connections:
            syncer = SYNCERS.get(connection.kind)
            if syncer is None:
                continue
            try:
                saved += syncer(db, connection)
            except Exception:
                log.exception("Синхронизация %s упала", connection.kind)
    return saved


def start() -> BackgroundScheduler:
    global _scheduler
    if _scheduler is None:
        _scheduler = BackgroundScheduler(timezone="UTC")
        _scheduler.add_job(
            sync_all,
            "interval",
            minutes=config.SYNC_INTERVAL_MINUTES,
            id="sync_sources",
            max_instances=1,
            coalesce=True,
        )
        _scheduler.start()
        log.info("Планировщик запущен: синхронизация раз в %s минут", config.SYNC_INTERVAL_MINUTES)
    return _scheduler


def shutdown() -> None:
    global _scheduler
    if _scheduler is not None:
        _scheduler.shutdown(wait=False)
        _scheduler = None
