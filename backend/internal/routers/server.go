// Package routers — HTTP-слой: маршруты, разбор запросов, ответы.
// Контракт зафиксирован в docs/03-data-model.md, п. 6.
package routers

import (
	"encoding/json"
	"io"
	"net/http"

	"personalinbox/internal/core"
	"personalinbox/internal/events"
	"personalinbox/internal/gmail"
	"personalinbox/internal/postgres"
	"personalinbox/internal/redis"
	"personalinbox/internal/services/analysis"
	"personalinbox/internal/services/ingest"
	"personalinbox/internal/telegram"
)

// Server — всё, что нужно ручкам.
type Server struct {
	cfg      core.Config
	db       *postgres.DB
	cache    *redis.Client
	bus      *events.Bus
	ingestor *ingest.Ingestor
	enqueuer analysis.Enqueuer
	gmail    *gmail.Client
	telegram *telegram.Client
}

// New собирает сервер.
func New(
	cfg core.Config,
	db *postgres.DB,
	cache *redis.Client,
	bus *events.Bus,
	ingestor *ingest.Ingestor,
	enqueuer analysis.Enqueuer,
	gmailClient *gmail.Client,
	telegramClient *telegram.Client,
) *Server {
	return &Server{
		cfg:      cfg,
		db:       db,
		cache:    cache,
		bus:      bus,
		ingestor: ingestor,
		enqueuer: enqueuer,
		gmail:    gmailClient,
		telegram: telegramClient,
	}
}

// Handler — маршруты приложения вместе с CORS.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/auth/register", s.handleRegister)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)

	mux.HandleFunc("GET /api/me", s.handleGetMe)
	mux.HandleFunc("PATCH /api/me", s.handleUpdateMe)

	mux.HandleFunc("GET /api/connections", s.handleListConnections)
	mux.HandleFunc("POST /api/connections/gmail/start", s.handleGmailStart)
	mux.HandleFunc("GET /api/connections/gmail/callback", s.handleGmailCallback)
	mux.HandleFunc("POST /api/connections/telegram", s.handleConnectTelegram)
	mux.HandleFunc("DELETE /api/connections/{kind}", s.handleDisconnect)

	mux.HandleFunc("GET /api/sources", s.handleListSources)

	mux.HandleFunc("GET /api/messages", s.handleListMessages)
	mux.HandleFunc("GET /api/messages/{id}", s.handleGetMessage)
	mux.HandleFunc("POST /api/messages/{id}/read", s.handleMarkRead)
	mux.HandleFunc("POST /api/messages/{id}/level", s.handleSetLevel)

	mux.HandleFunc("GET /api/summary", s.handleSummary)
	mux.HandleFunc("GET /api/stream", s.handleStream)

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, _ *http.Request) {
		respond(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	return s.withCORS(mux)
}

// withCORS — фронт живёт на другом порту в разработке, поэтому куки ходят
// с credentials и origin указывается явно, а не звёздочкой.
func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && origin == s.cfg.FrontendOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// respond пишет JSON-ответ.
func respond(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(payload)
}

// fail — ошибка в том же формате, что читает фронт: {"detail": "..."}.
func fail(w http.ResponseWriter, status int, detail string) {
	respond(w, status, map[string]string{"detail": detail})
}

// readJSON разбирает тело запроса. Кривой JSON — 422, как у прежнего бэкенда.
func readJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		fail(w, http.StatusUnprocessableEntity, "Тело запроса не прочитать")
		return false
	}
	if len(body) == 0 {
		body = []byte("{}")
	}
	if err := json.Unmarshal(body, target); err != nil {
		fail(w, http.StatusUnprocessableEntity, "Тело запроса не разобрать")
		return false
	}
	return true
}
