package routers

import (
	"encoding/json"
	"errors"
	"net/http"
	"personalinbox/internal/exceptions"
	"strings"

	"personalinbox/internal/postgres"
	"personalinbox/internal/schemas"
	"personalinbox/internal/telegram"
)

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
	byKind := map[string]*postgres.Connection{}
	for _, connection := range existing {
		byKind[connection.Kind] = connection
	}

	result := make([]schemas.Connection, 0, len(postgres.SourceKinds))
	for _, kind := range postgres.SourceKinds {
		if connection, ok := byKind[kind]; ok {
			result = append(result, schemas.ConnectionOut(connection))
			continue
		}
		result = append(result, schemas.Connection{Kind: kind, State: "off"})
	}
	respond(w, http.StatusOK, result)
}

func (s *Server) handleGmailStart(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	state, err := s.cache.NewOAuthState(r.Context(), user.ID)
	if err != nil {
		fail(w, http.StatusServiceUnavailable, "Хранилище недоступно")
		return
	}
	authURL, err := s.gmail.AuthURL(state)
	if err != nil {
		fail(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	respond(w, http.StatusOK, map[string]string{"auth_url": authURL})
}

func (s *Server) handleGmailCallback(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	// state одноразовый: TakeOAuthState забирает его и сразу удаляет, поэтому
	// повторно тот же ответ Google не пройдёт.
	expected := s.cache.TakeOAuthState(r.Context(), user.ID)
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" || expected == "" || state != expected {
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
	// Состав источников поменялся — карточки в кэше устарели.
	s.cache.DropCache(r.Context(), user.ID)
	http.Redirect(w, r, s.cfg.FrontendOrigin+"/connections?connected=gmail", http.StatusFound)
}

// Подключение Telegram идёт в два шага: номер телефона → код из Telegram
// (и пароль, если включена двухфакторная защита). Между шагами состояние
// лежит в Redis: в нём уже есть ключ шифрования сессии, поэтому живёт оно
// десять минут и забирается ровно один раз (решение №52).

// handleTelegramStart просит Telegram прислать код на номер.
func (s *Server) handleTelegramStart(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	if !s.telegram.Configured() {
		fail(w, http.StatusServiceUnavailable,
			"Не заданы TELEGRAM_API_ID и TELEGRAM_API_HASH")
		return
	}
	var payload struct {
		Phone string `json:"phone"`
	}
	if !readJSON(w, r, &payload) {
		return
	}
	phone := strings.TrimSpace(payload.Phone)
	if len(phone) < 5 {
		fail(w, http.StatusUnprocessableEntity, "Введите номер телефона")
		return
	}

	pending, err := s.telegram.SendCode(r.Context(), phone)
	if err != nil {
		fail(w, http.StatusBadGateway, err.Error())
		return
	}
	if err := s.cache.SavePending(r.Context(), user.ID, pending); err != nil {
		fail(w, http.StatusServiceUnavailable, "Хранилище недоступно")
		return
	}
	respond(w, http.StatusOK, map[string]string{"phone": phone})
}

// handleTelegramConfirm завершает вход и сохраняет сессию.
func (s *Server) handleTelegramConfirm(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	var payload struct {
		Code     string `json:"code"`
		Password string `json:"password"`
	}
	if !readJSON(w, r, &payload) {
		return
	}
	if strings.TrimSpace(payload.Code) == "" {
		fail(w, http.StatusUnprocessableEntity, "Введите код из Telegram")
		return
	}

	var pending telegram.Pending
	if !s.cache.TakePending(r.Context(), user.ID, &pending) {
		fail(w, http.StatusBadRequest, "Время ожидания кода истекло — начните заново")
		return
	}

	sessionData, account, err := s.telegram.SignIn(r.Context(), pending, payload.Code, payload.Password)
	if err != nil {
		// Нужен пароль — состояние возвращаем: пользователь введёт его
		// на том же шаге, повторно код запрашивать незачем.
		if errors.Is(err, telegram.ErrPasswordNeeded) {
			if err := s.cache.SavePending(r.Context(), user.ID, pending); err != nil {
				fail(w, http.StatusServiceUnavailable, "Хранилище недоступно")
				return
			}
			// Не ошибка, а второй шаг того же входа: фронт покажет поле пароля.
			respond(w, http.StatusOK, map[string]bool{"password_needed": true})
			return
		}
		fail(w, http.StatusBadRequest, err.Error())
		return
	}

	connection, err := s.db.GetOrCreateConnection(user.ID, "telegram")
	if err != nil {
		fail(w, http.StatusInternalServerError, "Не удалось сохранить подключение")
		return
	}
	credentials, _ := json.Marshal(telegram.Credentials{Session: sessionData})
	connection.Account = account
	connection.Credentials = string(credentials)
	connection.State = "active"
	// Метка синхронизации ставится первым проходом планировщика: до него
	// сообщений ещё нет, и «синхронизировано только что» было бы неправдой.
	connection.LastSyncAt = nil
	if err := s.db.SaveConnection(connection); err != nil {
		fail(w, http.StatusInternalServerError, "Не удалось сохранить подключение")
		return
	}
	s.cache.DropCache(r.Context(), user.ID)
	respond(w, http.StatusOK, schemas.ConnectionOut(connection))
}

// handleDisconnect обнуляет доступы, но не удаляет сообщения — они остаются
// в ленте.
func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	kind := r.PathValue("kind")
	if !postgres.Contains(postgres.SourceKinds, kind) {
		fail(w, http.StatusNotFound, "Неизвестный источник")
		return
	}
	connection, err := s.db.Connection(user.ID, kind)
	if errors.Is(err, exceptions.ErrNotFound) {
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
	s.cache.DropCache(r.Context(), user.ID)
	w.WriteHeader(http.StatusNoContent)
}
