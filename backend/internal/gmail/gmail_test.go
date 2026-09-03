package gmail

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"personalinbox/internal/core"
	"personalinbox/internal/events"
	"personalinbox/internal/services/ingest"
	"personalinbox/internal/sqlite"
)

type fakeQueue struct{ ids []int64 }

func (q *fakeQueue) Enqueue(id int64) { q.ids = append(q.ids, id) }

func encode(text string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(text))
}

func letter(id string, headers map[string]string, body payload) rawMessage {
	raw := rawMessage{ID: id, Payload: body}
	for name, value := range headers {
		raw.Payload.Headers = append(raw.Payload.Headers, struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		}{Name: name, Value: value})
	}
	return raw
}

func plain(text string) payload {
	source := payload{MimeType: "text/plain"}
	source.Body.Data = encode(text)
	return source
}

func TestParsesSenderSubjectAndBody(t *testing.T) {
	incoming := ToIncoming(letter("m1", map[string]string{
		"From":    "Анна Ковалёва <a.kovaleva@northline.io>",
		"Subject": "Договор Northline",
	}, plain("Нужны правки по пунктам 4.2 и 7.")))

	if incoming.SenderName != "Анна Ковалёва" || incoming.SenderAddr != "a.kovaleva@northline.io" {
		t.Fatalf("отправитель разобран неверно: %+v", incoming)
	}
	if incoming.Subject != "Договор Northline" {
		t.Fatalf("тема разобрана неверно: %q", incoming.Subject)
	}
	if !strings.Contains(incoming.Body, "4.2") {
		t.Fatalf("тело письма потеряно: %q", incoming.Body)
	}
	if incoming.ExternalURL != "https://mail.google.com/mail/u/0/#inbox/m1" {
		t.Fatalf("ссылка собрана неверно: %q", incoming.ExternalURL)
	}
}

func TestLetterWithoutSubjectAndSenderHasDefaults(t *testing.T) {
	incoming := ToIncoming(letter("m1", map[string]string{}, plain("текст")))
	if incoming.Subject != "(без темы)" {
		t.Fatalf("подпись пустой темы: %q", incoming.Subject)
	}
	if incoming.SenderName != "Без отправителя" {
		t.Fatalf("подпись пустого отправителя: %q", incoming.SenderName)
	}
}

func TestUsesInternalDate(t *testing.T) {
	raw := letter("m1", map[string]string{"Date": "Mon, 1 Sep 2025 10:00:00 +0300"}, plain("текст"))
	raw.InternalDate = "1756800000000"
	incoming := ToIncoming(raw)
	if !incoming.ReceivedAt.Equal(time.UnixMilli(1756800000000).UTC()) {
		t.Fatalf("internalDate имеет приоритет: %v", incoming.ReceivedAt)
	}
}

func TestFallsBackToDateHeader(t *testing.T) {
	incoming := ToIncoming(letter("m1",
		map[string]string{"Date": "Tue, 2 Sep 2025 12:00:00 +0300"}, plain("текст")))
	want := time.Date(2025, 9, 2, 9, 0, 0, 0, time.UTC)
	if !incoming.ReceivedAt.Equal(want) {
		t.Fatalf("заголовок Date разобран неверно: %v", incoming.ReceivedAt)
	}
}

func TestMultipartPrefersPlainText(t *testing.T) {
	html := payload{MimeType: "text/html"}
	html.Body.Data = encode("<p>Версия в HTML</p>")
	source := payload{MimeType: "multipart/alternative", Parts: []payload{
		html,
		plain("Версия обычным текстом"),
	}}
	incoming := ToIncoming(letter("m1", map[string]string{}, source))
	if incoming.Body != "Версия обычным текстом" {
		t.Fatalf("должна выбираться текстовая часть: %q", incoming.Body)
	}
}

func TestHTMLOnlyLetterIsStripped(t *testing.T) {
	source := payload{MimeType: "text/html"}
	source.Body.Data = encode(
		"<style>p{color:red}</style><p>Счёт <b>оплачен</b></p><script>alert(1)</script>")
	incoming := ToIncoming(letter("m1", map[string]string{}, source))
	if strings.Contains(incoming.Body, "<") || strings.Contains(incoming.Body, "alert") {
		t.Fatalf("разметка и скрипты должны вырезаться: %q", incoming.Body)
	}
	if !strings.Contains(incoming.Body, "Счёт оплачен") {
		t.Fatalf("текст письма потерян: %q", incoming.Body)
	}
}

func TestLetterWithoutBodyUsesSnippet(t *testing.T) {
	raw := letter("m1", map[string]string{}, payload{MimeType: "multipart/mixed"})
	raw.Snippet = "Короткая выжимка"
	if body := ToIncoming(raw).Body; body != "Короткая выжимка" {
		t.Fatalf("должен подставляться snippet: %q", body)
	}
}

