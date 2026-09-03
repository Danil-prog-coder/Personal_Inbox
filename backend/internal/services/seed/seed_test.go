package seed

import (
	"strings"
	"testing"
	"time"

	"personalinbox/internal/postgres"
	"personalinbox/internal/testenv"
)

func newDB(t *testing.T) *postgres.DB {
	t.Helper()
	db := testenv.DB(t)
	return db
}

func TestReferenceSetIsComplete(t *testing.T) {
	if len(Messages) != 19 {
		t.Fatalf("в демо-ленте должно быть 19 сообщений, их %d", len(Messages))
	}
	gmail, telegram := 0, 0
	for _, item := range Messages {
		switch item.Src {
		case "gmail":
			gmail++
		case "tg":
			telegram++
		default:
			t.Fatalf("неизвестный источник в демо-данных: %q", item.Src)
		}
		if !postgres.Contains(postgres.Levels, item.Level) {
			t.Fatalf("неизвестный уровень в демо-данных: %q", item.Level)
		}
	}
	if gmail != 11 || telegram != 8 {
		t.Fatalf("разбивка по источникам: %d Gmail и %d Telegram", gmail, telegram)
	}
	if len(LiveQueue) != 3 {
		t.Fatalf("в очереди живой демонстрации должно быть 3 сообщения, их %d", len(LiveQueue))
	}
}

func TestSeedKeepsTextsVerbatim(t *testing.T) {
	first := Messages[0]
	if first.Subj != "Договор Northline — правки до конца дня" {
		t.Fatalf("тема первого сообщения изменена: %q", first.Subj)
	}
	if !strings.HasPrefix(first.Text, "Юристы вернули версию с комментариями.") {
		t.Fatalf("текст первого сообщения изменён: %q", first.Text)
	}
	if first.Cat != "Юридическое" || first.Deadline != "Сегодня, 18:00" {
		t.Fatalf("оценка первого сообщения изменена: %+v", first)
	}
	group := Messages[11]
	if group.Addr != "групповой чат, 9 участников" {
		t.Fatalf("адрес группового чата изменён: %q", group.Addr)
	}
}

func TestSeedCreatesUserConnectionsAndMessages(t *testing.T) {
	db := newDB(t)
	created, err := Seed(db, "хеш", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if created != 19 {
		t.Fatalf("залито %d сообщений вместо 19", created)
	}

	user, err := db.UserByEmail(DemoEmail)
	if err != nil {
		t.Fatalf("демо-пользователь не создан: %v", err)
	}
	if user.Criteria == "" {
		t.Fatal("у демо-пользователя должны быть критерии важности")
	}
	connections, err := db.ConnectionsOf(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 2 {
		t.Fatalf("подключений должно быть два, их %d", len(connections))
	}
	for _, connection := range connections {
		if connection.State != States[connection.Kind] {
			t.Fatalf("состояние %s: %q вместо %q",
				connection.Kind, connection.State, States[connection.Kind])
		}
	}
}

func TestSeedIsIdempotent(t *testing.T) {
	db := newDB(t)
	if _, err := Seed(db, "хеш", time.Time{}); err != nil {
		t.Fatal(err)
	}
	created, err := Seed(db, "хеш", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if created != 0 {
		t.Fatalf("повторный запуск добавил %d сообщений", created)
	}
}

func TestSeededMessagesAreAnalyzed(t *testing.T) {
	db := newDB(t)
	if _, err := Seed(db, "хеш", time.Time{}); err != nil {
		t.Fatal(err)
	}
	user, err := db.UserByEmail(DemoEmail)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := db.MessagesOfUser(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range messages {
		if message.Status != "DONE" || message.AnalyzedAt == nil {
			t.Fatalf("демо-сообщение должно быть уже оценено: %+v", message)
		}
	}
}

func TestGroupChatMessageHasNoLink(t *testing.T) {
	db := newDB(t)
	if _, err := Seed(db, "хеш", time.Time{}); err != nil {
		t.Fatal(err)
	}
	user, err := db.UserByEmail(DemoEmail)
	if err != nil {
		t.Fatal(err)
	}
	messages, err := db.MessagesOfUser(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range messages {
		if strings.HasPrefix(message.SenderAddr, "групповой чат") && message.ExternalURL != "" {
			t.Fatalf("у группового чата не должно быть ссылки: %q", message.ExternalURL)
		}
		if message.SenderAddr == "@dmitry_pm" && message.ExternalURL != "https://t.me/dmitry_pm" {
			t.Fatalf("ссылка на личный чат собрана неверно: %q", message.ExternalURL)
		}
	}
}

func TestParseTimeToday(t *testing.T) {
	now := time.Date(2026, 9, 2, 15, 0, 0, 0, time.UTC)
	got := ParseTime("09:41", now)
	want := time.Date(2026, 9, 2, 9, 41, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("«09:41» разобрано как %v", got)
	}
}

func TestParseTimeYesterday(t *testing.T) {
	now := time.Date(2026, 9, 2, 15, 0, 0, 0, time.UTC)
	got := ParseTime("Вчера, 19:04", now)
	want := time.Date(2026, 9, 1, 19, 4, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("«Вчера, 19:04» разобрано как %v", got)
	}
}

func TestParseTimeWeekdayLooksBack(t *testing.T) {
	// 2 сентября 2026 — среда; «Пн» это два дня назад.
	now := time.Date(2026, 9, 2, 15, 0, 0, 0, time.UTC)
	got := ParseTime("Пн", now)
	want := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("«Пн» разобрано как %v, ожидалось %v", got, want)
	}
}

func TestParseTimeSameWeekdayIsAWeekAgo(t *testing.T) {
	now := time.Date(2026, 9, 2, 15, 0, 0, 0, time.UTC) // среда
	got := ParseTime("Ср", now)
	want := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("тот же день недели должен быть неделей раньше: %v", got)
	}
}

func TestParseTimeUnknownValue(t *testing.T) {
	now := time.Date(2026, 9, 2, 15, 0, 0, 0, time.UTC)
	got := ParseTime("когда-нибудь", now)
	want := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("непонятное время должно давать полдень: %v", got)
	}
}
