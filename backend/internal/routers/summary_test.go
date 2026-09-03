package routers

import (
	"net/http"
	"testing"
	"time"

	"personalinbox/internal/postgres"
	"personalinbox/internal/schemas"
)

func (e *env) summary(period string) schemas.Summary {
	e.t.Helper()
	status, raw := e.do(http.MethodGet, "/api/summary?period="+period, nil)
	if status != http.StatusOK {
		e.t.Fatalf("сводка вернула %d: %s", status, raw)
	}
	var result schemas.Summary
	e.decode(raw, &result)
	return result
}

func TestSummaryWindow(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, func(m *messageModel) { m.ReceivedAt = postgres.UTCNow().Add(-time.Hour) })
	e.message(connection, func(m *messageModel) {
		m.ReceivedAt = postgres.UTCNow().Add(-48 * time.Hour)
	})

	if result := e.summary("24h"); result.Total != 1 {
		t.Fatalf("окно 24 часов вернуло %d сообщений", result.Total)
	}
	if result := e.summary("week"); result.Total != 2 {
		t.Fatalf("окно недели вернуло %d сообщений", result.Total)
	}
}

func TestSummaryCountsReplyAndAction(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, func(m *messageModel) {
		m.NeedsReply = true
		m.NeedsAction = true
	})
	e.message(connection, func(m *messageModel) { m.NeedsReply = true })

	result := e.summary("24h")
	if result.NeedsReply != 2 || result.NeedsAction != 1 {
		t.Fatalf("счётчики сводки: ответ %d, действие %d", result.NeedsReply, result.NeedsAction)
	}
}

func TestSummaryCoversAllSources(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	e.message(e.connection(user, "gmail", "active"), nil)
	e.message(e.connection(user, "telegram", "active"), nil)

	if result := e.summary("24h"); result.Total != 2 {
		t.Fatalf("сводка считается по всем источникам сразу: %d", result.Total)
	}
}

func TestTopListsCriticalFirstAndLimitsToFour(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	for range 3 {
		e.message(connection, func(m *messageModel) { m.Level = "HIGH" })
	}
	for range 2 {
		e.message(connection, func(m *messageModel) { m.Level = "CRITICAL" })
	}

	result := e.summary("24h")
	if len(result.Top) != 4 {
		t.Fatalf("«Главное за период» показывает до четырёх сообщений: %d", len(result.Top))
	}
	if result.Top[0].Level != "CRITICAL" || result.Top[1].Level != "CRITICAL" {
		t.Fatalf("критичные должны идти первыми: %+v", result.Top)
	}
	if result.Top[2].Level != "HIGH" {
		t.Fatalf("после критичных идут важные: %+v", result.Top)
	}
}

func TestSummaryRejectsUnknownPeriod(t *testing.T) {
	e := newEnv(t)
	e.authorized()
	status, _ := e.do(http.MethodGet, "/api/summary?period=квартал", nil)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("неизвестный период вернул %d", status)
	}
}

func TestSummaryDefaultsTo24h(t *testing.T) {
	e := newEnv(t)
	e.authorized()
	status, raw := e.do(http.MethodGet, "/api/summary", nil)
	if status != http.StatusOK {
		t.Fatalf("сводка без параметра вернула %d", status)
	}
	var result schemas.Summary
	e.decode(raw, &result)
	if result.Period != "24h" {
		t.Fatalf("период по умолчанию: %q", result.Period)
	}
}

func TestSummaryRequiresAuth(t *testing.T) {
	e := newEnv(t)
	if status, _ := e.do(http.MethodGet, "/api/summary", nil); status != http.StatusUnauthorized {
		t.Fatalf("без входа ожидался 401, получен %d", status)
	}
}
