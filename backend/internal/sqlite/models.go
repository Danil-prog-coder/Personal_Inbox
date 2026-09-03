// Package store — модель данных и доступ к SQLite. Имена полей и значения
// перечислений — из docs/03-data-model.md, менять их без правки документа нельзя.
package sqlite

import "time"

// Перечисления хранятся строками — не числами и не индексами.
var (
	Levels      = []string{"CRITICAL", "HIGH", "NORMAL", "LOW"}
	SourceKinds = []string{"gmail", "telegram"}
	ConnStates  = []string{"off", "active", "reauth"}
	MsgStatuses = []string{"PROCESSING", "DONE"}
	Themes      = []string{"dark", "light"}
	Densities   = []string{"spacious", "compact"}
)

// Contains — проверка значения перечисления. Ошибка «неизвестный уровень»
// должна быть 422, а не запись мусора в базу.
func Contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

// UTCNow — единая точка получения времени: в базе всё лежит в UTC без зоны.
func UTCNow() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

// User — пользователь один на инсталляцию, но таблица всё равно нужна:
// на ней висит аутентификация.
type User struct {
	ID           int64
	Email        string
	PasswordHash string
	Criteria     string
	Theme        string
	Density      string
	CreatedAt    time.Time
}

// Connection — одна строка на (пользователь, сервис). Отключение источника
// ставит state = off и обнуляет credentials, но не удаляет сообщения.
type Connection struct {
	ID     int64
	UserID int64
	Kind   string
	State  string
	// JSON: refresh_token для Gmail, bot_token для Telegram.
	Account     string
	Credentials string
	LastSyncAt  *time.Time
	SyncCursor  string
}

// Message — сообщение из источника вместе с оценкой модели.
type Message struct {
	ID            int64
	ConnectionID  int64
	ExternalID    string
	SenderName    string
	SenderAddr    string
	Subject       string
	Body          string
	ReceivedAt    time.Time
	IsRead        bool
	Status        string
	Level         string // пустая строка — модель ещё не ответила
	LevelOverride string // ручное исправление пользователя
	Category      string
	DeadlineText  string
	NeedsReply    bool
	NeedsAction   bool
	Summary       string
	ExternalURL   string
	AnalyzedAt    *time.Time
	// Модель не ответила после трёх попыток — на карточке пометка «Оценка недоступна».
	AnalysisFailed bool

	// Kind — вид источника сообщения. Заполняется выборками, чтобы не ходить
	// за подключением отдельным запросом.
	Kind string
}

// EffectiveLevel — уровень, который видит пользователь: ручное исправление
// важнее оценки модели. Отдельной колонки под него нет.
func (m *Message) EffectiveLevel() string {
	if m.LevelOverride != "" {
		return m.LevelOverride
	}
	if m.Level != "" {
		return m.Level
	}
	return "NORMAL"
}

// OverrideLog — обратная связь для модели: в промпт уходят последние 20 записей.
// Пользователю он не показывается.
type OverrideLog struct {
	ID        int64
	MessageID int64
	FromLevel string
	ToLevel   string
	CreatedAt time.Time
}
