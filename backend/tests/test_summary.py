"""Сводка: окно периода, распределение, главное за период."""
from datetime import timedelta

from backend.models import utcnow


def test_summary_window(auth_client, gmail_connection, make_message):
    make_message(gmail_connection, received_at=utcnow() - timedelta(hours=3))
    make_message(gmail_connection, received_at=utcnow() - timedelta(days=3))
    make_message(gmail_connection, received_at=utcnow() - timedelta(days=40))
    assert auth_client.get("/api/summary", params={"period": "24h"}).json()["total"] == 1
    assert auth_client.get("/api/summary", params={"period": "week"}).json()["total"] == 2
    assert auth_client.get("/api/summary", params={"period": "month"}).json()["total"] == 2


def test_summary_counts_reply_and_action(auth_client, gmail_connection, make_message):
    make_message(gmail_connection, needs_reply=True, needs_action=True)
    make_message(gmail_connection, needs_reply=True, needs_action=False)
    body = auth_client.get("/api/summary").json()
    assert (body["needs_reply"], body["needs_action"]) == (2, 1)


def test_summary_covers_all_sources(
    auth_client, gmail_connection, telegram_connection, make_message
):
    make_message(gmail_connection)
    make_message(telegram_connection)
    assert auth_client.get("/api/summary").json()["total"] == 2


def test_top_lists_critical_first_and_limits_to_four(auth_client, gmail_connection, make_message):
    for _ in range(3):
        make_message(gmail_connection, level="HIGH")
    critical = [make_message(gmail_connection, level="CRITICAL") for _ in range(2)]
    make_message(gmail_connection, level="LOW")
    top = auth_client.get("/api/summary").json()["top"]
    assert len(top) == 4
    assert [item["level"] for item in top[:2]] == ["CRITICAL", "CRITICAL"]
    assert {item["id"] for item in top[:2]} == {m.id for m in critical}


def test_summary_rejects_unknown_period(auth_client):
    assert auth_client.get("/api/summary", params={"period": "год"}).status_code == 422
