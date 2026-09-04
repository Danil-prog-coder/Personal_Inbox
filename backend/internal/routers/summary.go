package routers

import (
	"net/http"
	"time"

	"personalinbox/internal/postgres"
	"personalinbox/internal/schemas"
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

	// Фронт дёргает сводку на каждом заходе во вкладку, а считается она по всем
	// сообщениям за период. Кэш живёт 45 секунд и сбрасывается любой записью.
	cacheName := "summary:" + period
	var cached schemas.Summary
	if s.cache.GetCached(r.Context(), user.ID, cacheName, &cached) {
		respond(w, http.StatusOK, cached)
		return
	}

	messages, err := s.db.MessagesSince(user.ID, postgres.SummaryPeriodStart(period, time.Time{}))
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
	top := make([]schemas.MessageBrief, 0, topLimit)
	for _, level := range []string{"CRITICAL", "HIGH"} {
		for _, message := range messages {
			if len(top) >= topLimit {
				break
			}
			if message.EffectiveLevel() == level {
				top = append(top, schemas.MessageBriefOut(message))
			}
		}
	}

	summary := schemas.Summary{
		Period:       period,
		Total:        len(messages),
		Distribution: postgres.Distribution(messages),
		NeedsReply:   needsReply,
		NeedsAction:  needsAction,
		Top:          top,
	}
	s.cache.SetCached(r.Context(), user.ID, cacheName, summary)
	respond(w, http.StatusOK, summary)
}
