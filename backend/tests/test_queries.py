"""Границы окон фильтра «Период» и сводки."""
from datetime import datetime, timedelta

from backend.queries import period_start, summary_period_start

NOW = datetime(2026, 9, 2, 10, 30)


def test_period_all_has_no_bound():
    assert period_start("all", now=NOW) is None
    assert period_start("неизвестно", now=NOW) is None


def test_today_starts_at_local_midnight_utc():
    assert period_start("today", tz_offset=0, now=NOW) == datetime(2026, 9, 2, 0, 0)


def test_today_respects_client_timezone():
    """Москва (+180): местная полночь 2 сентября — это 1 сентября 21:00 UTC."""
    assert period_start("today", tz_offset=180, now=NOW) == datetime(2026, 9, 1, 21, 0)


def test_today_for_far_east_starts_earlier_in_utc():
    """Владивосток (+600): в 10:30 UTC там уже 20:30 второго сентября,
    и местная полночь пришлась на 1 сентября 14:00 UTC."""
    assert period_start("today", tz_offset=600, now=NOW) == datetime(2026, 9, 1, 14, 0)


def test_absurd_timezone_offset_is_clamped():
    assert period_start("today", tz_offset=100_000, now=NOW) == period_start(
        "today", tz_offset=14 * 60, now=NOW
    )


def test_week_and_month_are_rolling_windows():
    assert period_start("week", now=NOW) == NOW - timedelta(days=7)
    assert period_start("month", now=NOW) == NOW - timedelta(days=30)


def test_summary_windows():
    assert summary_period_start("24h", now=NOW) == NOW - timedelta(hours=24)
    assert summary_period_start("week", now=NOW) == NOW - timedelta(days=7)
    assert summary_period_start("month", now=NOW) == NOW - timedelta(days=30)
    assert summary_period_start("что-то", now=NOW) == NOW - timedelta(hours=24)
