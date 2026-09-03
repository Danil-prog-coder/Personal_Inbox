// Package events — шина событий для SSE. Один процесс, один пользователь:
// хватает кольцевого буфера в памяти и опроса из обработчика потока
// (docs/03-data-model.md, п. 6.1).
package events

import "sync"

// Event — то, что уезжает в поток: message.created или message.analyzed.
type Event struct {
	Seq    int64
	UserID int64
	Name   string
	Data   any
}

// Bus — кольцевой буфер событий с курсором.
type Bus struct {
	mu     sync.Mutex
	events []Event
	maxLen int
	seq    int64
	hook   func(userID int64)
}

// New создаёт шину на maxLen последних событий.
func New(maxLen int) *Bus {
	if maxLen <= 0 {
		maxLen = 200
	}
	return &Bus{maxLen: maxLen}
}

// OnPublish задаёт обработчик, который вызывается на каждое событие. Через
// шину проходит любое изменение ленты — приём сообщения, оценка модели, ручное
// исправление уровня, — поэтому сброс кэша удобно повесить одной точкой сюда,
// а не разыскивать все места записи.
func (b *Bus) OnPublish(hook func(userID int64)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.hook = hook
}

// Publish кладёт событие в буфер и возвращает его номер.
func (b *Bus) Publish(userID int64, name string, data any) Event {
	b.mu.Lock()
	b.seq++
	event := Event{Seq: b.seq, UserID: userID, Name: name, Data: data}
	b.events = append(b.events, event)
	if len(b.events) > b.maxLen {
		b.events = append([]Event(nil), b.events[len(b.events)-b.maxLen:]...)
	}
	hook := b.hook
	b.mu.Unlock()

	// Хук ходит в Redis, поэтому вызывается уже без блокировки: иначе медленный
	// Redis останавливал бы всю шину.
	if hook != nil {
		hook(userID)
	}
	return event
}

// Cursor — текущая позиция: подписчик начинает с неё и не получает старое.
func (b *Bus) Cursor() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.seq
}

// Since — события пользователя после курсора и новая позиция курсора.
func (b *Bus) Since(userID int64, cursor int64) ([]Event, int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var result []Event
	for _, event := range b.events {
		if event.Seq > cursor && event.UserID == userID {
			result = append(result, event)
		}
	}
	return result, b.seq
}

// Clear нужен тестам: между случаями буфер не должен протекать.
func (b *Bus) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = nil
	b.seq = 0
}
