"""Адаптер языковой модели. Смена провайдера правит только этот файл
(docs/04-decisions.md, решение №5)."""
import json
from dataclasses import dataclass

from backend import config
from backend.models import LEVELS


class LLMUnavailable(RuntimeError):
    """Модель не ответила или ответила не по схеме. Обрабатывается повтором."""


@dataclass(frozen=True)
class AnalysisRequest:
    criteria: str
    sender: str
    subject: str
    body: str
    source: str
    overrides: list[tuple[str, str]]  # (тема, уровень, который поставил пользователь)


@dataclass(frozen=True)
class AnalysisResult:
    level: str
    category: str
    deadline: str
    needs_reply: bool
    needs_action: bool
    summary: str


RESPONSE_SCHEMA = {
    "type": "object",
    "additionalProperties": False,
    "required": ["level", "category", "deadline", "needs_reply", "needs_action", "summary"],
    "properties": {
        "level": {"type": "string", "enum": list(LEVELS)},
        "category": {"type": "string"},
        "deadline": {"type": "string"},
        "needs_reply": {"type": "boolean"},
        "needs_action": {"type": "boolean"},
        "summary": {"type": "string"},
    },
}

SYSTEM_PROMPT = (
    "Ты помощник, который сортирует входящие сообщения по важности для одного человека. "
    "Отвечай строго JSON по заданной схеме. Все текстовые поля — на русском языке, "
    "независимо от языка исходного сообщения.\n"
    "level — один из: CRITICAL (нельзя пропустить, есть жёсткий срок или риск), "
    "HIGH (важно, требует внимания сегодня-завтра), NORMAL (обычное рабочее), "
    "LOW (рассылки, уведомления сервисов, ничего не требуют).\n"
    "category — 1–2 слова, например «Финансы», «Юридическое», «Найм». "
    "Если тема непонятна, верни пустую строку.\n"
    "deadline — срок словами из сообщения: «Сегодня, 18:00», «Пятница». "
    "Если срока нет — пустая строка.\n"
    "summary — 1–2 предложения о сути сообщения."
)


def build_user_prompt(request: AnalysisRequest) -> str:
    parts = [
        "Критерии важности пользователя (свободный текст, не жёсткие правила):",
        request.criteria.strip() or "— не заданы, оценивай на общих основаниях —",
        "",
        f"Источник: {request.source}",
        f"Отправитель: {request.sender}",
        f"Тема: {request.subject}",
        "Текст:",
        request.body[: config.LLM_MAX_BODY_CHARS],
    ]
    if request.overrides:
        parts += [
            "",
            "Ручные исправления пользователя по прошлым сообщениям "
            "(тема → уровень, который он считает верным):",
        ]
        parts += [f"- {subject} → {level}" for subject, level in request.overrides]
    return "\n".join(parts)


def _client():
    if not config.OPENAI_API_KEY:
        raise LLMUnavailable("OPENAI_API_KEY не задан")
    from openai import OpenAI  # импорт здесь: без ключа пакет не нужен

    return OpenAI(api_key=config.OPENAI_API_KEY)


def analyze(request: AnalysisRequest) -> AnalysisResult:
    """Один вызов модели на сообщение. Любой сбой — LLMUnavailable."""
    try:
        response = _client().chat.completions.create(
            model=config.OPENAI_MODEL,
            messages=[
                {"role": "system", "content": SYSTEM_PROMPT},
                {"role": "user", "content": build_user_prompt(request)},
            ],
            response_format={
                "type": "json_schema",
                "json_schema": {"name": "analysis", "schema": RESPONSE_SCHEMA, "strict": True},
            },
            temperature=0,
        )
        raw = response.choices[0].message.content or ""
    except LLMUnavailable:
        raise
    except Exception as error:  # сеть, лимиты, отказ провайдера
        raise LLMUnavailable(str(error)) from error
    return parse_response(raw)


def parse_response(raw: str) -> AnalysisResult:
    """Разбор ответа модели. Всё, что не соответствует схеме, — ошибка вызова
    (docs/00-product-spec.md, п. 6.2)."""
    try:
        data = json.loads(raw)
    except (json.JSONDecodeError, TypeError) as error:
        raise LLMUnavailable(f"ответ модели не разобрать: {raw[:200]}") from error
    if not isinstance(data, dict):
        raise LLMUnavailable("ответ модели не объект")
    level = data.get("level")
    if level not in LEVELS:
        raise LLMUnavailable(f"неизвестный уровень: {level!r}")
    for key in ("category", "deadline", "summary"):
        if not isinstance(data.get(key), str):
            raise LLMUnavailable(f"поле {key} не строка")
    for key in ("needs_reply", "needs_action"):
        if not isinstance(data.get(key), bool):
            raise LLMUnavailable(f"поле {key} не булево")
    return AnalysisResult(
        level=level,
        category=data["category"].strip(),
        deadline=data["deadline"].strip(),
        needs_reply=data["needs_reply"],
        needs_action=data["needs_action"],
        summary=data["summary"].strip(),
    )
