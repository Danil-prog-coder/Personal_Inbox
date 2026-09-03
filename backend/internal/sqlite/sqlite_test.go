package sqlite

import (
	"path/filepath"
	"testing"
	"time"
)

// newDB — отдельная база на каждый тест: файл в каталоге, который t сам уберёт.
func newDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("открыть базу: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func mustUser(t *testing.T, db *DB, email string) *User {
	t.Helper()
	user, err := db.CreateUser(email, "хеш", "Важны договоры.")
	if err != nil {
		t.Fatalf("создать пользователя: %v", err)
	}
	return user
}

func mustConnection(t *testing.T, db *DB, user *User, kind string) *Connection {
	t.Helper()
	connection, err := db.GetOrCreateConnection(user.ID, kind)
	if err != nil {
		t.Fatalf("создать подключение: %v", err)
	}
	connection.State = "active"
	connection.Account = "me@northline.io"
	if err := db.SaveConnection(connection); err != nil {
		t.Fatalf("сохранить подключение: %v", err)
	}
	return connection
}

func mustMessage(t *testing.T, db *DB, connection *Connection, apply func(*Message)) *Message {
	t.Helper()
	message := &Message{
		ConnectionID: connection.ID,
		ExternalID:   "ext-" + time.Now().Format("150405.000000000"),
		SenderName:   "Анна Ковалёва",
		SenderAddr:   "a.kovaleva@northline.io",
		Subject:      "Тема",
		Body:         "Текст сообщения",
		ReceivedAt:   UTCNow(),
		Status:       "DONE",
		Level:        "NORMAL",
		Category:     "Работа",
		Summary:      "Краткое содержание",
		Kind:         connection.Kind,
	}
	if apply != nil {
		apply(message)
	}
	if err := db.InsertMessage(message); err != nil {
		t.Fatalf("создать сообщение: %v", err)
	}
	return message
}

