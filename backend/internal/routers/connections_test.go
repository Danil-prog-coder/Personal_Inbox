package routers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"personalinbox/internal/schemas"
	"personalinbox/internal/telegram"
)

// telegramStub подменяет Bot API: подключение проверяется вызовом getMe.
func (e *env) telegramStub(ok bool) {
	e.t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if !ok {
			w.Write([]byte(`{"ok": false, "description": "Unauthorized: неверный токен"}`))
			return
		}
		w.Write([]byte(`{"ok": true, "result": {"username": "maxorlov_bot"}}`))
	}))
	e.t.Cleanup(server.Close)
	*e.telegram = telegram.Client{BaseURL: server.URL, HTTP: server.Client()}
}

func TestListShowsBothSourcesEvenWithoutConnections(t *testing.T) {
	e := newEnv(t)
	e.user()
	status, raw := e.do(http.MethodGet, "/api/connections", nil)
	if status != http.StatusOK {
		t.Fatalf("список источников вернул %d", status)
	}
	var connections []schemas.Connection
	e.decode(raw, &connections)
	if len(connections) != 2 {
		t.Fatalf("источников должно быть два, их %d", len(connections))
	}
	for _, connection := range connections {
		if connection.State != "off" {
			t.Fatalf("неподключённый источник должен быть off: %+v", connection)
		}
	}
}

func TestListShowsStateAndAccount(t *testing.T) {
	e := newEnv(t)
	user := e.user()
	e.connection(user, "gmail", "active")

	_, raw := e.do(http.MethodGet, "/api/connections", nil)
	var connections []schemas.Connection
	e.decode(raw, &connections)
	for _, connection := range connections {
		if connection.Kind != "gmail" {
			continue
		}
		if connection.State != "active" || connection.Account != "me@northline.io" {
			t.Fatalf("состояние источника собрано неверно: %+v", connection)
		}
		if connection.LastSyncAt == nil {
			t.Fatal("время синхронизации должно приходить на фронт")
		}
	}
}

func TestConnectTelegramVerifiesToken(t *testing.T) {
	e := newEnv(t)
	user := e.user()
	e.telegramStub(true)

	status, raw := e.do(http.MethodPost, "/api/connections/telegram",
		map[string]string{"bot_token": "123456:секретный-токен"})
	if status != http.StatusOK {
		t.Fatalf("подключение вернуло %d: %s", status, raw)
	}
	var connection schemas.Connection
	e.decode(raw, &connection)
	if connection.Account != "@maxorlov_bot" || connection.State != "active" {
		t.Fatalf("подключение собрано неверно: %+v", connection)
	}

	stored, err := e.db.Connection(user.ID, "telegram")
	if err != nil {
		t.Fatal(err)
	}
	var credentials map[string]string
	if err := json.Unmarshal([]byte(stored.Credentials), &credentials); err != nil {
		t.Fatal(err)
	}
	if credentials["bot_token"] != "123456:секретный-токен" {
		t.Fatalf("токен сохранён неверно: %v", credentials)
	}
}

func TestConnectTelegramWithBadToken(t *testing.T) {
	e := newEnv(t)
	e.user()
	e.telegramStub(false)

	status, raw := e.do(http.MethodPost, "/api/connections/telegram",
		map[string]string{"bot_token": "123456:плохой-токен"})
	if status != http.StatusBadRequest {
		t.Fatalf("неверный токен вернул %d", status)
	}
	if !strings.Contains(e.detail(raw), "Unauthorized") {
		t.Fatalf("ошибка Telegram должна доходить до пользователя: %q", e.detail(raw))
	}
}

func TestConnectTelegramRejectsShortToken(t *testing.T) {
	e := newEnv(t)
	e.user()
	status, _ := e.do(http.MethodPost, "/api/connections/telegram",
		map[string]string{"bot_token": "123"})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("слишком короткий токен вернул %d", status)
	}
}

