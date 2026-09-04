package routers

import (
	"net/http"
	"testing"

	"personalinbox/internal/schemas"
)

func TestUpdateThemeAndDensity(t *testing.T) {
	e := newEnv(t)
	e.user()
	status, raw := e.do(http.MethodPatch, "/api/me",
		map[string]string{"theme": "light", "density": "compact"})
	if status != http.StatusOK {
		t.Fatalf("обновление профиля вернуло %d: %s", status, raw)
	}
	var result schemas.MeUpdateResult
	e.decode(raw, &result)
	if result.User.Theme != "light" || result.User.Density != "compact" {
		t.Fatalf("настройки не сохранены: %+v", result.User)
	}
	if result.ReanalyzeQueued != 0 {
		t.Fatal("смена темы не должна запускать переоценку")
	}
}

func TestUnknownThemeIsRejected(t *testing.T) {
	e := newEnv(t)
	e.user()
	status, _ := e.do(http.MethodPatch, "/api/me", map[string]string{"theme": "сумерки"})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("неизвестная тема вернула %d", status)
	}
}

func TestCriteriaChangeQueuesReanalysis(t *testing.T) {
	e := newEnv(t)
	user := e.user()
	connection := e.connection(user, "gmail", "active")
	first := e.message(connection, nil)
	second := e.message(connection, nil)

	status, raw := e.do(http.MethodPatch, "/api/me",
		map[string]string{"criteria": "Теперь важны только счета."})
	if status != http.StatusOK {
		t.Fatalf("смена критериев вернула %d: %s", status, raw)
	}
	var result schemas.MeUpdateResult
	e.decode(raw, &result)
	if result.ReanalyzeQueued != 2 {
		t.Fatalf("на переоценку ушло %d сообщений вместо двух", result.ReanalyzeQueued)
	}
	if len(e.queue.ids) != 2 {
		t.Fatalf("очередь оценки получила %v", e.queue.ids)
	}
	for _, id := range []int64{first.ID, second.ID} {
		message, err := e.db.MessageByID(id)
		if err != nil {
			t.Fatal(err)
		}
		if message.Status != "PROCESSING" {
			t.Fatalf("сообщение %d не ушло на переоценку: %q", id, message.Status)
		}
	}
}

func TestManualOverrideSurvivesReanalysis(t *testing.T) {
	e := newEnv(t)
	user := e.user()
	connection := e.connection(user, "gmail", "active")
	message := e.message(connection, func(m *messageModel) {
		m.Level = "LOW"
		m.LevelOverride = "CRITICAL"
	})

	if status, _ := e.do(http.MethodPatch, "/api/me",
		map[string]string{"criteria": "Другие критерии."}); status != http.StatusOK {
		t.Fatal("смена критериев не удалась")
	}
	updated, err := e.db.MessageByID(message.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.LevelOverride != "CRITICAL" {
		t.Fatalf("ручное исправление затёрто переоценкой: %q", updated.LevelOverride)
	}
}

func TestSameCriteriaDoesNotQueueReanalysis(t *testing.T) {
	e := newEnv(t)
	user := e.user()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, nil)

	status, raw := e.do(http.MethodPatch, "/api/me",
		map[string]string{"criteria": "Важны договоры и сроки."})
	if status != http.StatusOK {
		t.Fatalf("запрос вернул %d", status)
	}
	var result schemas.MeUpdateResult
	e.decode(raw, &result)
	if result.ReanalyzeQueued != 0 || len(e.queue.ids) != 0 {
		t.Fatal("те же критерии не должны запускать переоценку")
	}
}

// Ни входа, ни cookie: чистая установка сразу отвечает профилем,
// а не требованием войти (решение №50).
func TestProfileWorksWithoutAnyLogin(t *testing.T) {
	e := newEnv(t)
	status, _ := e.do(http.MethodPatch, "/api/me", map[string]string{"theme": "light"})
	if status != http.StatusOK {
		t.Fatalf("PATCH /api/me без входа вернул %d", status)
	}

	var profile schemas.User
	status, raw := e.do(http.MethodGet, "/api/me", nil)
	if status != http.StatusOK {
		t.Fatalf("GET /api/me без входа вернул %d", status)
	}
	e.decode(raw, &profile)
	if profile.Theme != "light" {
		t.Fatalf("тема не сохранилась: %q", profile.Theme)
	}
	if profile.ID == 0 {
		t.Fatal("профиль пустой: пользователь не завёлся сам")
	}
}