func TestMigrationsAreIdempotent(t *testing.T) {
	db := newDB(t)
	if err := db.Migrate(); err != nil {
		t.Fatalf("повторная миграция: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("ожидалась одна применённая миграция, получено %d", count)
	}
}

func TestUserLookupIgnoresCase(t *testing.T) {
	db := newDB(t)
	mustUser(t, db, "Max@Northline.io")

	user, err := db.UserByEmail("MAX@northline.IO")
	if err != nil {
		t.Fatalf("пользователь не найден: %v", err)
	}
	if user.Email != "max@northline.io" {
		t.Fatalf("email хранится не в нижнем регистре: %q", user.Email)
	}
}

func TestTimeSurvivesRoundTrip(t *testing.T) {
	db := newDB(t)
	user := mustUser(t, db, "max@northline.io")
	connection := mustConnection(t, db, user, "gmail")
	moment := time.Date(2026, 9, 2, 9, 41, 12, 456000000, time.UTC)
	created := mustMessage(t, db, connection, func(m *Message) { m.ReceivedAt = moment })

	loaded, err := db.MessageByID(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.ReceivedAt.Equal(moment) {
		t.Fatalf("время изменилось: %s вместо %s", loaded.ReceivedAt, moment)
	}
	if loaded.Kind != "gmail" {
		t.Fatalf("вид источника не подставлен: %q", loaded.Kind)
	}
}

func TestEffectiveLevelPrefersOverride(t *testing.T) {
	cases := []struct {
		level    string
		override string
		want     string
	}{
		{"LOW", "CRITICAL", "CRITICAL"},
		{"HIGH", "", "HIGH"},
		{"", "", "NORMAL"},
	}
	for _, item := range cases {
		message := &Message{Level: item.level, LevelOverride: item.override}
		if got := message.EffectiveLevel(); got != item.want {
			t.Fatalf("уровень %q/%q дал %q, ожидалось %q",
				item.level, item.override, got, item.want)
		}
	}
}

func TestPeriodAllHasNoBound(t *testing.T) {
	if PeriodStart("all", 0, UTCNow()) != nil {
		t.Fatal("«Всё время» не должно ограничивать выборку")
	}
}

func TestTodayStartsAtLocalMidnight(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 30, 0, 0, time.UTC)
	start := PeriodStart("today", 0, now)
	want := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	if start == nil || !start.Equal(want) {
		t.Fatalf("начало «Сегодня» = %v, ожидалось %v", start, want)
	}
}

func TestTodayRespectsClientTimezone(t *testing.T) {
	// Москва (UTC+3), 01:30 по местному времени — сутки начались в 21:00 UTC.
	now := time.Date(2026, 9, 2, 22, 30, 0, 0, time.UTC)
	start := PeriodStart("today", 180, now)
	want := time.Date(2026, 9, 2, 21, 0, 0, 0, time.UTC)
	if start == nil || !start.Equal(want) {
		t.Fatalf("начало «Сегодня» = %v, ожидалось %v", start, want)
	}
}

func TestAbsurdTimezoneOffsetIsClamped(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	start := PeriodStart("today", 100000, now)
	limit := PeriodStart("today", MaxTZOffsetMinutes, now)
	if start == nil || limit == nil || !start.Equal(*limit) {
		t.Fatalf("смещение не ограничено: %v против %v", start, limit)
	}
}

func TestWeekAndMonthAreRollingWindows(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	if start := PeriodStart("week", 0, now); start == nil || !start.Equal(now.Add(-7*24*time.Hour)) {
		t.Fatalf("окно недели: %v", start)
	}
	if start := PeriodStart("month", 0, now); start == nil || !start.Equal(now.Add(-30*24*time.Hour)) {
		t.Fatalf("окно месяца: %v", start)
	}
}

func TestSummaryWindows(t *testing.T) {
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	cases := map[string]time.Duration{
		"24h":       24 * time.Hour,
		"week":      7 * 24 * time.Hour,
		"month":     30 * 24 * time.Hour,
		"невнятное": 24 * time.Hour,
	}
	for period, window := range cases {
		if got := SummaryPeriodStart(period, now); !got.Equal(now.Add(-window)) {
			t.Fatalf("окно сводки %q = %v", period, got)
		}
	}
}

func TestSearchIsCaseInsensitiveForCyrillic(t *testing.T) {
	db := newDB(t)
	user := mustUser(t, db, "max@northline.io")
	connection := mustConnection(t, db, user, "gmail")
	mustMessage(t, db, connection, func(m *Message) { m.Subject = "Договор Northline" })
	mustMessage(t, db, connection, func(m *Message) { m.Subject = "Счёт за август" })

	found, err := db.FilteredMessages(user.ID, Filter{Q: "ДОГОВОР"})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].Subject != "Договор Northline" {
		t.Fatalf("регистронезависимый поиск по кириллице не работает: %d совпадений", len(found))
	}
}

func TestSearchTreatsWildcardsAsText(t *testing.T) {
	db := newDB(t)
	user := mustUser(t, db, "max@northline.io")
	connection := mustConnection(t, db, user, "gmail")
	mustMessage(t, db, connection, func(m *Message) { m.Subject = "Скидка 50% на всё" })
	mustMessage(t, db, connection, func(m *Message) { m.Subject = "Обычное письмо" })

	found, err := db.FilteredMessages(user.ID, Filter{Q: "50%"})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("процент должен искаться как символ, найдено %d", len(found))
	}

	all, err := db.FilteredMessages(user.ID, Filter{Q: "%"})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("одиночный %% не должен совпадать со всем: найдено %d", len(all))
	}
}

