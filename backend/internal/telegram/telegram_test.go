package telegram

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"personalinbox/internal/events"
	"personalinbox/internal/services/ingest"
	"personalinbox/internal/sqlite"
)

type fakeQueue struct{ ids []int64 }

func (q *fakeQueue) Enqueue(id int64) { q.ids = append(q.ids, id) }

// botAPI поднимает поддельный Bot API: ключ — имя метода.
func botAPI(t *testing.T, handlers map[string]func(request map[string]any) (any, bool)) (*Client, *[]string) {
	t.Helper()
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		method := parts[len(parts)-1]
		calls = append(calls, method)

		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)

		handler, ok := handlers[method]
		if !ok {
			w.Write([]byte(`{"ok": false, "description": "Unauthorized: неизвестный метод"}`))
			return
		}
		result, success := handler(payload)
		if !success {
			body, _ := json.Marshal(map[string]any{"ok": false, "description": result})
			w.Write(body)
			return
		}
		body, _ := json.Marshal(map[string]any{"ok": true, "result": result})
		w.Write(body)
	}))
	t.Cleanup(server.Close)
	return &Client{BaseURL: server.URL, HTTP: server.Client()}, &calls
}

func newIngestor(t *testing.T) (*ingest.Ingestor, *sqlite.Connection) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	user, err := db.CreateUser("max@northline.io", "хеш", "")
	if err != nil {
		t.Fatal(err)
	}
	connection, err := db.GetOrCreateConnection(user.ID, "telegram")
	if err != nil {
		t.Fatal(err)
	}
	connection.State = "active"
	connection.Credentials = `{"bot_token": "123:abc"}`
	if err := db.SaveConnection(connection); err != nil {
		t.Fatal(err)
	}
	return ingest.New(db, events.New(50), &fakeQueue{}), connection
}

func makeUpdate(chat map[string]any, text string, updateID, messageID int) map[string]any {
	return map[string]any{
		"update_id": updateID,
		"message": map[string]any{
			"message_id": messageID,
			"date":       1756800000,
			"text":       text,
			"chat":       chat,
		},
	}
}

func TestVerifyTokenReturnsBotUsername(t *testing.T) {
	client, _ := botAPI(t, map[string]func(map[string]any) (any, bool){
		"getMe": func(map[string]any) (any, bool) {
			return map[string]any{"username": "maxorlov_bot", "first_name": "Инбокс"}, true
		},
	})
	name, err := client.VerifyToken("123:abc")
	if err != nil {
		t.Fatal(err)
	}
	if name != "@maxorlov_bot" {
		t.Fatalf("имя бота собрано неверно: %q", name)
	}
}

func TestVerifyTokenFallsBackToFirstName(t *testing.T) {
	client, _ := botAPI(t, map[string]func(map[string]any) (any, bool){
		"getMe": func(map[string]any) (any, bool) {
			return map[string]any{"first_name": "Инбокс"}, true
		},
	})
	name, err := client.VerifyToken("123:abc")
	if err != nil || name != "Инбокс" {
		t.Fatalf("ожидалось имя без собаки: %q, %v", name, err)
	}
}

func TestVerifyTokenRejectsBadToken(t *testing.T) {
	client, _ := botAPI(t, map[string]func(map[string]any) (any, bool){
		"getMe": func(map[string]any) (any, bool) {
			return "Unauthorized", false
		},
	})
	if _, err := client.VerifyToken("плохой"); err == nil {
		t.Fatal("неверный токен должен быть ошибкой")
	}
}

func TestSyncStoresPrivateMessage(t *testing.T) {
	client, _ := botAPI(t, map[string]func(map[string]any) (any, bool){
		"getUpdates": func(map[string]any) (any, bool) {
			return []any{makeUpdate(map[string]any{
				"id": 42, "type": "private", "first_name": "Дима", "username": "dmitry_pm",
			}, "Релиз 2.4 — блокер", 10, 7)}, true
		},
	})
	ingestor, connection := newIngestor(t)

	saved, err := client.Sync(ingestor, connection)
	if err != nil {
		t.Fatal(err)
	}
	if saved != 1 {
		t.Fatalf("сохранено %d сообщений вместо одного", saved)
	}
	message, err := ingestor.DB.MessageByExternalID(connection.ID, "42:7")
	if err != nil {
		t.Fatalf("сообщение не найдено: %v", err)
	}
	if message.SenderName != "Дима" || message.SenderAddr != "@dmitry_pm" {
		t.Fatalf("отправитель разобран неверно: %+v", message)
	}
	if message.ExternalURL != "https://t.me/dmitry_pm/7" {
		t.Fatalf("ссылка собрана неверно: %q", message.ExternalURL)
	}
	if connection.SyncCursor != "10" {
		t.Fatalf("курсор не сохранён: %q", connection.SyncCursor)
	}
}

func TestGroupChatShowsMemberCount(t *testing.T) {
	client, calls := botAPI(t, map[string]func(map[string]any) (any, bool){
		"getUpdates": func(map[string]any) (any, bool) {
			chat := map[string]any{"id": 55, "type": "supergroup", "title": "Инвесторы · Seed round"}
			return []any{
				makeUpdate(chat, "Первое", 1, 1),
				makeUpdate(chat, "Второе", 2, 2),
			}, true
		},
		"getChatMemberCount": func(map[string]any) (any, bool) { return 9, true },
	})
	ingestor, connection := newIngestor(t)

	if _, err := client.Sync(ingestor, connection); err != nil {
		t.Fatal(err)
	}
	message, err := ingestor.DB.MessageByExternalID(connection.ID, "55:1")
	if err != nil {
		t.Fatal(err)
	}
	if message.SenderAddr != "групповой чат, 9 участников" {
		t.Fatalf("адрес группового чата собран неверно: %q", message.SenderAddr)
	}
	if message.ExternalURL != "" {
		t.Fatalf("у чата без username не должно быть ссылки: %q", message.ExternalURL)
	}

	counts := 0
	for _, call := range *calls {
		if call == "getChatMemberCount" {
			counts++
		}
	}
	if counts != 1 {
		t.Fatalf("счётчик участников запрошен %d раз вместо одного", counts)
	}
}

