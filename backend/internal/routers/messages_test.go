package routers

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"personalinbox/internal/schemas"
	"personalinbox/internal/sqlite"
)

func (e *env) list(query string) schemas.MessageList {
	e.t.Helper()
	status, raw := e.do(http.MethodGet, "/api/messages"+query, nil)
	if status != http.StatusOK {
		e.t.Fatalf("лента вернула %d: %s", status, raw)
	}
	var result schemas.MessageList
	e.decode(raw, &result)
	return result
}

func TestListReturnsNewestFirst(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	older := e.message(connection, func(m *messageModel) {
		m.Subject = "Старое"
		m.ReceivedAt = sqlite.UTCNow().Add(-2 * time.Hour)
	})
	newer := e.message(connection, func(m *messageModel) {
		m.Subject = "Новое"
		m.ReceivedAt = sqlite.UTCNow()
	})

	result := e.list("")
	if result.Total != 2 {
		t.Fatalf("в ленте %d сообщений вместо двух", result.Total)
	}
	if result.Items[0].ID != newer.ID || result.Items[1].ID != older.ID {
		t.Fatal("сообщения идут не от новых к старым")
	}
}

func TestListCountsUnread(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, func(m *messageModel) { m.IsRead = true })
	e.message(connection, func(m *messageModel) { m.IsRead = false })

	result := e.list("")
	if result.Total != 2 || result.Unread != 1 {
		t.Fatalf("счётчики подзаголовка: всего %d, непрочитанных %d", result.Total, result.Unread)
	}
}

func TestFilterBySource(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	gmailConnection := e.connection(user, "gmail", "active")
	telegramConnection := e.connection(user, "telegram", "active")
	e.message(gmailConnection, nil)
	e.message(telegramConnection, nil)

	if result := e.list("?source=gmail"); result.Total != 1 || result.Items[0].Source != "gmail" {
		t.Fatalf("фильтр по источнику вернул %d сообщений", result.Total)
	}
}

func TestFilterByLevelUsesOverride(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, func(m *messageModel) {
		m.Level = "LOW"
		m.LevelOverride = "CRITICAL"
	})
	e.message(connection, func(m *messageModel) { m.Level = "LOW" })

	if result := e.list("?level=CRITICAL"); result.Total != 1 {
		t.Fatalf("фильтр по уровню должен учитывать исправление: %d", result.Total)
	}
	if result := e.list("?level=LOW"); result.Total != 1 {
		t.Fatalf("исправленное сообщение не должно оставаться в LOW: %d", result.Total)
	}
}

func TestFilterByStatus(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, func(m *messageModel) { m.IsRead = true })
	e.message(connection, func(m *messageModel) { m.Status = "PROCESSING" })

	if result := e.list("?status=unread"); result.Total != 1 {
		t.Fatalf("непрочитанных должно быть одно: %d", result.Total)
	}
	if result := e.list("?status=read"); result.Total != 1 {
		t.Fatalf("прочитанных должно быть одно: %d", result.Total)
	}
	if result := e.list("?status=done"); result.Total != 1 {
		t.Fatalf("оценённых должно быть одно: %d", result.Total)
	}
}

func TestFilterByReplyAndAction(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, func(m *messageModel) {
		m.NeedsReply = true
		m.NeedsAction = false
	})
	e.message(connection, func(m *messageModel) {
		m.NeedsReply = false
		m.NeedsAction = true
	})

	if result := e.list("?reply=yes"); result.Total != 1 {
		t.Fatalf("фильтр «требует ответа»: %d", result.Total)
	}
	if result := e.list("?action=yes"); result.Total != 1 {
		t.Fatalf("фильтр «требует действия»: %d", result.Total)
	}
	if result := e.list("?reply=no&action=no"); result.Total != 0 {
		t.Fatalf("комбинация «нет и нет»: %d", result.Total)
	}
}

func TestFilterByPeriod(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, func(m *messageModel) { m.ReceivedAt = sqlite.UTCNow().Add(-time.Hour) })
	e.message(connection, func(m *messageModel) {
		m.ReceivedAt = sqlite.UTCNow().Add(-40 * 24 * time.Hour)
	})

	if result := e.list("?period=week"); result.Total != 1 {
		t.Fatalf("окно недели: %d сообщений", result.Total)
	}
	if result := e.list("?period=all"); result.Total != 2 {
		t.Fatalf("«Всё время» должно показывать всё: %d", result.Total)
	}
}

func TestSearchCoversSenderSubjectAndBody(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, func(m *messageModel) { m.SenderName = "Кирилл Ушаков" })
	e.message(connection, func(m *messageModel) { m.Subject = "Выписка по счёту" })
	e.message(connection, func(m *messageModel) { m.Body = "Питч-дек для фонда" })

	for query, want := range map[string]int{"ушаков": 1, "ВЫПИСКА": 1, "питч": 1, "нетутакого": 0} {
		if result := e.list("?q=" + query); result.Total != want {
			t.Fatalf("поиск %q нашёл %d вместо %d", query, result.Total, want)
		}
	}
}

func TestBlankSearchIsIgnored(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, nil)

	if result := e.list("?q=%20%20"); result.Total != 1 {
		t.Fatalf("пробельный запрос не должен фильтровать: %d", result.Total)
	}
}

