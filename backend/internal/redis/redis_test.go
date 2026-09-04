package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
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

func TestOAuthStateRoundTrip(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	state, err := client.NewOAuthState(ctx, 42)
	if err != nil {
		t.Fatalf("завести state: %v", err)
	}
	if state == "" {
		t.Fatal("пустой state")
	}
	if got := client.TakeOAuthState(ctx, 42); got != state {
		t.Fatalf("прочитан %q вместо %q", got, state)
	}
}

func TestOAuthStateIsSingleUse(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	if _, err := client.NewOAuthState(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if first := client.TakeOAuthState(ctx, 1); first == "" {
		t.Fatal("state не прочитался с первого раза")
	}
	// Повторный ответ Google с тем же state не должен проходить.
	if second := client.TakeOAuthState(ctx, 1); second != "" {
		t.Fatalf("state пережил чтение: %q", second)
	}
}

func TestOAuthStatesAreUniqueAndPerUser(t *testing.T) {
	client := newClient(t)
	ctx := context.Background()

	seen := make(map[string]bool, 30)
	for i := range 30 {
		state, err := client.NewOAuthState(ctx, int64(i))
		if err != nil {
			t.Fatal(err)
		}
		if seen[state] {
			t.Fatalf("state повторился: %s", state)
		}
		seen[state] = true
	}
	// Чужой state не подходит: ключ свой у каждого id.
	if got := client.TakeOAuthState(ctx, 100); got != "" {
		t.Fatalf("нашёлся state у пользователя без запроса: %q", got)
	}
}

func TestMissingOAuthStateIsEmpty(t *testing.T) {
	client := newClient(t)
	if got := client.TakeOAuthState(context.Background(), 777); got != "" {
		t.Fatalf("на пустом месте вернулся %q", got)
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
