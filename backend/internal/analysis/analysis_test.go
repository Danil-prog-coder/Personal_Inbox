package analysis

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"personalinbox/internal/events"
	"personalinbox/internal/llm"
	"personalinbox/internal/store"
)

// fakeAnalyzer отвечает по сценарию: сколько раз упасть и что вернуть потом.
type fakeAnalyzer struct {
	failures int
	calls    int
	result   llm.Result
	seen     llm.Request
}

func (f *fakeAnalyzer) Analyze(request llm.Request) (llm.Result, error) {
	f.calls++
	f.seen = request
	if f.calls <= f.failures {
		return llm.Result{}, fmt.Errorf("%w: тестовый сбой", llm.ErrUnavailable)
	}
	return f.result, nil
}

type fakeQueue struct{ ids []int64 }

func (q *fakeQueue) Enqueue(id int64) { q.ids = append(q.ids, id) }

func goodResult() llm.Result {
	return llm.Result{
		Level: "HIGH", Category: "Финансы", Deadline: "Пятница",
		NeedsReply: true, NeedsAction: true, Summary: "Нужны документы.",
	}
}

func newWorker(t *testing.T, analyzer llm.Analyzer) (*Worker, *store.DB, *events.Bus) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("база: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	bus := events.New(50)
	worker := NewWorker(db, bus, analyzer)
	// Тесты не ждут 40 секунд задержек между попытками.
	worker.Sleep = func(time.Duration) {}
	return worker, db, bus
}

func newMessage(t *testing.T, db *store.DB, criteria string, apply func(*store.Message)) (*store.Message, *store.User) {
	t.Helper()
	user, err := db.CreateUser("max@northline.io", "хеш", criteria)
	if err != nil {
		t.Fatal(err)
	}
	connection, err := db.GetOrCreateConnection(user.ID, "gmail")
	if err != nil {
		t.Fatal(err)
	}
	connection.State = "active"
	if err := db.SaveConnection(connection); err != nil {
		t.Fatal(err)
	}
	message := &store.Message{
		ConnectionID: connection.ID,
		ExternalID:   "letter-1",
		SenderName:   "Анна Ковалёва",
		SenderAddr:   "a.kovaleva@northline.io",
		Subject:      "Договор",
		Body:         "Нужны правки",
		ReceivedAt:   store.UTCNow(),
		Status:       "PROCESSING",
	}
	if apply != nil {
		apply(message)
	}
	if err := db.InsertMessage(message); err != nil {
		t.Fatal(err)
	}
	return message, user
}

