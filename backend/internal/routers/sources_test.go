package routers

import (
	"net/http"
	"testing"

	"personalinbox/internal/schemas"
)

func (e *env) cards() []schemas.SourceCard {
	e.t.Helper()
	status, raw := e.do(http.MethodGet, "/api/sources", nil)
	if status != http.StatusOK {
		e.t.Fatalf("карточки источников вернули %d: %s", status, raw)
	}
	var cards []schemas.SourceCard
	e.decode(raw, &cards)
	return cards
}

func TestDisconnectedSourceIsHidden(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	e.connection(user, "gmail", "off")

	if cards := e.cards(); len(cards) != 0 {
		t.Fatalf("отключённый источник не должен показываться: %+v", cards)
	}
}

func TestReauthSourceIsShown(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	e.connection(user, "telegram", "reauth")

	cards := e.cards()
	if len(cards) != 1 || cards[0].State != "reauth" {
		t.Fatalf("источник в состоянии reauth должен быть в ленте: %+v", cards)
	}
}

func TestCardCountsAndDistribution(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, func(m *messageModel) {
		m.Level = "CRITICAL"
		m.IsRead = false
	})
	e.message(connection, func(m *messageModel) {
		m.Level = "LOW"
		m.IsRead = true
	})

	cards := e.cards()
	if len(cards) != 1 {
		t.Fatalf("карточек должно быть одна: %d", len(cards))
	}
	card := cards[0]
	if card.Total != 2 || card.Unread != 1 {
		t.Fatalf("счётчики карточки: всего %d, непрочитанных %d", card.Total, card.Unread)
	}
	if card.Distribution["CRITICAL"] != 1 || card.Distribution["LOW"] != 1 ||
		card.Distribution["HIGH"] != 0 || card.Distribution["NORMAL"] != 0 {
		t.Fatalf("распределение по уровням: %v", card.Distribution)
	}
}

func TestUrgentPrefersCriticalThenHigh(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, func(m *messageModel) {
		m.Level = "HIGH"
		m.Subject = "Важное"
	})
	e.message(connection, func(m *messageModel) {
		m.Level = "CRITICAL"
		m.Subject = "Критичное"
	})

	card := e.cards()[0]
	if card.Urgent == nil || card.Urgent.Subject != "Критичное" {
		t.Fatalf("самым срочным должно быть CRITICAL: %+v", card.Urgent)
	}
}

func TestUrgentFallsBackToHigh(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, func(m *messageModel) { m.Level = "NORMAL" })
	e.message(connection, func(m *messageModel) {
		m.Level = "HIGH"
		m.Subject = "Важное"
	})

	card := e.cards()[0]
	if card.Urgent == nil || card.Urgent.Subject != "Важное" {
		t.Fatalf("без CRITICAL берётся HIGH: %+v", card.Urgent)
	}
}

func TestUrgentIsNullWhenNothingUrgent(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, func(m *messageModel) { m.Level = "NORMAL" })

	if card := e.cards()[0]; card.Urgent != nil {
		t.Fatalf("срочного сообщения нет, поле должно быть пустым: %+v", card.Urgent)
	}
}

func TestUrgentRespectsManualOverride(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, func(m *messageModel) {
		m.Level = "LOW"
		m.LevelOverride = "CRITICAL"
		m.Subject = "Поднято вручную"
	})

	card := e.cards()[0]
	if card.Urgent == nil || card.Urgent.Subject != "Поднято вручную" {
		t.Fatalf("ручное исправление должно учитываться: %+v", card.Urgent)
	}
	if card.Urgent.Level != "CRITICAL" {
		t.Fatalf("уровень в карточке: %q", card.Urgent.Level)
	}
}

func TestEmptySourceCard(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	e.connection(user, "gmail", "active")

	card := e.cards()[0]
	if card.Total != 0 || card.Unread != 0 || card.Urgent != nil {
		t.Fatalf("пустая карточка собрана неверно: %+v", card)
	}
	if len(card.Distribution) != 4 {
		t.Fatalf("полоса распределения должна иметь четыре сегмента: %v", card.Distribution)
	}
}

func TestSourcesRequireAuth(t *testing.T) {
	e := newEnv(t)
	if status, _ := e.do(http.MethodGet, "/api/sources", nil); status != http.StatusUnauthorized {
		t.Fatalf("без входа ожидался 401, получен %d", status)
	}
}