func TestBrokenBase64DoesNotCrash(t *testing.T) {
	source := payload{MimeType: "text/plain"}
	source.Body.Data = "не base64!!!"
	raw := letter("m1", map[string]string{}, source)
	raw.Snippet = "выжимка"
	if body := ToIncoming(raw).Body; body != "выжимка" {
		t.Fatalf("битое тело не должно ронять разбор: %q", body)
	}
}

func TestAuthErrorsAreRecognized(t *testing.T) {
	for _, text := range []string{"invalid_grant", "401 Unauthorized", "HTTP 403"} {
		if !IsAuthError(errTest(text)) {
			t.Fatalf("ошибка доступа не распознана: %q", text)
		}
	}
	if IsAuthError(errTest("connection reset")) {
		t.Fatal("сетевой сбой не должен считаться отказом в доступе")
	}
}

type errTest string

func (e errTest) Error() string { return string(e) }

func TestAuthURLRequiresClientCredentials(t *testing.T) {
	client := &Client{}
	if _, err := client.AuthURL("state"); err == nil {
		t.Fatal("без ключей Google адрес согласия собрать нельзя")
	}

	ready := &Client{ClientID: "id", ClientSecret: "secret",
		RedirectURI: "http://localhost:8000/api/connections/gmail/callback"}
	url, err := ready.AuthURL("состояние")
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"access_type=offline", "state=", "client_id=id",
		"gmail.readonly"} {
		if !strings.Contains(url, fragment) {
			t.Fatalf("в адресе согласия нет %q: %s", fragment, url)
		}
	}
}

