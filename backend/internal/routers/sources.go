package routers

import (
	"net/http"

	"personalinbox/internal/schemas"
	"personalinbox/internal/sqlite"
)

// handleListSources — лента уровня 1. Отключённые источники не показываются
// вообще (решение №22).
func (s *Server) handleListSources(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	connections, err := s.db.VisibleConnectionsOf(user.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Не удалось получить источники")
		return
	}

	cards := make([]schemas.SourceCard, 0, len(connections))
	for _, connection := range connections {
		messages, err := s.db.MessagesOfConnection(connection.ID)
		if err != nil {
			fail(w, http.StatusInternalServerError, "Не удалось получить сообщения")
			return
		}
		unread := 0
		for _, message := range messages {
			if !message.IsRead {
				unread++
			}
		}
		cards = append(cards, schemas.SourceCard{
			Kind:         connection.Kind,
			State:        connection.State,
			Account:      connection.Account,
			LastSyncAt:   schemas.TimePtr(connection.LastSyncAt),
			Total:        len(messages),
			Unread:       unread,
			Distribution: sqlite.Distribution(messages),
			Urgent:       urgent(messages),
		})
	}
	respond(w, http.StatusOK, cards)
}

// urgent — самое срочное сообщение источника: сначала CRITICAL, потом HIGH.
func urgent(messages []*sqlite.Message) *schemas.MessageBrief {
	for _, level := range []string{"CRITICAL", "HIGH"} {
		for _, message := range messages {
			if message.EffectiveLevel() == level {
				brief := schemas.MessageBriefOut(message)
				return &brief
			}
		}
	}
	return nil
}