func TestGroupWithoutMemberCountDegradesGracefully(t *testing.T) {
	client, _ := botAPI(t, map[string]func(map[string]any) (any, bool){
		"getUpdates": func(map[string]any) (any, bool) {
			return []any{makeUpdate(map[string]any{
				"id": 55, "type": "group", "title": "Дизайн-ревью",
			}, "Обсудили навигацию", 1, 1)}, true
		},
		"getChatMemberCount": func(map[string]any) (any, bool) { return "Forbidden", false },
	})
	ingestor, connection := newIngestor(t)

	if _, err := client.Sync(ingestor, connection); err != nil {
		t.Fatal(err)
	}
	message, err := ingestor.DB.MessageByExternalID(connection.ID, "55:1")
	if err != nil {
		t.Fatal(err)
	}
	if message.SenderAddr != "групповой чат" {
		t.Fatalf("без счётчика адрес должен остаться общим: %q", message.SenderAddr)
	}
}

func TestSubjectIsFirstLine(t *testing.T) {
	client, _ := botAPI(t, map[string]func(map[string]any) (any, bool){
		"getUpdates": func(map[string]any) (any, bool) {
			return []any{makeUpdate(map[string]any{"id": 1, "type": "private", "first_name": "Лена"},
				"Нужны закрывающие\nПо трём подрядчикам", 1, 1)}, true
		},
	})
	ingestor, connection := newIngestor(t)

	if _, err := client.Sync(ingestor, connection); err != nil {
		t.Fatal(err)
	}
	message, err := ingestor.DB.MessageByExternalID(connection.ID, "1:1")
	if err != nil {
		t.Fatal(err)
	}
	if message.Subject != "Нужны закрывающие" {
		t.Fatalf("тема должна быть первой строкой: %q", message.Subject)
	}
	if !strings.Contains(message.Body, "По трём подрядчикам") {
		t.Fatal("тело должно содержать сообщение целиком")
	}
}

func TestMessageWithoutTextIsSkipped(t *testing.T) {
	client, _ := botAPI(t, map[string]func(map[string]any) (any, bool){
		"getUpdates": func(map[string]any) (any, bool) {
			return []any{map[string]any{
				"update_id": 5,
				"message": map[string]any{
					"message_id": 1, "date": 1756800000,
					"chat": map[string]any{"id": 1, "type": "private", "first_name": "Саша"},
				},
			}}, true
		},
	})
	ingestor, connection := newIngestor(t)

	saved, err := client.Sync(ingestor, connection)
	if err != nil {
		t.Fatal(err)
	}
	if saved != 0 {
		t.Fatal("медиа без подписи оценивать нечем, сохранять не нужно")
	}
	if connection.SyncCursor != "5" {
		t.Fatalf("курсор должен сдвинуться и на пропущенном обновлении: %q", connection.SyncCursor)
	}
}

func TestNextSyncAsksForUpdatesAfterCursor(t *testing.T) {
	var offset any
	client, _ := botAPI(t, map[string]func(map[string]any) (any, bool){
		"getUpdates": func(request map[string]any) (any, bool) {
			offset = request["offset"]
			return []any{}, true
		},
	})
	ingestor, connection := newIngestor(t)
	connection.SyncCursor = "17"
	if err := ingestor.DB.SaveConnection(connection); err != nil {
		t.Fatal(err)
	}

	if _, err := client.Sync(ingestor, connection); err != nil {
		t.Fatal(err)
	}
	if offset != float64(18) {
		t.Fatalf("следующий запрос должен идти с offset 18, ушёл %v", offset)
	}
}

func TestInvalidTokenSwitchesToReauth(t *testing.T) {
	client, _ := botAPI(t, map[string]func(map[string]any) (any, bool){
		"getUpdates": func(map[string]any) (any, bool) {
			return "Unauthorized", false
		},
	})
	ingestor, connection := newIngestor(t)

	if _, err := client.Sync(ingestor, connection); err != nil {
		t.Fatal(err)
	}
	if connection.State != "reauth" {
		t.Fatalf("неверный токен должен просить переподключения, состояние %q", connection.State)
	}
}

func TestNetworkErrorKeepsConnectionActive(t *testing.T) {
	// Сервер поднят и сразу закрыт: обращение к нему даёт сетевую ошибку.
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	address := server.URL
	server.Close()
	client := &Client{BaseURL: address}
	ingestor, connection := newIngestor(t)

	if _, err := client.Sync(ingestor, connection); err != nil {
		t.Fatal(err)
	}
	if connection.State != "active" {
		t.Fatalf("сетевой сбой не повод просить переподключение: %q", connection.State)
	}
}

func TestMissingTokenSwitchesToReauth(t *testing.T) {
	client, _ := botAPI(t, map[string]func(map[string]any) (any, bool){})
	ingestor, connection := newIngestor(t)
	connection.Credentials = ""
	if err := ingestor.DB.SaveConnection(connection); err != nil {
		t.Fatal(err)
	}

	if _, err := client.Sync(ingestor, connection); err != nil {
		t.Fatal(err)
	}
	if connection.State != "reauth" {
		t.Fatalf("без токена состояние должно стать reauth, стало %q", connection.State)
	}
}
