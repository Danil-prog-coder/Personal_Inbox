package routers

import (
	"fmt"
	"net/http"
	"testing"
)

// Кэш источников и сводки живёт 45 секунд, поэтому внутри одного теста он
// заведомо ещё горячий. Эти тесты проверяют не сам кэш, а инвалидацию: после
// записи пользователь обязан увидеть новые числа, а не вчерашние.

func TestMarkReadInvalidatesSourceCounters(t *testing.T) {
	e := newEnv(t)
	user := e.user()
	connection := e.connection(user, "gmail", "active")
	message := e.message(connection, func(m *messageModel) { m.IsRead = false })
	e.message(connection, func(m *messageModel) { m.IsRead = false })

	// Первый запрос кладёт карточки в кэш.
	if unread := e.cards()[0].Unread; unread != 2 {
		t.Fatalf("до прочтения непрочитанных %d, ожидали 2", unread)
	}

	status, raw := e.do(http.MethodPost, fmt.Sprintf("/api/messages/%d/read", message.ID), nil)
	if status != http.StatusOK {
		t.Fatalf("отметка о прочтении вернула %d: %s", status, raw)
	}

	if unread := e.cards()[0].Unread; unread != 1 {
		t.Fatalf("после прочтения непрочитанных %d, ожидали 1 — кэш не сброшен", unread)
	}
}

func TestLevelChangeInvalidatesSummary(t *testing.T) {
	e := newEnv(t)
	user := e.user()
	connection := e.connection(user, "gmail", "active")
	message := e.message(connection, func(m *messageModel) { m.Level = "LOW" })

	first := e.summary("24h")
	if first.Distribution["LOW"] != 1 || first.Distribution["CRITICAL"] != 0 {
		t.Fatalf("исходное распределение: %v", first.Distribution)
	}

	status, raw := e.do(http.MethodPost, fmt.Sprintf("/api/messages/%d/level", message.ID),
		map[string]string{"level": "CRITICAL"})
	if status != http.StatusOK {
		t.Fatalf("смена уровня вернула %d: %s", status, raw)
	}

	second := e.summary("24h")
	if second.Distribution["CRITICAL"] != 1 || second.Distribution["LOW"] != 0 {
		t.Fatalf("после смены уровня распределение %v — кэш сводки не сброшен",
			second.Distribution)
	}
}

func TestNewMessageInvalidatesSourceCounters(t *testing.T) {
	e := newEnv(t)
	user := e.user()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, func(m *messageModel) {})

	if total := e.cards()[0].Total; total != 1 {
		t.Fatalf("до приёма всего %d, ожидали 1", total)
	}

	// Приём сообщения публикует событие в шину — на нём и висит сброс кэша.
	e.message(connection, func(m *messageModel) {})
	e.bus.Publish(user.ID, "message.created", nil)

	if total := e.cards()[0].Total; total != 2 {
		t.Fatalf("после приёма всего %d, ожидали 2 — кэш не сброшен", total)
	}
}
