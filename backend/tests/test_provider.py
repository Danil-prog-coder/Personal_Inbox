"""Адаптер модели: промпт и строгий разбор ответа."""
import pytest

from backend import config
from backend.llm.provider import AnalysisRequest, LLMUnavailable, build_user_prompt, parse_response

VALID = (
    '{"level":"HIGH","category":"Финансы","deadline":"Пятница",'
    '"needs_reply":true,"needs_action":false,"summary":"Нужны акты."}'
)


def test_parse_valid_response():
    result = parse_response(VALID)
    assert result.level == "HIGH"
    assert result.category == "Финансы"
    assert result.needs_reply is True
    assert result.needs_action is False


def test_parse_trims_whitespace():
    raw = VALID.replace('"Финансы"', '"  Финансы  "')
    assert parse_response(raw).category == "Финансы"


@pytest.mark.parametrize(
    "raw",
    [
        "это не json",
        "[]",
        '{"level":"URGENT","category":"","deadline":"","needs_reply":true,"needs_action":true,"summary":""}',
        '{"level":"LOW","category":1,"deadline":"","needs_reply":true,"needs_action":true,"summary":""}',
        '{"level":"LOW","category":"","deadline":"","needs_reply":"да","needs_action":true,"summary":""}',
        '{"level":"LOW"}',
        "",
    ],
)
def test_invalid_response_is_llm_unavailable(raw):
    """Всё, что не соответствует схеме, — ошибка вызова, а не «пустая оценка»."""
    with pytest.raises(LLMUnavailable):
        parse_response(raw)


def _request(**kwargs) -> AnalysisRequest:
    defaults = dict(
        criteria="Важны договоры",
        sender="Анна <a@northline.io>",
        subject="Договор",
        body="Текст письма",
        source="gmail",
        overrides=[],
    )
    defaults.update(kwargs)
    return AnalysisRequest(**defaults)


def test_prompt_contains_criteria_and_message():
    prompt = build_user_prompt(_request())
    assert "Важны договоры" in prompt
    assert "Анна <a@northline.io>" in prompt
    assert "Текст письма" in prompt
    assert "gmail" in prompt


def test_prompt_without_criteria_says_so():
    assert "не заданы" in build_user_prompt(_request(criteria="   "))


def test_prompt_truncates_long_body():
    """Текст сообщения уходит в модель обрезанным до 4000 символов."""
    prompt = build_user_prompt(_request(body="ы" * 10_000))
    assert "ы" * config.LLM_MAX_BODY_CHARS in prompt
    assert "ы" * (config.LLM_MAX_BODY_CHARS + 1) not in prompt


def test_prompt_includes_overrides():
    prompt = build_user_prompt(_request(overrides=[("Счёт на оплату", "CRITICAL")]))
    assert "Счёт на оплату → CRITICAL" in prompt


def test_analyze_without_api_key_raises(monkeypatch):
    from backend.llm import provider

    monkeypatch.setattr(provider.config, "OPENAI_API_KEY", "")
    with pytest.raises(LLMUnavailable):
        provider.analyze(_request())