// gmailAPI поднимает поддельные token endpoint и Gmail API.
func gmailAPI(t *testing.T, routes map[string]func(r *http.Request) (int, string)) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for prefix, handler := range routes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				status, body := handler(r)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				w.Write([]byte(body))
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "not found"}`))
	}))
	t.Cleanup(server.Close)
	return &Client{
		ClientID: "id", ClientSecret: "secret", RedirectURI: "http://localhost:8000/cb",
		TokenURL: server.URL + "/token", APIBase: server.URL + "/gmail/v1",
		HTTP: server.Client(),
	}
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
	connection, err := db.GetOrCreateConnection(user.ID, "gmail")
	if err != nil {
		t.Fatal(err)
	}
	connection.State = "active"
	connection.Credentials = `{"refresh_token": "тест"}`
	if err := db.SaveConnection(connection); err != nil {
		t.Fatal(err)
	}
	return ingest.New(db, events.New(50), &fakeQueue{}), connection
}

func messageJSON(id, subject string) string {
	raw, _ := json.Marshal(map[string]any{
		"id":           id,
		"internalDate": "1756800000000",
		"payload": map[string]any{
			"mimeType": "text/plain",
			"headers": []map[string]string{
				{"name": "From", "value": "Анна <a.kovaleva@northline.io>"},
				{"name": "Subject", "value": subject},
			},
			"body": map[string]string{"data": encode("текст письма")},
		},
	})
	return string(raw)
}

func TestFirstSyncImportsAndStoresHistoryID(t *testing.T) {
	client := gmailAPI(t, map[string]func(*http.Request) (int, string){
		"/token": func(*http.Request) (int, string) {
			return 200, `{"access_token": "токен"}`
		},
		"/gmail/v1/users/me/messages/m1": func(*http.Request) (int, string) {
			return 200, messageJSON("m1", "Договор Northline")
		},
		"/gmail/v1/users/me/messages": func(r *http.Request) (int, string) {
			if !strings.Contains(r.URL.RawQuery, "after") {
				t.Errorf("первый импорт должен ограничиваться датой: %s", r.URL.RawQuery)
			}
			return 200, `{"messages": [{"id": "m1"}]}`
		},
		"/gmail/v1/users/me/profile": func(*http.Request) (int, string) {
			return 200, `{"emailAddress": "me@northline.io", "historyId": "9001"}`
		},
	})
	ingestor, connection := newIngestor(t)

	saved, err := client.Sync(ingestor, connection)
	if err != nil {
		t.Fatal(err)
	}
	if saved != 1 {
		t.Fatalf("сохранено %d писем вместо одного", saved)
	}
	if connection.SyncCursor != "9001" {
		t.Fatalf("historyId не сохранён: %q", connection.SyncCursor)
	}
	if connection.LastSyncAt == nil {
		t.Fatal("время синхронизации не проставлено")
	}
}

func TestIncrementalSyncKeepsCursor(t *testing.T) {
	client := gmailAPI(t, map[string]func(*http.Request) (int, string){
		"/token": func(*http.Request) (int, string) { return 200, `{"access_token": "токен"}` },
		"/gmail/v1/users/me/history": func(r *http.Request) (int, string) {
			if r.URL.Query().Get("startHistoryId") != "9001" {
				t.Errorf("инкремент должен идти от сохранённого historyId: %s", r.URL.RawQuery)
			}
			return 200, `{"history": [{"messagesAdded": [{"message": {"id": "m2"}}]}],
				"historyId": "9100"}`
		},
		"/gmail/v1/users/me/messages/m2": func(*http.Request) (int, string) {
			return 200, messageJSON("m2", "Новое письмо")
		},
	})
	ingestor, connection := newIngestor(t)
	connection.SyncCursor = "9001"
	if err := ingestor.DB.SaveConnection(connection); err != nil {
		t.Fatal(err)
	}

	saved, err := client.Sync(ingestor, connection)
	if err != nil {
		t.Fatal(err)
	}
	if saved != 1 {
		t.Fatalf("инкремент сохранил %d писем", saved)
	}
	if connection.SyncCursor != "9100" {
		t.Fatalf("курсор не обновлён: %q", connection.SyncCursor)
	}
}

func TestStaleHistoryIDTriggersFullImport(t *testing.T) {
	listed := false
	client := gmailAPI(t, map[string]func(*http.Request) (int, string){
		"/token": func(*http.Request) (int, string) { return 200, `{"access_token": "токен"}` },
		"/gmail/v1/users/me/history": func(*http.Request) (int, string) {
			return 404, `{"error": {"code": 404, "message": "startHistoryId not found"}}`
		},
		"/gmail/v1/users/me/messages": func(*http.Request) (int, string) {
			listed = true
			return 200, `{"messages": []}`
		},
		"/gmail/v1/users/me/profile": func(*http.Request) (int, string) {
			return 200, `{"emailAddress": "me@northline.io", "historyId": "9200"}`
		},
	})
	ingestor, connection := newIngestor(t)
	connection.SyncCursor = "1"
	if err := ingestor.DB.SaveConnection(connection); err != nil {
		t.Fatal(err)
	}

	if _, err := client.Sync(ingestor, connection); err != nil {
		t.Fatal(err)
	}
	if !listed {
		t.Fatal("протухший historyId должен запускать импорт заново")
	}
	if connection.State != "active" {
		t.Fatalf("состояние не должно меняться: %q", connection.State)
	}
}

func TestExpiredTokenSwitchesToReauth(t *testing.T) {
	client := gmailAPI(t, map[string]func(*http.Request) (int, string){
		"/token": func(*http.Request) (int, string) {
			return 400, `{"error": "invalid_grant"}`
		},
	})
	ingestor, connection := newIngestor(t)

	if _, err := client.Sync(ingestor, connection); err != nil {
		t.Fatal(err)
	}
	if connection.State != "reauth" {
		t.Fatalf("истёкший токен должен просить переподключения: %q", connection.State)
	}
}

func TestSyncWithoutCredentialsSwitchesToReauth(t *testing.T) {
	client := gmailAPI(t, map[string]func(*http.Request) (int, string){})
	ingestor, connection := newIngestor(t)
	connection.Credentials = ""
	if err := ingestor.DB.SaveConnection(connection); err != nil {
		t.Fatal(err)
	}

	if _, err := client.Sync(ingestor, connection); err != nil {
		t.Fatal(err)
	}
	if connection.State != "reauth" {
		t.Fatalf("без refresh_token состояние должно стать reauth: %q", connection.State)
	}
}

func TestNetworkErrorKeepsConnectionActive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	address := server.URL
	server.Close()
	client := &Client{ClientID: "id", ClientSecret: "secret",
		TokenURL: address + "/token", APIBase: address + "/gmail/v1"}
	ingestor, connection := newIngestor(t)

	if _, err := client.Sync(ingestor, connection); err != nil {
		t.Fatal(err)
	}
	if connection.State != "active" {
		t.Fatalf("сетевой сбой не повод просить переподключение: %q", connection.State)
	}
}

func TestExchangeCodeRequiresRefreshToken(t *testing.T) {
	client := gmailAPI(t, map[string]func(*http.Request) (int, string){
		"/token": func(*http.Request) (int, string) {
			return 200, `{"access_token": "токен"}`
		},
	})
	if _, _, err := client.ExchangeCode("код"); err == nil {
		t.Fatal("без refresh_token подключение невозможно")
	}
}

func TestExchangeCodeReturnsAccountEmail(t *testing.T) {
	client := gmailAPI(t, map[string]func(*http.Request) (int, string){
		"/token": func(*http.Request) (int, string) {
			return 200, `{"access_token": "токен", "refresh_token": "обновление"}`
		},
		"/gmail/v1/users/me/profile": func(*http.Request) (int, string) {
			return 200, `{"emailAddress": "me@northline.io", "historyId": "1"}`
		},
	})
	email, refresh, err := client.ExchangeCode("код")
	if err != nil {
		t.Fatal(err)
	}
	if email != "me@northline.io" || refresh != "обновление" {
		t.Fatalf("обмен кода вернул %q / %q", email, refresh)
	}
}

func TestImportLimitIsRespected(t *testing.T) {
	if core.GmailImportLimit != 200 || core.GmailImportDays != 30 {
		t.Fatalf("границы первого импорта изменены: %d писем за %d дней",
			core.GmailImportLimit, core.GmailImportDays)
	}
}
