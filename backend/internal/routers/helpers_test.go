package routers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
	"time"

	"personalinbox/internal/core"
	"personalinbox/internal/events"
	"personalinbox/internal/gmail"
	"personalinbox/internal/postgres"
	"personalinbox/internal/redis"
	"personalinbox/internal/services/ingest"
	"personalinbox/internal/telegram"
	"personalinbox/internal/testenv"
	"personalinbox/internal/utils/security"
)

// messageModel — короткое имя для правок сообщения в тестах.
type messageModel = postgres.Message

// fakeQueue заменяет рабочий поток оценки: ручкам важно, что сообщение
// поставлено в очередь, а не то, что ответила модель.
type fakeQueue struct{ ids []int64 }

func (q *fakeQueue) Enqueue(id int64) { q.ids = append(q.ids, id) }

type env struct {
	t        *testing.T
	server   *httptest.Server
	client   *http.Client
	db       *postgres.DB
	cache    *redis.Client
	bus      *events.Bus
	queue    *fakeQueue
	gmail    *gmail.Client
	telegram *telegram.Client
	counter  int
}

const testPassword = "qwerty12345"

func newEnv(t *testing.T) *env {
	t.Helper()
	db := testenv.DB(t)
	cache := testenv.Cache(t)

	cfg := core.Config{FrontendOrigin: "http://localhost:5173"}
	bus := events.New(50)
	// Тот же сброс кэша, что и в бою: иначе тесты проверяли бы не то поведение.
	bus.OnPublish(func(userID int64) { cache.DropCache(context.Background(), userID) })
	queue := &fakeQueue{}
	ingestor := ingest.New(db, bus, queue)
	gmailClient := &gmail.Client{}
	telegramClient := telegram.NewClient()

	server := httptest.NewServer(
		New(cfg, db, cache, bus, ingestor, queue, gmailClient, telegramClient).Handler())
	t.Cleanup(server.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &env{
		t: t, server: server, db: db, cache: cache, bus: bus, queue: queue,
		gmail: gmailClient, telegram: telegramClient,
		client: &http.Client{Jar: jar, Timeout: 10 * time.Second,
			// Редирект после OAuth проверяем сами, а не следуем за ним.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			}},
	}
}

// do выполняет запрос к тестовому серверу и возвращает статус и тело.
func (e *env) do(method, path string, body any) (int, []byte) {
	e.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			e.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	request, err := http.NewRequest(method, e.server.URL+path, reader)
	if err != nil {
		e.t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := e.client.Do(request)
	if err != nil {
		e.t.Fatalf("запрос %s %s: %v", method, path, err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		e.t.Fatal(err)
	}
	return response.StatusCode, raw
}

// decode разбирает тело ответа в структуру.
func (e *env) decode(raw []byte, target any) {
	e.t.Helper()
	if err := json.Unmarshal(raw, target); err != nil {
		e.t.Fatalf("ответ не разобрать: %v\n%s", err, raw)
	}
}

// detail достаёт текст ошибки в формате, который читает фронт.
func (e *env) detail(raw []byte) string {
	e.t.Helper()
	var payload struct {
		Detail string `json:"detail"`
	}
	e.decode(raw, &payload)
	return payload.Detail
}

// user создаёт пользователя прямо в базе, минуя регистрацию.
func (e *env) user(email string) *postgres.User {
	e.t.Helper()
	hash, err := security.HashPassword(testPassword)
	if err != nil {
		e.t.Fatal(err)
	}
	user, err := e.db.CreateUser(email, hash, "Важны договоры и сроки.")
	if err != nil {
		e.t.Fatal(err)
	}
	return user
}

// login выполняет вход: cookie сохраняется в jar клиента.
func (e *env) login(email string) {
	e.t.Helper()
	status, raw := e.do(http.MethodPost, "/api/auth/login",
		map[string]string{"email": email, "password": testPassword})
	if status != http.StatusOK {
		e.t.Fatalf("вход не удался: %d %s", status, raw)
	}
}

// authorized создаёт пользователя и сразу входит под ним.
func (e *env) authorized() *postgres.User {
	e.t.Helper()
	user := e.user("max@northline.io")
	e.login(user.Email)
	return user
}

func (e *env) connection(user *postgres.User, kind, state string) *postgres.Connection {
	e.t.Helper()
	connection, err := e.db.GetOrCreateConnection(user.ID, kind)
	if err != nil {
		e.t.Fatal(err)
	}
	now := postgres.UTCNow()
	connection.State = state
	connection.Account = map[string]string{"gmail": "me@northline.io", "telegram": "@maxorlov"}[kind]
	connection.Credentials = `{"bot_token": "123:abc"}`
	connection.LastSyncAt = &now
	if err := e.db.SaveConnection(connection); err != nil {
		e.t.Fatal(err)
	}
	return connection
}

// message создаёт сообщение с разумными значениями по умолчанию.
func (e *env) message(connection *postgres.Connection, apply func(*postgres.Message)) *postgres.Message {
	e.t.Helper()
	e.counter++
	message := &postgres.Message{
		ConnectionID: connection.ID,
		ExternalID:   "ext-" + time.Now().Format("150405.000000000"),
		SenderName:   "Анна Ковалёва",
		SenderAddr:   "a.kovaleva@northline.io",
		Subject:      "Договор Northline",
		Body:         "Нужны правки по пунктам 4.2 и 7",
		ReceivedAt:   postgres.UTCNow().Add(-time.Duration(e.counter) * time.Minute),
		Status:       "DONE",
		Level:        "NORMAL",
		Category:     "Работа",
		Summary:      "Краткое содержание",
		Kind:         connection.Kind,
	}
	if apply != nil {
		apply(message)
	}
	if err := e.db.InsertMessage(message); err != nil {
		e.t.Fatal(err)
	}
	return message
}
