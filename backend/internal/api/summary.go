package api

import (
	"net/http"
	"time"

	"personalinbox/internal/store"
	"personalinbox/internal/view"
)

// topLimit — «Главное за период» показывает до 4 сообщений.
const topLimit = 4

// handleSummary — сводка за период по всем источникам сразу.
func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	period, ok := param(w, r, "period", "24h", []string{"24h", "week", "month"})
	if !ok {
		return
	}

	messages, err := s.db.MessagesSince(user.ID, store.SummaryPeriodStart(period, time.Time{}))
	if err != nil {
		fail(w, http.StatusInternalServerError, "Не удалось собрать сводку")
		return
	}

	needsReply, needsAction := 0, 0
	for _, message := range messages {
		if message.NeedsReply {
			needsReply++
		}
		if message.NeedsAction {
			needsAction++
		}
	}

	// Сначала все CRITICAL, потом HIGH — и не больше четырёх.
	top := make([]view.MessageBrief, 0, topLimit)
	for _, level := range []string{"CRITICAL", "HIGH"} {
		for _, message := range messages {
			if len(top) >= topLimit {
				break
			}
			if message.EffectiveLevel() == level {
				top = append(top, view.MessageBriefOut(message))
			}
		}
	}

	respond(w, http.StatusOK, view.Summary{
		Period:       period,
		Total:        len(messages),
		Distribution: store.Distribution(messages),
		NeedsReply:   needsReply,
		NeedsAction:  needsAction,
		Top:          top,
	})
}
