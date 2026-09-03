// Package view — перевод моделей в схемы ответа. Один формат сообщения
// на все ручки и на SSE (docs/03-data-model.md, п. 6).
package schemas

import (
	"strings"
	"time"

	"personalinbox/internal/postgres"
)

// Time — время в ответе API: ISO без зоны, как отдавал прежний бэкенд.
// Фронт дописывает «Z» сам (frontend/src/lib/format.ts).
type Time time.Time

const jsonLayout = "2006-01-02T15:04:05"

// MarshalJSON пишет момент в UTC без суффикса зоны.
func (t Time) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(t).UTC().Format(jsonLayout) + `"`), nil
}

// UnmarshalJSON нужен тестам, которые разбирают собственные ответы.
func (t *Time) UnmarshalJSON(raw []byte) error {
	value := strings.Trim(string(raw), `"`)
	parsed, err := time.Parse(jsonLayout, value)
	if err != nil {
		return err
	}
	*t = Time(parsed)
	return nil
}

// TimePtr — nullable-время: null, если метки нет.
func TimePtr(value *time.Time) *Time {
	if value == nil {
		return nil
	}
	converted := Time(*value)
	return &converted
}

// User — профиль: критерии, тема, плотность.
type User struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Criteria  string `json:"criteria"`
	Theme     string `json:"theme"`
	Density   string `json:"density"`
	CreatedAt Time   `json:"created_at"`
}

// Connection — строка экрана «Источники».
type Connection struct {
	Kind       string `json:"kind"`
	State      string `json:"state"`
	Account    string `json:"account"`
	LastSyncAt *Time  `json:"last_sync_at"`
}

// MessageBrief — строка «Отправитель — Тема» с точкой уровня:
// карточка источника и сводка.
type MessageBrief struct {
	ID         int64  `json:"id"`
	SenderName string `json:"sender_name"`
	Subject    string `json:"subject"`
	Level      string `json:"level"`
}

// SourceCard — карточка уровня 1.
type SourceCard struct {
	Kind         string         `json:"kind"`
	State        string         `json:"state"`
	Account      string         `json:"account"`
	LastSyncAt   *Time          `json:"last_sync_at"`
	Total        int            `json:"total"`
	Unread       int            `json:"unread"`
	Distribution map[string]int `json:"distribution"`
	Urgent       *MessageBrief  `json:"urgent"`
}

// Message — сообщение целиком: и в списке, и в деталях, и в событии SSE.
type Message struct {
	ID             int64   `json:"id"`
	Source         string  `json:"source"`
	ExternalID     string  `json:"external_id"`
	SenderName     string  `json:"sender_name"`
	SenderAddr     string  `json:"sender_addr"`
	Subject        string  `json:"subject"`
	Body           string  `json:"body"`
	ReceivedAt     Time    `json:"received_at"`
	IsRead         bool    `json:"is_read"`
	Status         string  `json:"status"`
	Level          string  `json:"level"`
	LevelOverride  *string `json:"level_override"`
	Category       string  `json:"category"`
	DeadlineText   string  `json:"deadline_text"`
	NeedsReply     bool    `json:"needs_reply"`
	NeedsAction    bool    `json:"needs_action"`
	Summary        string  `json:"summary"`
	ExternalURL    string  `json:"external_url"`
	AnalyzedAt     *Time   `json:"analyzed_at"`
	AnalysisFailed bool    `json:"analysis_failed"`
}

// MessageList — ответ ленты: счётчики нужны подзаголовку
// «N сообщений · M непрочитанных» и пересчитываются по текущим фильтрам.
type MessageList struct {
	Items  []Message `json:"items"`
	Total  int       `json:"total"`
	Unread int       `json:"unread"`
}

// Summary — сводка за период.
type Summary struct {
	Period       string         `json:"period"`
	Total        int            `json:"total"`
	Distribution map[string]int `json:"distribution"`
	NeedsReply   int            `json:"needs_reply"`
	NeedsAction  int            `json:"needs_action"`
	Top          []MessageBrief `json:"top"`
}

// MeUpdateResult — второе поле говорит фронту, сколько сообщений ушло
// на переоценку после смены критериев.
type MeUpdateResult struct {
	User            User `json:"user"`
	ReanalyzeQueued int  `json:"reanalyze_queued"`
}

// UserOut собирает профиль для ответа.
func UserOut(user *postgres.User) User {
	return User{
		ID:        user.ID,
		Email:     user.Email,
		Criteria:  user.Criteria,
		Theme:     user.Theme,
		Density:   user.Density,
		CreatedAt: Time(user.CreatedAt),
	}
}

// ConnectionOut собирает строку источника.
func ConnectionOut(connection *postgres.Connection) Connection {
	return Connection{
		Kind:       connection.Kind,
		State:      connection.State,
		Account:    connection.Account,
		LastSyncAt: TimePtr(connection.LastSyncAt),
	}
}

// MessageOut собирает сообщение. Уровень наружу отдаётся эффективный:
// ручное исправление важнее оценки модели.
func MessageOut(message *postgres.Message) Message {
	var override *string
	if message.LevelOverride != "" {
		value := message.LevelOverride
		override = &value
	}
	return Message{
		ID:             message.ID,
		Source:         message.Kind,
		ExternalID:     message.ExternalID,
		SenderName:     message.SenderName,
		SenderAddr:     message.SenderAddr,
		Subject:        message.Subject,
		Body:           message.Body,
		ReceivedAt:     Time(message.ReceivedAt),
		IsRead:         message.IsRead,
		Status:         message.Status,
		Level:          message.EffectiveLevel(),
		LevelOverride:  override,
		Category:       message.Category,
		DeadlineText:   message.DeadlineText,
		NeedsReply:     message.NeedsReply,
		NeedsAction:    message.NeedsAction,
		Summary:        message.Summary,
		ExternalURL:    message.ExternalURL,
		AnalyzedAt:     TimePtr(message.AnalyzedAt),
		AnalysisFailed: message.AnalysisFailed,
	}
}

// MessageBriefOut собирает короткую строку сообщения.
func MessageBriefOut(message *postgres.Message) MessageBrief {
	return MessageBrief{
		ID:         message.ID,
		SenderName: message.SenderName,
		Subject:    message.Subject,
		Level:      message.EffectiveLevel(),
	}
}