func TestReconnectUpdatesExistingRow(t *testing.T) {
	e := newEnv(t)
	user := e.user()
	e.connection(user, "telegram", "reauth")
	e.telegramStub(true)

	if status, _ := e.do(http.MethodPost, "/api/connections/telegram",
		map[string]string{"bot_token": "123456:новый-токен"}); status != http.StatusOK {
		t.Fatal("переподключение не удалось")
	}
	connections, err := e.db.ConnectionsOf(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 {
		t.Fatalf("переподключение создало вторую строку: %d", len(connections))
	}
	if connections[0].State != "active" {
		t.Fatalf("состояние после переподключения: %q", connections[0].State)
	}
}

func TestDisconnectClearsCredentialsButKeepsMessages(t *testing.T) {
	e := newEnv(t)
	user := e.user()
	connection := e.connection(user, "gmail", "active")
	e.message(connection, nil)

	if status, _ := e.do(http.MethodDelete, "/api/connections/gmail", nil); status != http.StatusNoContent {
		t.Fatal("отключение не удалось")
	}
	stored, err := e.db.Connection(user.ID, "gmail")
	if err != nil {
		t.Fatal(err)
	}
	if stored.State != "off" || stored.Credentials != "" {
		t.Fatalf("доступы не обнулены: %+v", stored)
	}
	messages, err := e.db.MessagesOfConnection(connection.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatal("сообщения должны остаться в ленте после отключения")
	}
}

func TestDisconnectUnknownSource(t *testing.T) {
	e := newEnv(t)
	e.user()
	status, _ := e.do(http.MethodDelete, "/api/connections/slack", nil)
	if status != http.StatusNotFound {
		t.Fatalf("неизвестный источник вернул %d", status)
	}
}

func TestDisconnectMissingConnection(t *testing.T) {
	e := newEnv(t)
	e.user()
	status, _ := e.do(http.MethodDelete, "/api/connections/gmail", nil)
	if status != http.StatusNotFound {
		t.Fatalf("неподключённый источник вернул %d", status)
	}
}

func TestGmailStartWithoutGoogleCredentials(t *testing.T) {
	e := newEnv(t)
	e.user()
	status, raw := e.do(http.MethodPost, "/api/connections/gmail/start", nil)
	if status != http.StatusServiceUnavailable {
		t.Fatalf("без ключей Google ожидался 503, получен %d", status)
	}
	if !strings.Contains(e.detail(raw), "GOOGLE_CLIENT_ID") {
		t.Fatalf("причина должна называть недостающие ключи: %q", e.detail(raw))
	}
}

func TestGmailStartReturnsAuthURL(t *testing.T) {
	e := newEnv(t)
	e.user()
	e.gmail.ClientID = "id"
	e.gmail.ClientSecret = "secret"
	e.gmail.RedirectURI = "http://localhost:8000/api/connections/gmail/callback"

	status, raw := e.do(http.MethodPost, "/api/connections/gmail/start", nil)
	if status != http.StatusOK {
		t.Fatalf("запуск OAuth вернул %d: %s", status, raw)
	}
	var payload struct {
		AuthURL string `json:"auth_url"`
	}
	e.decode(raw, &payload)
	if !strings.Contains(payload.AuthURL, "accounts.google.com") {
		t.Fatalf("адрес согласия собран неверно: %q", payload.AuthURL)
	}
}

func TestGmailCallbackRejectsForeignState(t *testing.T) {
	e := newEnv(t)
	e.user()
	status, raw := e.do(http.MethodGet, "/api/connections/gmail/callback?code=код&state=чужое", nil)
	if status != http.StatusBadRequest {
		t.Fatalf("чужой state вернул %d", status)
	}
	if e.detail(raw) != "Авторизация Google не подтверждена" {
		t.Fatalf("текст ошибки: %q", e.detail(raw))
	}
}

// Входа нет: список источников отдаётся сразу, на чистой базе тоже
// (решение №50). Здесь только GET: остальные ручки заводят подключения
// и проверяются своими тестами.
func TestConnectionsOpenWithoutLogin(t *testing.T) {
	e := newEnv(t)
	status, raw := e.do(http.MethodGet, "/api/connections", nil)
	if status != http.StatusOK {
		t.Fatalf("список источников вернул %d", status)
	}
	var connections []schemas.Connection
	e.decode(raw, &connections)
	if len(connections) != 2 {
		t.Fatalf("источников в ответе: %d", len(connections))
	}
}
