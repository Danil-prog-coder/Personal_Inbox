package ingest

import (
	"testing"
	"time"

	"personalinbox/internal/events"
	"personalinbox/internal/postgres"
	"personalinbox/internal/testenv"
)

// fakeQueue заменяет рабочий поток: в тестах важно, что сообщение поставлено
// в очередь, а не то, что модель ответила.
type fakeQueue struct{ ids []int64 }

func (q *fakeQueue) Enqueue(id int64) { q.ids = append(q.ids, id) }

func newIngestor(t *testing.T) (*Ingestor, *postgres.DB, *events.Bus, *fakeQueue) {
	t.Helper()
	db := testenv.DB(t)
	bus := events.New(50)
	queue := &fakeQueue{}
	return New(db, bus, queue), db, bus, queue
}

func newConnection(t *testing.T, db *postgres.DB, email, kind string) *postgres.Connection {
	t.Helper()
	user, err := db.CreateUser(email, "хеш", "")
	if err != nil {
		t.Fatal(err)
	}
	connection, err := db.GetOrCreateConnection(user.ID, kind)
	if err != nil {
		t.Fatal(err)
	}
	connection.State = "active"
	if err := db.SaveConnection(connection); err != nil {
		t.Fatal(err)
	}
	return connection
}

func incoming(externalID string) Incoming {
	return Incoming{
		ExternalID: externalID,
		SenderName: "Анна Ковалёва",
		SenderAddr: "a.kovaleva@northline.io",
		Subject:    "Договор",
		Body:       "Нужны правки",
		ReceivedAt: time.Date(2026, 9, 2, 9, 41, 0, 0, time.UTC),
	}
}

func TestStoreCreatesProcessingMessage(t *testing.T) {
	ingestor, _, _, queue := newIngestor(t)
	connection := newConnection(t, ingestor.DB, "max@northline.io", "gmail")

	message, err := ingestor.Store(connection, incoming("letter-1"))
	if err != nil {
		t.Fatal(err)
	}
	if message == nil {
		t.Fatal("сообщение не сохранено")
	}
	if message.Status != "PROCESSING" {
		t.Fatalf("новое сообщение должно ждать оценки, статус %q", message.Status)
	}
	if message.IsRead {
		t.Fatal("новое сообщение не может быть прочитанным")
	}
	if len(queue.ids) != 1 || queue.ids[0] != message.ID {
		t.Fatalf("сообщение не поставлено в очередь оценки: %v", queue.ids)
	}
}

func TestStorePublishesCreatedEvent(t *testing.T) {
	ingestor, _, bus, _ := newIngestor(t)
	connection := newConnection(t, ingestor.DB, "max@northline.io", "gmail")

	if _, err := ingestor.Store(connection, incoming("letter-1")); err != nil {
		t.Fatal(err)
	}
	published, _ := bus.Since(connection.UserID, 0)
	if len(published) != 1 || published[0].Name != "message.created" {
		t.Fatalf("событие о новой карточке не отправлено: %v", published)
	}
}

func TestDuplicateExternalIDIsSkipped(t *testing.T) {
	ingestor, _, _, queue := newIngestor(t)
	connection := newConnection(t, ingestor.DB, "max@northline.io", "gmail")

	if _, err := ingestor.Store(connection, incoming("letter-1")); err != nil {
		t.Fatal(err)
	}
	again, err := ingestor.Store(connection, incoming("letter-1"))
	if err != nil {
		t.Fatal(err)
	}
	if again != nil {
		t.Fatal("повторная синхронизация не должна дублировать сообщение")
	}
	if len(queue.ids) != 1 {
		t.Fatalf("дубль ушёл в очередь оценки: %v", queue.ids)
	}
}

func TestSameExternalIDInOtherConnectionIsStored(t *testing.T) {
	ingestor, db, _, _ := newIngestor(t)
	first := newConnection(t, db, "max@northline.io", "gmail")
	second := newConnection(t, db, "other@northline.io", "gmail")

	if _, err := ingestor.Store(first, incoming("letter-1")); err != nil {
		t.Fatal(err)
	}
	message, err := ingestor.Store(second, incoming("letter-1"))
	if err != nil {
		t.Fatal(err)
	}
	if message == nil {
		t.Fatal("одинаковый внешний id в разных подключениях — разные сообщения")
	}
}

func TestStoreWithoutAnalysis(t *testing.T) {
	ingestor, _, _, queue := newIngestor(t)
	connection := newConnection(t, ingestor.DB, "max@northline.io", "gmail")

	if _, err := ingestor.StoreWithoutAnalysis(connection, incoming("letter-1")); err != nil {
		t.Fatal(err)
	}
	if len(queue.ids) != 0 {
		t.Fatalf("оценка не должна запускаться: %v", queue.ids)
	}
}