func TestFiltersCombine(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, func(m *messageModel) {
		m.Level = "CRITICAL"
		m.NeedsReply = true
		m.Subject = "Договор"
	})
	e.message(connection, func(m *messageModel) {
		m.Level = "CRITICAL"
		m.NeedsReply = false
		m.Subject = "Договор"
	})

	if result := e.list("?level=CRITICAL&reply=yes&q=договор"); result.Total != 1 {
		t.Fatalf("комбинация фильтров вернула %d", result.Total)
	}
}

func TestUnknownFilterValueIsRejected(t *testing.T) {
	e := newEnv(t)
	e.authorized()
	for _, query := range []string{"?level=СРОЧНО", "?status=любой", "?reply=может",
		"?period=квартал", "?source=slack", "?tz_offset=много"} {
		if status, _ := e.do(http.MethodGet, "/api/messages"+query, nil); status != http.StatusUnprocessableEntity {
			t.Fatalf("значение %q принято со статусом %d", query, status)
		}
	}
}

func TestGetMessageDetails(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	message := e.message(connection, nil)

	status, raw := e.do(http.MethodGet, fmt.Sprintf("/api/messages/%d", message.ID), nil)
	if status != http.StatusOK {
		t.Fatalf("детали вернули %d: %s", status, raw)
	}
	var details schemas.Message
	e.decode(raw, &details)
	if details.ID != message.ID || details.Body != message.Body || details.Source != "gmail" {
		t.Fatalf("детали собраны неверно: %+v", details)
	}
}

func TestGetMessageOfAnotherUserIs404(t *testing.T) {
	e := newEnv(t)
	stranger := e.user("other@northline.io")
	foreign := e.connection(stranger, "gmail", "active")
	message := e.message(foreign, nil)
	e.authorized()

	status, _ := e.do(http.MethodGet, fmt.Sprintf("/api/messages/%d", message.ID), nil)
	if status != http.StatusNotFound {
		t.Fatalf("чужое сообщение вернуло %d", status)
	}
}

func TestMarkReadIsIdempotent(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	message := e.message(connection, nil)

	for range 2 {
		status, raw := e.do(http.MethodPost, fmt.Sprintf("/api/messages/%d/read", message.ID), nil)
		if status != http.StatusOK {
			t.Fatalf("пометка прочитанным вернула %d", status)
		}
		var updated schemas.Message
		e.decode(raw, &updated)
		if !updated.IsRead {
			t.Fatal("сообщение должно стать прочитанным")
		}
	}
}

func TestSetLevelWritesOverrideLog(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	message := e.message(connection, func(m *messageModel) { m.Level = "NORMAL" })

	status, raw := e.do(http.MethodPost, fmt.Sprintf("/api/messages/%d/level", message.ID),
		map[string]string{"level": "CRITICAL"})
	if status != http.StatusOK {
		t.Fatalf("исправление уровня вернуло %d: %s", status, raw)
	}
	var updated schemas.Message
	e.decode(raw, &updated)
	if updated.Level != "CRITICAL" || updated.LevelOverride == nil || *updated.LevelOverride != "CRITICAL" {
		t.Fatalf("исправление не отражено в ответе: %+v", updated)
	}

	count, err := e.db.CountOverrides(message.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("в журнале %d записей вместо одной", count)
	}

	published, _ := e.bus.Since(user.ID, 0)
	if len(published) != 1 || published[0].Name != "message.analyzed" {
		t.Fatalf("исправление должно уезжать в поток: %v", published)
	}
}

func TestSetSameLevelDoesNotDuplicateLog(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	message := e.message(connection, func(m *messageModel) { m.Level = "HIGH" })

	if status, _ := e.do(http.MethodPost, fmt.Sprintf("/api/messages/%d/level", message.ID),
		map[string]string{"level": "HIGH"}); status != http.StatusOK {
		t.Fatal("запрос не удался")
	}
	count, err := e.db.CountOverrides(message.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("тот же уровень не должен писаться в журнал: %d записей", count)
	}
}

func TestSetLevelRejectsUnknownValue(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	message := e.message(connection, nil)

	status, _ := e.do(http.MethodPost, fmt.Sprintf("/api/messages/%d/level", message.ID),
		map[string]string{"level": "СРОЧНО"})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("неизвестный уровень вернул %d", status)
	}
}

func TestMessagesRequireAuth(t *testing.T) {
	e := newEnv(t)
	status, _ := e.do(http.MethodGet, "/api/messages", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("лента без входа вернула %d", status)
	}
}

func TestMessageTimeFormatMatchesFrontend(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, func(m *messageModel) {
		m.ReceivedAt = time.Date(2026, 9, 2, 9, 41, 0, 0, time.UTC)
	})

	_, raw := e.do(http.MethodGet, "/api/messages", nil)
	// Фронт ждёт ISO без зоны и дописывает «Z» сам (frontend/src/lib/format.ts).
	if !strings.Contains(string(raw), `"received_at":"2026-09-02T09:41:00"`) {
		t.Fatalf("формат времени изменился: %s", raw)
	}
}
