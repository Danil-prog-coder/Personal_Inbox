"""Оценка сообщения моделью: успех, повторы, отказ, обратная связь, переоценка."""
import pytest

from backend import analysis
from backend.llm.provider import AnalysisResult, LLMUnavailable
from backend.models import Message, OverrideLog, utcnow


@pytest.fixture
def message(gmail_connection, make_message) -> Message:
    return make_message(
        gmail_connection,
        status="PROCESSING",
        level=None,
        category="",
        summary="",
        subject="Договор Northline",
    )


def _result(**kwargs) -> AnalysisResult:
    defaults = dict(
        level="CRITICAL",
        category="Юридическое",
        deadline="Сегодня, 18:00",
        needs_reply=True,
        needs_action=True,
        summary="Нужно решение по двум пунктам.",
    )
    defaults.update(kwargs)
    return AnalysisResult(**defaults)


def test_successful_analysis_fills_message(db, message, monkeypatch):
    monkeypatch.setattr(analysis, "analyze", lambda request: _result())
    analysis.process_message(db, message.id)

    db.expire_all()
    stored = db.get(Message, message.id)
    assert stored.status == "DONE"
    assert stored.level == "CRITICAL"
    assert stored.category == "Юридическое"
    assert stored.deadline_text == "Сегодня, 18:00"
    assert stored.needs_reply and stored.needs_action
    assert stored.analyzed_at is not None
    assert stored.analysis_failed is False


def test_retries_three_times_then_gives_up(db, message, monkeypatch):
    """Три попытки с задержками 2с / 8с / 30с, потом NORMAL и «Оценка недоступна»."""
    calls = {"n": 0}
    delays: list[float] = []

    def failing(_request):
        calls["n"] += 1
        raise LLMUnavailable("нет связи")

    monkeypatch.setattr(analysis, "analyze", failing)
    analysis.process_message(db, message.id, sleep=delays.append)

    assert calls["n"] == 3
    assert delays == [2, 8]  # после последней попытки не ждём

    db.expire_all()
    stored = db.get(Message, message.id)
    assert stored.status == "DONE"
    assert stored.level == "NORMAL"
    assert stored.analysis_failed is True
    assert stored.summary == ""


def test_second_attempt_succeeds(db, message, monkeypatch):
    calls = {"n": 0}

    def flaky(_request):
        calls["n"] += 1
        if calls["n"] == 1:
            raise LLMUnavailable("таймаут")
        return _result(level="HIGH")

    monkeypatch.setattr(analysis, "analyze", flaky)
    analysis.process_message(db, message.id, sleep=lambda _: None)

    db.expire_all()
    assert db.get(Message, message.id).level == "HIGH"
    assert db.get(Message, message.id).analysis_failed is False


def test_missing_message_is_not_an_error(db, monkeypatch):
    monkeypatch.setattr(analysis, "analyze", lambda request: _result())
    assert analysis.process_message(db, 10_000) is None


def test_analysis_publishes_event(db, message, monkeypatch):
    from backend.events import bus

    cursor = bus.cursor
    monkeypatch.setattr(analysis, "analyze", lambda request: _result())
    analysis.process_message(db, message.id)

    events, _ = bus.since(message.connection.user_id, cursor)
    assert [event.name for event in events] == ["message.analyzed"]
    assert events[0].data["level"] == "CRITICAL"


def test_request_carries_criteria_and_overrides(db, message, user, monkeypatch):
    db.add(OverrideLog(message_id=message.id, from_level="NORMAL", to_level="CRITICAL"))
    db.commit()

    captured = {}

    def capture(request):
        captured["request"] = request
        return _result()

    monkeypatch.setattr(analysis, "analyze", capture)
    analysis.process_message(db, message.id)

    request = captured["request"]
    assert request.criteria == user.criteria
    assert request.source == "gmail"
    assert ("Договор Northline", "CRITICAL") in request.overrides


def test_recent_overrides_limited_and_newest_first(db, user, gmail_connection, make_message):
    for index in range(3):
        item = make_message(gmail_connection, subject=f"Тема {index}")
        db.add(OverrideLog(message_id=item.id, from_level="LOW", to_level="HIGH"))
        db.commit()
    overrides = analysis.recent_overrides(db, user, limit=2)
    assert len(overrides) == 2
    assert overrides[0][0] == "Тема 2"


def test_queue_reanalysis_marks_all_processing(db, user, gmail_connection, make_message, queued):
    first = make_message(gmail_connection, status="DONE")
    second = make_message(gmail_connection, status="DONE", level_override="CRITICAL")

    assert analysis.queue_reanalysis(db, user) == 2
    db.expire_all()
    assert db.get(Message, first.id).status == "PROCESSING"
    # Ручное исправление переоценка не затирает.
    assert db.get(Message, second.id).level_override == "CRITICAL"
    assert set(queued) == {first.id, second.id}


def test_queue_reanalysis_ignores_other_users(db, user, gmail_connection, make_message):
    from backend.models import Connection, User
    from backend.security import hash_password

    other = User(email="other@northline.io", password_hash=hash_password("qwerty12345"))
    db.add(other)
    db.commit()
    other_connection = Connection(user_id=other.id, kind="gmail", state="active")
    db.add(other_connection)
    db.commit()
    make_message(other_connection)
    make_message(gmail_connection)

    assert analysis.queue_reanalysis(db, user) == 1


def test_failed_reanalysis_keeps_previous_verdict(db, gmail_connection, make_message, monkeypatch):
    """Переоценка при недоступной модели не должна стирать готовую карточку."""
    message = make_message(
        gmail_connection,
        status="PROCESSING",
        level="CRITICAL",
        category="Юридическое",
        summary="Старая сводка",
        needs_reply=True,
        analyzed_at=utcnow(),
    )

    def failing(_request):
        raise LLMUnavailable("нет связи")

    monkeypatch.setattr(analysis, "analyze", failing)
    analysis.process_message(db, message.id, sleep=lambda _: None)

    db.expire_all()
    stored = db.get(Message, message.id)
    assert stored.status == "DONE"
    assert stored.level == "CRITICAL"
    assert stored.category == "Юридическое"
    assert stored.summary == "Старая сводка"
    assert stored.analysis_failed is True
