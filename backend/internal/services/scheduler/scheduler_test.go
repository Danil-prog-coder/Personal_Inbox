package scheduler

import (
	"testing"
	"time"

	"personalinbox/internal/events"
	"personalinbox/internal/gmail"
	"personalinbox/internal/postgres"
	"personalinbox/internal/services/ingest"
	"personalinbox/internal/telegram"
	"personalinbox/internal/testenv"
)

type fakeQueue struct{ ids []int64 }

func (q *fakeQueue) Enqueue(id int64) { q.ids = append(q.ids, id) }

func newScheduler(t *testing.T) (*Scheduler, *postgres.DB, *postgres.User) {
	t.Helper()
	db := testenv.DB(t)
	user, err := db.CreateUser("max@northline.io", "хеш", "")
	if err != nil {
		t.Fatal(err)
	}
	ingestor := ingest.New(db, events.New(10), &fakeQueue{})
	// Адреса заведомо недоступны: важно, что планировщик переживает сбой.
	return New(ingestor, &gmail.Client{}, &telegram.Client{BaseURL: "http://127.0.0.1:1"}), db, user
}

func connect(t *testing.T, db *postgres.DB, user *postgres.User, kind, state string) {
	t.Helper()
	connection, err := db.GetOrCreateConnection(user.ID, kind)
	if err != nil {
		t.Fatal(err)
	}
	connection.State = state
	connection.Credentials = `{"bot_token": "123:abc", "refresh_token": "тест"}`
	if err := db.SaveConnection(connection); err != nil {
		t.Fatal(err)
	}
}

func TestSyncAllSurvivesBrokenSource(t *testing.T) {
	sync, db, user := newScheduler(t)
	connect(t, db, user, "telegram", "active")
	connect(t, db, user, "gmail", "active")

	// Оба источника недоступны — обход должен завершиться без паники.
	if saved := sync.SyncAll(); saved != 0 {
		t.Fatalf("недоступные источники не могли ничего сохранить: %d", saved)
	}
}

func TestSyncAllSkipsInactiveConnections(t *testing.T) {
	sync, db, user := newScheduler(t)
	connect(t, db, user, "telegram", "off")

	if saved := sync.SyncAll(); saved != 0 {
		t.Fatalf("отключённые источники не синхронизируются: %d", saved)
	}
	connection, err := db.Connection(user.ID, "telegram")
	if err != nil {
		t.Fatal(err)
	}
	if connection.State != "off" {
		t.Fatalf("состояние отключённого источника изменилось: %q", connection.State)
	}
}

func TestStopIsIdempotent(t *testing.T) {
	sync, _, _ := newScheduler(t)
	sync.interval = 10 * time.Millisecond
	sync.Start()
	sync.Start()
	sync.Stop()
	sync.Stop()
}
