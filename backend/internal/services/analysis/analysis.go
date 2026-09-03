// Package analysis — оценка сообщений моделью: очередь, повторы, фоновая
// переоценка. Один рабочий поток на процесс, этого достаточно
// (docs/04-decisions.md, решение №2).
//
// Сообщение никогда не пропадает из ленты из-за ошибки модели: после трёх
// неудач оно закрывается как NORMAL с пометкой «Оценка недоступна».
package analysis

import (
	"errors"
	"log"
	"personalinbox/internal/exceptions"
	"sync"
	"time"

	"personalinbox/internal/core"
	"personalinbox/internal/events"
	"personalinbox/internal/openai"
	"personalinbox/internal/postgres"
	"personalinbox/internal/schemas"
)

// Enqueuer — то, куда ручки складывают сообщения на оценку.
// В тестах подставляется список, а не рабочий поток.
type Enqueuer interface {
	Enqueue(messageID int64)
}

// queue — неограниченная очередь id: переоценка ставит в неё сразу все
// сообщения пользователя, и терять их из-за размера буфера нельзя.
type queue struct {
	mu     sync.Mutex
	cond   *sync.Cond
	items  []int64
	closed bool
}

func newQueue() *queue {
	q := &queue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *queue) push(id int64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, id)
	q.cond.Signal()
}

// pop ждёт следующий id. Второе значение — false, если очередь закрыта.
func (q *queue) pop() (int64, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.items) == 0 {
		return 0, false
	}
	id := q.items[0]
	q.items = q.items[1:]
	return id, true
}

func (q *queue) close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	q.cond.Broadcast()
}

// Worker — рабочий поток оценки.
type Worker struct {
	db       *postgres.DB
	bus      *events.Bus
	analyzer openai.Analyzer

	// Sleep и RetryDelays вынесены наружу, чтобы тесты не ждали 40 секунд.
	Sleep       func(time.Duration)
	RetryDelays []time.Duration

	queue   *queue
	once    sync.Once
	done    chan struct{}
	stopped sync.Once
}

// NewWorker собирает воркер поверх базы, шины событий и адаптера модели.
func NewWorker(db *postgres.DB, bus *events.Bus, analyzer openai.Analyzer) *Worker {
	return &Worker{
		db:          db,
		bus:         bus,
		analyzer:    analyzer,
		Sleep:       time.Sleep,
		RetryDelays: core.LLMRetryDelays,
		queue:       newQueue(),
		done:        make(chan struct{}),
	}
}

// Enqueue ставит сообщение в очередь на оценку и поднимает поток, если он спит.
func (w *Worker) Enqueue(messageID int64) {
	w.queue.push(messageID)
	w.Start()
}

// Start запускает рабочий поток. Повторный вызов ничего не делает.
func (w *Worker) Start() {
	w.once.Do(func() {
		go w.run()
	})
}

// Stop останавливает поток: нужен корректному завершению сервера и тестам.
func (w *Worker) Stop() {
	w.stopped.Do(func() {
		w.queue.close()
	})
}

func (w *Worker) run() {
	defer close(w.done)
	for {
		id, ok := w.queue.pop()
		if !ok {
			return
		}
		if _, err := w.ProcessMessage(id); err != nil {
			log.Printf("не удалось оценить сообщение %d: %v", id, err)
		}
	}
}

// BuildRequest собирает то, что уходит в модель об одном сообщении.
func (w *Worker) BuildRequest(message *postgres.Message) (openai.Request, error) {
	connection, err := w.db.ConnectionByID(message.ConnectionID)
	if err != nil {
		return openai.Request{}, err
	}
	user, err := w.db.UserByID(connection.UserID)
	if err != nil {
		return openai.Request{}, err
	}
	overrides, err := w.db.RecentOverrides(user.ID, core.LLMOverrideHistory)
	if err != nil {
		return openai.Request{}, err
	}
	sender := message.SenderName + " <" + message.SenderAddr + ">"
	return openai.Request{
		Criteria:  user.Criteria,
		Sender:    sender,
		Subject:   message.Subject,
		Body:      message.Body,
		Source:    connection.Kind,
		Overrides: overrides,
	}, nil
}

// ProcessMessage оценивает одно сообщение. Три повтора с задержками 2с / 8с / 30с.
// Отсутствующее сообщение — не ошибка: его могли удалить, пока оно ждало очереди.
func (w *Worker) ProcessMessage(messageID int64) (*postgres.Message, error) {
	message, err := w.db.MessageByID(messageID)
	if errors.Is(err, exceptions.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	request, err := w.BuildRequest(message)
	if err != nil {
		return nil, err
	}

	var result openai.Result
	var lastErr error
	succeeded := false
	for attempt, delay := range w.RetryDelays {
		result, lastErr = w.analyzer.Analyze(request)
		if lastErr == nil {
			succeeded = true
			break
		}
		log.Printf("модель не ответила (попытка %d): %v", attempt+1, lastErr)
		if attempt < len(w.RetryDelays)-1 {
			w.Sleep(delay)
		}
	}

	if !succeeded {
		log.Printf("оценка недоступна для сообщения %d: %v", messageID, lastErr)
		// У сообщения, которое уже оценивалось, прежнюю оценку не стираем:
		// при переоценке потерять готовую карточку хуже, чем показать старую.
		if message.AnalyzedAt == nil {
			message.Level = "NORMAL"
			message.Category = ""
			message.DeadlineText = ""
			message.Summary = ""
			message.NeedsReply = false
			message.NeedsAction = false
		}
		message.Status = "DONE"
		message.AnalysisFailed = true
	} else {
		message.Status = "DONE"
		message.Level = result.Level
		message.Category = result.Category
		message.DeadlineText = result.Deadline
		message.NeedsReply = result.NeedsReply
		message.NeedsAction = result.NeedsAction
		message.Summary = result.Summary
		message.AnalysisFailed = false
	}
	now := postgres.UTCNow()
	message.AnalyzedAt = &now
	if err := w.db.SaveMessage(message); err != nil {
		return nil, err
	}

	connection, err := w.db.ConnectionByID(message.ConnectionID)
	if err != nil {
		return nil, err
	}
	w.bus.Publish(connection.UserID, "message.analyzed", schemas.MessageOut(message))
	return message, nil
}

// QueueReanalysis — смена критериев: все сообщения пользователя уходят
// на переоценку. Ручные исправления (level_override) при этом не трогаем —
// решение №15.
func QueueReanalysis(db *postgres.DB, enqueuer Enqueuer, userID int64) (int, error) {
	ids, err := db.MarkAllProcessing(userID)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		enqueuer.Enqueue(id)
	}
	return len(ids), nil
}