func TestSuccessfulAnalysisFillsMessage(t *testing.T) {
	analyzer := &fakeAnalyzer{result: goodResult()}
	worker, db, _ := newWorker(t, analyzer)
	message, _ := newMessage(t, db, "Важны договоры.", nil)

	updated, err := worker.ProcessMessage(message.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "DONE" || updated.Level != "HIGH" || updated.Category != "Финансы" {
		t.Fatalf("оценка не записана: %+v", updated)
	}
	if !updated.NeedsReply || !updated.NeedsAction || updated.AnalysisFailed {
		t.Fatalf("признаки записаны неверно: %+v", updated)
	}
	if updated.AnalyzedAt == nil {
		t.Fatal("время оценки не проставлено")
	}
}

func TestRetriesThreeTimesThenGivesUp(t *testing.T) {
	analyzer := &fakeAnalyzer{failures: 5}
	worker, db, _ := newWorker(t, analyzer)
	message, _ := newMessage(t, db, "", nil)

	updated, err := worker.ProcessMessage(message.ID)
	if err != nil {
		t.Fatal(err)
	}
	if analyzer.calls != 3 {
		t.Fatalf("должно быть ровно три попытки, было %d", analyzer.calls)
	}
	if updated.Status != "DONE" || updated.Level != "NORMAL" || !updated.AnalysisFailed {
		t.Fatalf("сообщение должно закрыться как NORMAL с пометкой: %+v", updated)
	}
	if updated.Summary != "" || updated.Category != "" {
		t.Fatalf("после сбоя не должно быть оценки: %+v", updated)
	}
}

func TestSecondAttemptSucceeds(t *testing.T) {
	analyzer := &fakeAnalyzer{failures: 1, result: goodResult()}
	worker, db, _ := newWorker(t, analyzer)
	message, _ := newMessage(t, db, "", nil)

	updated, err := worker.ProcessMessage(message.ID)
	if err != nil {
		t.Fatal(err)
	}
	if analyzer.calls != 2 {
		t.Fatalf("ожидались две попытки, было %d", analyzer.calls)
	}
	if updated.AnalysisFailed || updated.Level != "HIGH" {
		t.Fatalf("успешная вторая попытка не записана: %+v", updated)
	}
}

func TestMissingMessageIsNotAnError(t *testing.T) {
	worker, _, _ := newWorker(t, &fakeAnalyzer{result: goodResult()})
	message, err := worker.ProcessMessage(999)
	if err != nil || message != nil {
		t.Fatalf("исчезнувшее сообщение не должно быть ошибкой: %v, %v", message, err)
	}
}

func TestAnalysisPublishesEvent(t *testing.T) {
	worker, db, bus := newWorker(t, &fakeAnalyzer{result: goodResult()})
	message, user := newMessage(t, db, "", nil)

	if _, err := worker.ProcessMessage(message.ID); err != nil {
		t.Fatal(err)
	}
	published, _ := bus.Since(user.ID, 0)
	if len(published) != 1 || published[0].Name != "message.analyzed" {
		t.Fatalf("событие об оценке не отправлено: %v", published)
	}
}

func TestRequestCarriesCriteriaAndOverrides(t *testing.T) {
	analyzer := &fakeAnalyzer{result: goodResult()}
	worker, db, _ := newWorker(t, analyzer)
	message, _ := newMessage(t, db, "Важны договоры и сроки.", nil)
	if err := db.AddOverrideLog(message.ID, "NORMAL", "CRITICAL"); err != nil {
		t.Fatal(err)
	}

	if _, err := worker.ProcessMessage(message.ID); err != nil {
		t.Fatal(err)
	}
	if analyzer.seen.Criteria != "Важны договоры и сроки." {
		t.Fatalf("критерии не попали в запрос: %q", analyzer.seen.Criteria)
	}
	if analyzer.seen.Source != "gmail" {
		t.Fatalf("источник не попал в запрос: %q", analyzer.seen.Source)
	}
	if len(analyzer.seen.Overrides) != 1 || analyzer.seen.Overrides[0].Level != "CRITICAL" {
		t.Fatalf("ручные исправления не попали в запрос: %v", analyzer.seen.Overrides)
	}
	if analyzer.seen.Sender != "Анна Ковалёва <a.kovaleva@northline.io>" {
		t.Fatalf("отправитель собран неверно: %q", analyzer.seen.Sender)
	}
}

func TestFailedReanalysisKeepsPreviousVerdict(t *testing.T) {
	analyzer := &fakeAnalyzer{failures: 3}
	worker, db, _ := newWorker(t, analyzer)
	analyzed := store.UTCNow()
	message, _ := newMessage(t, db, "", func(m *store.Message) {
		m.Level = "CRITICAL"
		m.Category = "Юридическое"
		m.Summary = "Правки к договору."
		m.AnalyzedAt = &analyzed
	})

	updated, err := worker.ProcessMessage(message.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Level != "CRITICAL" || updated.Category != "Юридическое" {
		t.Fatalf("прежняя оценка затёрта: %+v", updated)
	}
	if !updated.AnalysisFailed {
		t.Fatal("на карточке должна быть пометка «Оценка недоступна»")
	}
}

func TestQueueReanalysisMarksAllProcessing(t *testing.T) {
	worker, db, _ := newWorker(t, &fakeAnalyzer{result: goodResult()})
	_ = worker
	message, user := newMessage(t, db, "", func(m *store.Message) { m.Status = "DONE" })
	queue := &fakeQueue{}

	count, err := QueueReanalysis(db, queue, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || len(queue.ids) != 1 {
		t.Fatalf("переоценка поставила %d сообщений, в очереди %v", count, queue.ids)
	}
	updated, err := db.MessageByID(message.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "PROCESSING" {
		t.Fatalf("статус на переоценке должен быть PROCESSING, получен %q", updated.Status)
	}
}

func TestWorkerRunsQueuedMessages(t *testing.T) {
	analyzer := &fakeAnalyzer{result: goodResult()}
	worker, db, _ := newWorker(t, analyzer)
	message, _ := newMessage(t, db, "", nil)

	worker.Enqueue(message.ID)
	defer worker.Stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		updated, err := db.MessageByID(message.ID)
		if err != nil {
			t.Fatal(err)
		}
		if updated.Status == "DONE" {
			if updated.Level != "HIGH" {
				t.Fatalf("рабочий поток записал не ту оценку: %+v", updated)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("рабочий поток не обработал очередь")
}

func TestUnavailableErrorIsRecognized(t *testing.T) {
	analyzer := &fakeAnalyzer{failures: 1, result: goodResult()}
	if _, err := analyzer.Analyze(llm.Request{}); !errors.Is(err, llm.ErrUnavailable) {
		t.Fatalf("сбой модели должен быть ErrUnavailable: %v", err)
	}
}
