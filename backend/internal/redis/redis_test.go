package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"
)

// newClient — свой префикс ключей на каждый тест. Свой хелпер, а не
// internal/testenv: тот импортирует этот пакет, и вышел бы цикл.
func newClient(t *testing.T) *Client {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		url = os.Getenv("REDIS_URL")
	}
	if url == "" {
		url = "redis://localhost:6379/0"
	}
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("случайный префикс: %v", err)
	}
	client, err := OpenWithPrefix(url, "test:"+hex.EncodeToString(raw)+":")
	if err != nil {
		t.Fatalf("redis недоступен (%s): %v", url, err)
	}
	t.Cleanup(func() {
		client.DropPrefix(context.Background())
		client.Close()
	})
	return client
}

func TestSessionRoundTrip(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	token, err := client.NewSession(ctx, Session{UserID: 42}, time.Minute)
	if err != nil {
		t.Fatalf("создать сессию: %v", err)
	}
	if token == "" {
		t.Fatal("пустой токен")
	}
	value, ok := client.Session(ctx, token)
	if !ok || value.UserID != 42 {
		t.Fatalf("сессия не прочиталась: %+v ok=%v", value, ok)
	}
}

func TestSessionTokensAreUnique(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	seen := make(map[string]bool, 50)
	for i := 0; i < 50; i++ {
		token, err := client.NewSession(ctx, Session{UserID: 1}, time.Minute)
		if err != nil {
			t.Fatalf("создать сессию: %v", err)
		}
		if seen[token] {
			t.Fatalf("токен повторился: %s", token)
		}
		seen[token] = true
	}
}

func TestUnknownTokenIsNotASession(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	for _, token := range []string{"", "выдуманный", "AAAAAAAAAAAAAAAAAAAAAAAA"} {
		if _, ok := client.Session(ctx, token); ok {
			t.Fatalf("чужой токен %q принят за сессию", token)
		}
	}
}

func TestDropSessionEndsIt(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	token, err := client.NewSession(ctx, Session{UserID: 7}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.DropSession(ctx, token); err != nil {
		t.Fatalf("погасить сессию: %v", err)
	}
	if _, ok := client.Session(ctx, token); ok {
		t.Fatal("сессия жива после выхода")
	}
	// Повторный выход — не ошибка: пользователь мог нажать «выйти» дважды.
	if err := client.DropSession(ctx, token); err != nil {
		t.Fatalf("повторный выход вернул ошибку: %v", err)
	}
}

func TestExpiredSessionIsGone(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	token, err := client.NewSession(ctx, Session{UserID: 9}, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(120 * time.Millisecond)
	if _, ok := client.Session(ctx, token); ok {
		t.Fatal("просроченная сессия всё ещё принимается")
	}
}

func TestSaveSessionKeepsToken(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	token, err := client.NewSession(ctx, Session{UserID: 3}, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SaveSession(ctx, token, Session{UserID: 3, OAuthState: "abc"}, time.Minute); err != nil {
		t.Fatalf("сохранить сессию: %v", err)
	}
	value, ok := client.Session(ctx, token)
	if !ok || value.OAuthState != "abc" || value.UserID != 3 {
		t.Fatalf("сессия после правки: %+v ok=%v", value, ok)
	}
	// Пустой токен — ошибка, а не молчаливая запись в ключ "session:".
	if err := client.SaveSession(ctx, "", Session{UserID: 3}, time.Minute); err == nil {
		t.Fatal("запись по пустому токену прошла")
	}
}

func TestCacheRoundTripAndDrop(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	type payload struct {
		Total int      `json:"total"`
		Names []string `json:"names"`
	}
	want := payload{Total: 19, Names: []string{"Gmail", "Telegram"}}
	client.SetCached(ctx, 1, "sources", want)

	var got payload
	if !client.GetCached(ctx, 1, "sources", &got) {
		t.Fatal("значение не нашлось в кэше")
	}
	if got.Total != want.Total || len(got.Names) != 2 || got.Names[0] != "Gmail" {
		t.Fatalf("из кэша пришло другое: %+v", got)
	}

	// Кэш одного пользователя не виден другому.
	var other payload
	if client.GetCached(ctx, 2, "sources", &other) {
		t.Fatal("кэш утёк к другому пользователю")
	}

	client.DropCache(ctx, 1)
	if client.GetCached(ctx, 1, "sources", &got) {
		t.Fatal("кэш пережил сброс")
	}
}

func TestDropCacheClearsAllNamesOfUser(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	client.SetCached(ctx, 5, "sources", map[string]int{"a": 1})
	client.SetCached(ctx, 5, "summary:24h", map[string]int{"b": 2})
	client.SetCached(ctx, 6, "sources", map[string]int{"c": 3})

	client.DropCache(ctx, 5)

	var value map[string]int
	if client.GetCached(ctx, 5, "sources", &value) {
		t.Fatal("sources пятого пользователя не сброшен")
	}
	if client.GetCached(ctx, 5, "summary:24h", &value) {
		t.Fatal("summary пятого пользователя не сброшен")
	}
	if !client.GetCached(ctx, 6, "sources", &value) {
		t.Fatal("сброс задел чужого пользователя")
	}
	// Сброс пустого кэша не должен падать.
	client.DropCache(ctx, 999)
}

func TestOpenRejectsBadURL(t *testing.T) {
	if _, err := Open("не-адрес"); err == nil {
		t.Fatal("кривой адрес принят")
	}
	if _, err := Open("redis://127.0.0.1:1/0"); err == nil {
		t.Fatal("недоступный redis принят")
	}
}
