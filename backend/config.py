"""Настройки приложения. Читаются из переменных окружения, всё имеет разумный
дефолт — pet-проект должен запускаться без единой настройки."""
import os
from pathlib import Path

from dotenv import load_dotenv

BASE_DIR = Path(__file__).resolve().parent
load_dotenv(BASE_DIR.parent / ".env")


def _flag(name: str, default: bool = False) -> bool:
    return os.getenv(name, "1" if default else "0").strip().lower() in ("1", "true", "yes", "on")


# --- Хранилище -------------------------------------------------------------
DATABASE_URL = os.getenv("DATABASE_URL", f"sqlite:///{BASE_DIR / 'personal_inbox.db'}")

# --- Сессии ----------------------------------------------------------------
# Ключ по умолчанию годится только для локального запуска: при смене ключа
# все выданные cookie перестают действовать.
SESSION_SECRET = os.getenv("SESSION_SECRET", "personal-inbox-dev-secret")
SESSION_COOKIE = "pi_session"
SESSION_MAX_AGE = 60 * 60 * 24 * 30  # 30 дней

# --- Языковая модель -------------------------------------------------------
OPENAI_API_KEY = os.getenv("OPENAI_API_KEY", "")
OPENAI_MODEL = os.getenv("OPENAI_MODEL", "gpt-4o-mini")
LLM_MAX_BODY_CHARS = 4000          # обрезка текста сообщения перед отправкой в модель
LLM_RETRY_DELAYS = (2, 8, 30)      # три повтора: 2с / 8с / 30с (см. 00-product-spec, п. 6.1)
LLM_OVERRIDE_HISTORY = 20          # сколько ручных исправлений уходит в промпт

# --- Источники -------------------------------------------------------------
GOOGLE_CLIENT_ID = os.getenv("GOOGLE_CLIENT_ID", "")
GOOGLE_CLIENT_SECRET = os.getenv("GOOGLE_CLIENT_SECRET", "")
APP_BASE_URL = os.getenv("APP_BASE_URL", "http://localhost:8000")
GOOGLE_REDIRECT_URI = os.getenv(
    "GOOGLE_REDIRECT_URI", f"{APP_BASE_URL}/api/connections/gmail/callback"
)
GMAIL_IMPORT_DAYS = 30             # глубина первого импорта
GMAIL_IMPORT_LIMIT = 200           # и потолок по числу писем
SYNC_INTERVAL_MINUTES = 5          # частота синхронизации источников

# --- Разное ----------------------------------------------------------------
FRONTEND_ORIGIN = os.getenv("FRONTEND_ORIGIN", "http://localhost:5173")
ENABLE_SCHEDULER = _flag("ENABLE_SCHEDULER", True)
# Проигрывает очередь «новых» сообщений из референса при старте сервера —
# нужно, чтобы посмотреть появление карточек в реальном времени без источников.
DEMO_LIVE = _flag("DEMO_LIVE", False)
