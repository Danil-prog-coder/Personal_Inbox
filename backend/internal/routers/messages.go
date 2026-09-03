package routers

import (
	"errors"
	"net/http"
	"personalinbox/internal/exceptions"
	"strconv"

	"personalinbox/internal/postgres"
	"personalinbox/internal/schemas"
)

// param читает значение из строки запроса и проверяет его по списку
// допустимых. Неизвестное значение — 422, как у прежнего бэкенда.
func param(w http.ResponseWriter, r *http.Request, name, fallback string, allowed []string) (string, bool) {
	value := r.URL.Query().Get(name)
	if value == "" {
		return fallback, true
	}
	if !postgres.Contains(allowed, value) {
		fail(w, http.StatusUnprocessableEntity, "Недопустимое значение фильтра «"+name+"»")
		return "", false
	}
	return value, true
}

// ownedMessage — сообщение пользователя. Чужое сообщение — 404, а не 403:
// о его существовании знать незачем.
func (s *Server) ownedMessage(w http.ResponseWriter, r *http.Request, user *postgres.User) (*postgres.Message, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, http.StatusNotFound, "Сообщение не найдено")
		return nil, false
	}
	message, err := s.db.MessageByID(id)
	if errors.Is(err, exceptions.ErrNotFound) {
		fail(w, http.StatusNotFound, "Сообщение не найдено")
		return nil, false
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "Не удалось получить сообщение")
		return nil, false
	}
	connection, err := s.db.ConnectionByID(message.ConnectionID)
	if err != nil || connection.UserID != user.ID {
		fail(w, http.StatusNotFound, "Сообщение не найдено")
		return nil, false
	}
	return message, true
}

func (s *Server) handleListMessages(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}

	source, ok := param(w, r, "source", "", postgres.SourceKinds)
	if !ok {
		return
	}
	level, ok := param(w, r, "level", "all",
		[]string{"all", "CRITICAL", "HIGH", "NORMAL", "LOW"})
	if !ok {
		return
	}
	status, ok := param(w, r, "status", "all", []string{"all", "unread", "read", "done"})
	if !ok {
		return
	}
	reply, ok := param(w, r, "reply", "all", []string{"all", "yes", "no"})
	if !ok {
		return
	}
	action, ok := param(w, r, "action", "all", []string{"all", "yes", "no"})
	if !ok {
		return
	}
	period, ok := param(w, r, "period", "all", []string{"all", "today", "week", "month"})
	if !ok {
		return
	}

	tzOffset := 0
	if raw := r.URL.Query().Get("tz_offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			fail(w, http.StatusUnprocessableEntity, "tz_offset должен быть числом")
			return
		}
		tzOffset = parsed
	}

	messages, err := s.db.FilteredMessages(user.ID, postgres.Filter{
		Source:   source,
		Level:    level,
		Status:   status,
		Reply:    reply,
		Action:   action,
		Period:   period,
		Q:        r.URL.Query().Get("q"),
		TZOffset: tzOffset,
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, "Не удалось получить сообщения")
		return
	}

	items := make([]schemas.Message, 0, len(messages))
	unread := 0
	for _, message := range messages {
		items = append(items, schemas.MessageOut(message))
		if !message.IsRead {
			unread++
		}
	}
	respond(w, http.StatusOK, schemas.MessageList{
		Items:  items,
		Total:  len(items),
		Unread: unread,
	})
}

func (s *Server) handleGetMessage(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	message, ok := s.ownedMessage(w, r, user)
	if !ok {
		return
	}
	respond(w, http.StatusOK, schemas.MessageOut(message))
}

// handleMarkRead: помечается при открытии деталей. Обратной операции
// в MVP нет (решение №12).
func (s *Server) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	message, ok := s.ownedMessage(w, r, user)
	if !ok {
		return
	}
	if !message.IsRead {
		message.IsRead = true
		if err := s.db.SaveMessage(message); err != nil {
			fail(w, http.StatusInternalServerError, "Не удалось сохранить сообщение")
			return
		}
		// Событие в шину не уходит (карточка не перерисовывается), а счётчик
		// непрочитанных в кэше источников уже неверный — сбрасываем руками.
		s.cache.DropCache(r.Context(), user.ID)
	}
	respond(w, http.StatusOK, schemas.MessageOut(message))
}

// handleSetLevel — ручное исправление уровня, единственная точка правки оценки.
func (s *Server) handleSetLevel(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	message, ok := s.ownedMessage(w, r, user)
	if !ok {
		return
	}
	var payload struct {
		Level string `json:"level"`
	}
	if !readJSON(w, r, &payload) {
		return
	}
	if !postgres.Contains(postgres.Levels, payload.Level) {
		fail(w, http.StatusUnprocessableEntity, "Неизвестный уровень важности")
		return
	}

	previous := message.EffectiveLevel()
	if payload.Level != previous {
		if err := s.db.AddOverrideLog(message.ID, previous, payload.Level); err != nil {
			fail(w, http.StatusInternalServerError, "Не удалось сохранить исправление")
			return
		}
	}
	message.LevelOverride = payload.Level
	if err := s.db.SaveMessage(message); err != nil {
		fail(w, http.StatusInternalServerError, "Не удалось сохранить сообщение")
		return
	}
	s.bus.Publish(user.ID, "message.analyzed", schemas.MessageOut(message))
	respond(w, http.StatusOK, schemas.MessageOut(message))
}
