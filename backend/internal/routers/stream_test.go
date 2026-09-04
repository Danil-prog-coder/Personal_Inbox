package routers

import (
	"bufio"
	"net/http"
	"strings"
	"testing"
	"time"

	"personalinbox/internal/schemas"
)

func TestStreamRequiresAuth(t *testing.T) {
	e := newEnv(t)
	if status, _ := e.do(http.MethodGet, "/api/stream", nil); status != http.StatusUnauthorized {
		t.Fatalf("поток без входа вернул %d", status)
	}
}

func TestStreamSendsNewEvents(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	connection := e.connection(user, "gmail", "active")

	request, err := http.NewRequest(http.MethodGet, e.server.URL+"/api/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Cookie сессии берём из того же клиента, что выполнял вход.
	for _, cookie := range e.client.Jar.Cookies(request.URL) {
		request.AddCookie(cookie)
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("тип ответа потока: %q", got)
	}

	reader := bufio.NewReader(response.Body)
	// Первый байт приходит сразу, иначе браузер держит соединение «висящим».
	first, err := reader.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first, ": ok") {
		t.Fatalf("поток не открылся сразу: %q", first)
	}

	message := e.message(connection, func(m *messageModel) { m.Subject = "Новое письмо" })
	e.bus.Publish(user.ID, "message.created", schemas.MessageOut(message))

	deadline := time.Now().Add(5 * time.Second)
	var event, data string
	for time.Now().Before(deadline) {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("чтение потока: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case strings.HasPrefix(line, "event: "):
			event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data = strings.TrimPrefix(line, "data: ")
		}
		if event != "" && data != "" {
			break
		}
	}
	if event != "message.created" {
		t.Fatalf("имя события: %q", event)
	}
	if !strings.Contains(data, "Новое письмо") {
		t.Fatalf("тело события: %s", data)
	}
}

func TestStreamSkipsEventsOfOtherUsers(t *testing.T) {
	e := newEnv(t)
	user := e.authorized()
	stranger := e.user("other@northline.io")

	cursor := e.bus.Cursor()
	e.bus.Publish(stranger.ID, "message.created", map[string]string{"subject": "чужое"})

	received, _ := e.bus.Since(user.ID, cursor)
	if len(received) != 0 {
		t.Fatalf("подписчик получил чужие события: %v", received)
	}
}
