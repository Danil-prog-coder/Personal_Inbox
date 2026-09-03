package api

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"personalinbox/internal/store"
	"personalinbox/internal/view"
)

// randomState — одноразовое значение state для OAuth: защита от подмены ответа.
func randomState() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "personal-inbox-state"
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// handleListConnections: оба источника всегда в списке, неподключённый
// показывается как off.
func (s *Server) handleListConnections(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	existing, err := s.db.ConnectionsOf(user.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Не удалось получить источники")
		return
	}
	byKind := map[string]*store.Connection{}
	for _, connection := range existing {
		byKind[connection.Kind] = connection
	}

	result := make([]view.Connection, 0, len(store.SourceKinds))
	for _, kind := range store.SourceKinds {
		if connection, ok := byKind[kind]; ok {
			result = append(result, view.ConnectionOut(connection))
			continue
		}
		result = append(result, view.Connection{Kind: kind, State: "off"})
	}
	respond(w, http.StatusOK, result)
}

func (s *Server) handleGmailStart(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	state := randomState()
	authURL, err := s.gmail.AuthURL(state)
	if err != nil {
		fail(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	s.setSession(w, session{UserID: user.ID, OAuthState: state})
	respond(w, http.StatusOK, map[string]string{"auth_url": authURL})
}

func (s *Server) handleGmailCallback(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	expected := s.decodeSession(r).OAuthState
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	// state одноразовый: после проверки его в сессии больше нет.
	s.setSession(w, session{UserID: user.ID})
	if code == "" || state == "" || state != expected {
		fail(w, http.StatusBadRequest, "Авторизация Google не подтверждена")
		return
	}

	email, refreshToken, err := s.gmail.ExchangeCode(code)
	if err != nil {
		fail(w, http.StatusBadGateway, err.Error())
		return
	}
	connection, err := s.db.GetOrCreateConnection(user.ID, "gmail")
	if err != nil {
		fail(w, http.StatusInternalServerError, "Не удалось сохранить подключение")
		return
	}
	credentials, _ := json.Marshal(map[string]string{"refresh_token": refreshToken})
	connection.Account = email
	connection.Credentials = string(credentials)
	connection.State = "active"
	// Новое подключение — импорт за 30 дней.
	connection.SyncCursor = ""
	connection.LastSyncAt = nil
	if err := s.db.SaveConnection(connection); err != nil {
		fail(w, http.StatusInternalServerError, "Не удалось сохранить подключение")
		return
	}
	http.Redirect(w, r, s.cfg.FrontendOrigin+"/connections?connected=gmail", http.StatusFound)
}

// handleConnectTelegram: токен проверяется через getMe — пользователю
// показываем имя бота.
func (s *Server) handleConnectTelegram(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	var payload struct {
		BotToken string `json:"bot_token"`
	}
	if !readJSON(w, r, &payload) {
		return
	}
	token := strings.TrimSpace(payload.BotToken)
	if len(token) < 10 {
		fail(w, http.StatusUnprocessableEntity, "Токен бота слишком короткий")
		return
	}

	botName, err := s.telegram.VerifyToken(token)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}

	connection, err := s.db.GetOrCreateConnection(user.ID, "telegram")
	if err != nil {
		fail(w, http.StatusInternalServerError, "Не удалось сохранить подключение")
		return
	}
	credentials, _ := json.Marshal(map[string]string{"bot_token": token})
	now := store.UTCNow()
	connection.Account = botName
	connection.Credentials = string(credentials)
	connection.State = "active"
	connection.LastSyncAt = &now
	if err := s.db.SaveConnection(connection); err != nil {
		fail(w, http.StatusInternalServerError, "Не удалось сохранить подключение")
		return
	}
	respond(w, http.StatusOK, view.ConnectionOut(connection))
}

// handleDisconnect обнуляет доступы, но не удаляет сообщения — они остаются
// в ленте.
func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	kind := r.PathValue("kind")
	if !store.Contains(store.SourceKinds, kind) {
		fail(w, http.StatusNotFound, "Неизвестный источник")
		return
	}
	connection, err := s.db.Connection(user.ID, kind)
	if errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "Источник не подключён")
		return
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, "Не удалось отключить источник")
		return
	}
	connection.State = "off"
	connection.Credentials = ""
	if err := s.db.SaveConnection(connection); err != nil {
		fail(w, http.StatusInternalServerError, "Не удалось отключить источник")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
