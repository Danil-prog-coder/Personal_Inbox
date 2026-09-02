"""Лента уровня 1: карточки источников."""


def test_disconnected_source_is_hidden(auth_client, db, gmail_connection):
    gmail_connection.state = "off"
    db.commit()
    assert auth_client.get("/api/sources").json() == []


def test_reauth_source_is_shown(auth_client, db, gmail_connection):
    gmail_connection.state = "reauth"
    db.commit()
    cards = auth_client.get("/api/sources").json()
    assert [card["state"] for card in cards] == ["reauth"]


def test_card_counts_and_distribution(auth_client, gmail_connection, make_message):
    make_message(gmail_connection, level="CRITICAL", is_read=False)
    make_message(gmail_connection, level="LOW", is_read=True)
    make_message(gmail_connection, level="LOW", is_read=True)
    card = auth_client.get("/api/sources").json()[0]
    assert (card["total"], card["unread"]) == (3, 1)
    # Полоса распределения рисует все четыре уровня, включая нулевые.
    assert card["distribution"] == {"CRITICAL": 1, "HIGH": 0, "NORMAL": 0, "LOW": 2}


def test_urgent_prefers_critical_then_high(auth_client, gmail_connection, make_message):
    make_message(gmail_connection, level="HIGH", subject="Важное")
    critical = make_message(gmail_connection, level="CRITICAL", subject="Критичное")
    card = auth_client.get("/api/sources").json()[0]
    assert card["urgent"]["id"] == critical.id
    assert card["urgent"]["subject"] == "Критичное"


def test_urgent_falls_back_to_high(auth_client, gmail_connection, make_message):
    high = make_message(gmail_connection, level="HIGH")
    make_message(gmail_connection, level="NORMAL")
    assert auth_client.get("/api/sources").json()[0]["urgent"]["id"] == high.id


def test_urgent_is_null_when_nothing_urgent(auth_client, gmail_connection, make_message):
    make_message(gmail_connection, level="NORMAL")
    assert auth_client.get("/api/sources").json()[0]["urgent"] is None


def test_urgent_respects_manual_override(auth_client, gmail_connection, make_message):
    make_message(gmail_connection, level="HIGH")
    overridden = make_message(gmail_connection, level="LOW", level_override="CRITICAL")
    assert auth_client.get("/api/sources").json()[0]["urgent"]["id"] == overridden.id


def test_empty_source_card(auth_client, gmail_connection):
    card = auth_client.get("/api/sources").json()[0]
    assert (card["total"], card["unread"], card["urgent"]) == (0, 0, None)
    assert card["distribution"] == {"CRITICAL": 0, "HIGH": 0, "NORMAL": 0, "LOW": 0}
