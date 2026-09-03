package routers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Поток опрашивает шину раз в секунду и раз в 15 секунд шлёт комментарий-пинг,
// чтобы прокси не закрыл соединение как простаивающее.
const (
	pollInterval = time.Second
	pingInterval = 15 * time.Second
)

// handleStream — Server-Sent Events: новые сообщения и результаты оценки
// приезжают на фронт. WebSocket не нужен, поток односторонний (решение №18).
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		fail(w, http.StatusInternalServerError, "Поток недоступен")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	// Первый байт сразу — иначе браузер держит соединение «висящим».
	fmt.Fprint(w, ": ok\n\n")
	flusher.Flush()

	// Старые события новому подписчику не отдаём.
	cursor := s.bus.Cursor()
	sincePing := time.Duration(0)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			var batch []string
			events, next := s.bus.Since(user.ID, cursor)
			cursor = next
			for _, event := range events {
				payload, err := json.Marshal(event.Data)
				if err != nil {
					continue
				}
				batch = append(batch, fmt.Sprintf("event: %s\ndata: %s\n\n", event.Name, payload))
			}
			if len(batch) > 0 {
				sincePing = 0
				for _, chunk := range batch {
					fmt.Fprint(w, chunk)
				}
				flusher.Flush()
				continue
			}
			sincePing += pollInterval
			if sincePing >= pingInterval {
				sincePing = 0
				fmt.Fprint(w, ": ping\n\n")
				flusher.Flush()
			}
		}
	}
}