func TestFilterByEffectiveLevelUsesOverride(t *testing.T) {
	db := newDB(t)
	user := mustUser(t, db, "max@northline.io")
	connection := mustConnection(t, db, user, "gmail")
	mustMessage(t, db, connection, func(m *Message) {
		m.Level = "LOW"
		m.LevelOverride = "CRITICAL"
	})
	mustMessage(t, db, connection, func(m *Message) { m.Level = "CRITICAL" })

	found, err := db.FilteredMessages(user.ID, Filter{Level: "CRITICAL"})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 {
		t.Fatalf("ручное исправление должно попадать в фильтр: найдено %d", len(found))
	}

	low, err := db.FilteredMessages(user.ID, Filter{Level: "LOW"})
	if err != nil {
		t.Fatal(err)
	}
	if len(low) != 0 {
		t.Fatalf("исправленное сообщение не должно попадать в старый уровень: %d", len(low))
	}
}

func TestFilterByPeriodUsesReceivedAt(t *testing.T) {
	db := newDB(t)
	user := mustUser(t, db, "max@northline.io")
	connection := mustConnection(t, db, user, "gmail")
	now := UTCNow()
	mustMessage(t, db, connection, func(m *Message) { m.ReceivedAt = now.Add(-2 * time.Hour) })
	mustMessage(t, db, connection, func(m *Message) { m.ReceivedAt = now.Add(-10 * 24 * time.Hour) })

	week, err := db.FilteredMessages(user.ID, Filter{Period: "week", Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if len(week) != 1 {
		t.Fatalf("фильтр «Неделя» должен оставить одно сообщение, оставил %d", len(week))
	}
}

func TestDistributionIncludesZeros(t *testing.T) {
	counts := Distribution([]*Message{{Level: "HIGH"}, {Level: "HIGH"}, {Level: "LOW"}})
	if counts["HIGH"] != 2 || counts["LOW"] != 1 || counts["CRITICAL"] != 0 || counts["NORMAL"] != 0 {
		t.Fatalf("распределение посчитано неверно: %v", counts)
	}
	if len(counts) != 4 {
		t.Fatalf("в полосе должно быть четыре уровня, получено %d", len(counts))
	}
}

func TestRecentOverridesNewestFirstAndLimited(t *testing.T) {
	db := newDB(t)
	user := mustUser(t, db, "max@northline.io")
	connection := mustConnection(t, db, user, "gmail")
	for index := 0; index < 3; index++ {
		message := mustMessage(t, db, connection, func(m *Message) {
			m.Subject = "Тема " + string(rune('А'+index))
		})
		if err := db.AddOverrideLog(message.ID, "NORMAL", "HIGH"); err != nil {
			t.Fatal(err)
		}
	}
	overrides, err := db.RecentOverrides(user.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(overrides) != 2 {
		t.Fatalf("ограничение по количеству не сработало: %d", len(overrides))
	}
	if overrides[0].Subject != "Тема В" {
		t.Fatalf("исправления идут не от новых к старым: %v", overrides)
	}
}

func TestMarkAllProcessingTouchesOnlyOwnMessages(t *testing.T) {
	db := newDB(t)
	owner := mustUser(t, db, "max@northline.io")
	stranger := mustUser(t, db, "other@northline.io")
	own := mustConnection(t, db, owner, "gmail")
	foreign := mustConnection(t, db, stranger, "gmail")
	mustMessage(t, db, own, nil)
	foreignMessage := mustMessage(t, db, foreign, nil)

	ids, err := db.MarkAllProcessing(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 {
		t.Fatalf("на переоценку ушло %d сообщений вместо одного", len(ids))
	}
	untouched, err := db.MessageByID(foreignMessage.ID)
	if err != nil {
		t.Fatal(err)
	}
	if untouched.Status != "DONE" {
		t.Fatal("чужое сообщение не должно уходить на переоценку")
	}
}

func TestDisconnectKeepsMessages(t *testing.T) {
	db := newDB(t)
	user := mustUser(t, db, "max@northline.io")
	connection := mustConnection(t, db, user, "gmail")
	mustMessage(t, db, connection, nil)

	connection.State = "off"
	connection.Credentials = ""
	if err := db.SaveConnection(connection); err != nil {
		t.Fatal(err)
	}
	messages, err := db.MessagesOfConnection(connection.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatal("отключение источника не должно удалять сообщения")
	}
}
