package postgres

import (
	"strings"
	"time"
)

// MaxTZOffsetMinutes — смещение часового пояса приходит с фронта в минутах
// (как -getTimezoneOffset()). Дальше реальных часовых поясов не бывает.
const MaxTZOffsetMinutes = 14 * 60

// Filter — набор фильтров ленты уровня 2 (docs/03-data-model.md, п. 6).
type Filter struct {
	Source   string
	Level    string
	Status   string
	Reply    string
	Action   string
	Period   string
	Q        string
	TZOffset int
	Now      time.Time
}

// PeriodStart — начало окна фильтра «Период». nil — «Всё время».
func PeriodStart(period string, tzOffset int, now time.Time) *time.Time {
	if now.IsZero() {
		now = UTCNow()
	}
	switch period {
	case "today":
		minutes := tzOffset
		if minutes > MaxTZOffsetMinutes {
			minutes = MaxTZOffsetMinutes
		}
		if minutes < -MaxTZOffsetMinutes {
			minutes = -MaxTZOffsetMinutes
		}
		offset := time.Duration(minutes) * time.Minute
		local := now.Add(offset)
		midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
		start := midnight.Add(-offset)
		return &start
	case "week":
		start := now.Add(-7 * 24 * time.Hour)
		return &start
	case "month":
		start := now.Add(-30 * 24 * time.Hour)
		return &start
	}
	return nil
}

// SummaryPeriodStart — окно сводки: 24ч / Неделя / Месяц.
func SummaryPeriodStart(period string, now time.Time) time.Time {
	if now.IsZero() {
		now = UTCNow()
	}
	hours := 24
	switch period {
	case "week":
		hours = 24 * 7
	case "month":
		hours = 24 * 30
	}
	return now.Add(-time.Duration(hours) * time.Hour)
}

// escapeLike — % и _ внутри поискового запроса это обычные символы, а не шаблон.
func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "%", `\%`)
	return strings.ReplaceAll(value, "_", `\_`)
}

// FilteredMessages — лента уровня 2: сообщения пользователя под фильтрами,
// новые первыми.
func (db *DB) FilteredMessages(userID int64, filter Filter) ([]*Message, error) {
	where := []string{"c.user_id = ?"}
	args := []any{userID}

	if filter.Source != "" {
		where = append(where, "c.kind = ?")
		args = append(args, filter.Source)
	}
	if filter.Level != "" && filter.Level != "all" {
		where = append(where, effectiveLevelSQL+" = ?")
		args = append(args, filter.Level)
	}
	switch filter.Status {
	case "unread":
		where = append(where, "m.is_read = false")
	case "read":
		where = append(where, "m.is_read = true")
	case "done":
		where = append(where, "m.status = 'DONE'")
	}
	if filter.Reply == "yes" || filter.Reply == "no" {
		where = append(where, "m.needs_reply = ?")
		args = append(args, filter.Reply == "yes")
	}
	if filter.Action == "yes" || filter.Action == "no" {
		where = append(where, "m.needs_action = ?")
		args = append(args, filter.Action == "yes")
	}
	if start := PeriodStart(filter.Period, filter.TZOffset, filter.Now); start != nil {
		where = append(where, "m.received_at >= ?")
		args = append(args, start.UTC())
	}
	if query := strings.TrimSpace(filter.Q); query != "" {
		// Поиск по отправителю, теме и тексту, регистронезависимо.
		pattern := "%" + escapeLike(strings.ToLower(query)) + "%"
		where = append(where, `(lower(m.sender_name) LIKE ? ESCAPE '\'
			OR lower(m.sender_addr) LIKE ? ESCAPE '\'
			OR lower(m.subject) LIKE ? ESCAPE '\'
			OR lower(m.body) LIKE ? ESCAPE '\')`)
		args = append(args, pattern, pattern, pattern, pattern)
	}

	return db.queryMessages(
		`SELECT `+messageColumns+` FROM message m JOIN connection c ON c.id = m.connection_id
		 WHERE `+strings.Join(where, " AND ")+`
		 ORDER BY m.received_at DESC, m.id DESC`, args...)
}

// MessagesSince — сообщения пользователя за окно сводки, новые первыми.
func (db *DB) MessagesSince(userID int64, since time.Time) ([]*Message, error) {
	return db.queryMessages(
		`SELECT `+messageColumns+` FROM message m JOIN connection c ON c.id = m.connection_id
		 WHERE c.user_id = ? AND m.received_at >= ?
		 ORDER BY m.received_at DESC, m.id DESC`, userID, since.UTC())
}

// Distribution — счётчики по четырём уровням для полосы распределения.
// Нули включены: полоса всегда состоит из четырёх сегментов.
func Distribution(messages []*Message) map[string]int {
	counts := make(map[string]int, len(Levels))
	for _, level := range Levels {
		counts[level] = 0
	}
	for _, message := range messages {
		counts[message.EffectiveLevel()]++
	}
	return counts
}
