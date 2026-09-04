package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/tg"
)

// Вход и синхронизация ходят в настоящий Telegram: код приходит на телефон,
// подменить это нечем. Поэтому тестами закрыто всё, что до сети — разбор
// ответов, состояния и хранение сессии (решение №52).

func TestCollectPeersReadsNamesAndAddresses(t *testing.T) {
	dialogs := &tg.MessagesDialogs{
		Dialogs: []tg.DialogClass{
			&tg.Dialog{Peer: &tg.PeerUser{UserID: 10}},
			&tg.Dialog{Peer: &tg.PeerChat{ChatID: 20}},
			&tg.Dialog{Peer: &tg.PeerChannel{ChannelID: 30}},
		},
		Users: []tg.UserClass{
			&tg.User{ID: 10, FirstName: "Анна", LastName: "Ковалёва", Username: "kovaleva"},
		},
		Chats: []tg.ChatClass{
			&tg.Chat{ID: 20, Title: "Northline — договор", ParticipantsCount: 7},
			&tg.Channel{ID: 30, Title: "Релизы", Broadcast: true, Username: "releases"},
		},
	}

	index := collectPeers(dialogs)
	if len(index.order) != 3 {
		t.Fatalf("собрано пиров: %d", len(index.order))
	}
	// Порядок Telegram сохраняется: свежие диалоги идут первыми.
	if index.order[0] != "u10" || index.order[2] != "s30" {
		t.Fatalf("порядок сбился: %v", index.order)
	}

	user := index.byKey["u10"]
	if user.name != "Анна Ковалёва" || user.addr != "@kovaleva" {
		t.Fatalf("личный чат разобран неверно: %+v", user)
	}
	group := index.byKey["c20"]
	if group.name != "Northline — договор" || group.addr != "групповой чат, 7 участников" {
		t.Fatalf("группа разобрана неверно: %+v", group)
	}
	channel := index.byKey["s30"]
	if channel.addr != "@releases" {
		t.Fatalf("канал разобран неверно: %+v", channel)
	}
}

func TestCollectPeersSkipsSelfAndUnknown(t *testing.T) {
	dialogs := &tg.MessagesDialogsSlice{
		Dialogs: []tg.DialogClass{
			&tg.Dialog{Peer: &tg.PeerUser{UserID: 1}},
			// Диалог есть, а собеседника в ответе нет — пропускаем, иначе
			// в ленте появится сообщение без имени.
			&tg.Dialog{Peer: &tg.PeerUser{UserID: 999}},
		},
		Users: []tg.UserClass{&tg.User{ID: 1, Self: true, FirstName: "Я"}},
	}

	index := collectPeers(dialogs)
	if len(index.order) != 0 {
		t.Fatalf("свой же чат и неизвестный собеседник не должны попадать в обход: %v", index.order)
	}
}

func TestCollectPeersFallsBackOnEmptyNames(t *testing.T) {
	dialogs := &tg.MessagesDialogs{
		Dialogs: []tg.DialogClass{
			&tg.Dialog{Peer: &tg.PeerUser{UserID: 5}},
			&tg.Dialog{Peer: &tg.PeerChat{ChatID: 6}},
		},
		Users: []tg.UserClass{&tg.User{ID: 5}},
		Chats: []tg.ChatClass{&tg.Chat{ID: 6}},
	}

	index := collectPeers(dialogs)
	if got := index.byKey["u5"]; got.name != "Без имени" || got.addr != "личный чат" {
		t.Fatalf("пустое имя собеседника: %+v", got)
	}
	if got := index.byKey["c6"]; got.name != "Групповой чат" || got.addr != "групповой чат" {
		t.Fatalf("пустое название группы: %+v", got)
	}
}

func TestPeerKeysDoNotCollide(t *testing.T) {
	// Один и тот же номер у пользователя, чата и канала — разные сущности.
	same := []tg.PeerClass{
		&tg.PeerUser{UserID: 42},
		&tg.PeerChat{ChatID: 42},
		&tg.PeerChannel{ChannelID: 42},
	}
	seen := map[string]bool{}
	for _, item := range same {
		key := peerKey(item)
		if key == "" || seen[key] {
			t.Fatalf("ключ повторился или пуст: %q", key)
		}
		seen[key] = true
	}
	if peerKey(nil) != "" {
		t.Fatal("неизвестный вид пира должен давать пустой ключ")
	}
}

func TestIncomingFromMessage(t *testing.T) {
	source := peer{key: "u10", name: "Анна", addr: "@kovaleva", username: "kovaleva"}
	moment := time.Date(2026, 9, 2, 9, 41, 0, 0, time.UTC)

	incoming, ok := incomingFrom(&tg.Message{
		ID:      77,
		Date:    int(moment.Unix()),
		Message: "Договор Northline — правки\nвторая строка",
	}, source)
	if !ok {
		t.Fatal("обычное сообщение должно попадать в ленту")
	}
	if incoming.ExternalID != "u10:77" {
		t.Fatalf("внешний id: %q", incoming.ExternalID)
	}
	if incoming.Subject != "Договор Northline — правки" {
		t.Fatalf("тема — первая строка, получено: %q", incoming.Subject)
	}
	if incoming.Body != "Договор Northline — правки\nвторая строка" {
		t.Fatalf("текст изменён: %q", incoming.Body)
	}
	if !incoming.ReceivedAt.Equal(moment) {
		t.Fatalf("время: %s вместо %s", incoming.ReceivedAt, moment)
	}
	if incoming.ExternalURL != "https://t.me/kovaleva/77" {
		t.Fatalf("ссылка: %q", incoming.ExternalURL)
	}
}

func TestIncomingFromSkipsWhatLentaDoesNotNeed(t *testing.T) {
	source := peer{key: "u10", name: "Анна", addr: "личный чат"}
	cases := []struct {
		name string
		raw  tg.MessageClass
	}{
		{"своё отправленное", &tg.Message{ID: 1, Message: "мой ответ", Out: true}},
		{"медиа без подписи", &tg.Message{ID: 2, Message: "   "}},
		{"служебное", &tg.MessageService{ID: 3}},
		{"пустое место", &tg.MessageEmpty{ID: 4}},
	}
	for _, item := range cases {
		if _, ok := incomingFrom(item.raw, source); ok {
			t.Fatalf("%s не должно попадать в ленту", item.name)
		}
	}
}

func TestExternalURLOnlyForPublicChats(t *testing.T) {
	if url := externalURL(peer{key: "c20", name: "Группа"}, 5); url != "" {
		t.Fatalf("у чата без username ссылки быть не может: %q", url)
	}
}

func TestMessagesOfHandlesEveryResponseShape(t *testing.T) {
	one := &tg.Message{ID: 1}
	cases := map[string]tg.MessagesMessagesClass{
		"messages": &tg.MessagesMessages{Messages: []tg.MessageClass{one}},
		"slice":    &tg.MessagesMessagesSlice{Messages: []tg.MessageClass{one}},
		"channel":  &tg.MessagesChannelMessages{Messages: []tg.MessageClass{one}},
	}
	for name, response := range cases {
		if got := messagesOf(response); len(got) != 1 {
			t.Fatalf("%s: получено сообщений %d", name, len(got))
		}
	}
	if got := messagesOf(&tg.MessagesMessagesNotModified{}); got != nil {
		t.Fatalf("«без изменений» — это пусто, получено %d", len(got))
	}
}

func TestFirstLineTrimsToSubject(t *testing.T) {
	long := strings.Repeat("а", 250)
	if got := firstLine(long); len([]rune(got)) != 200 {
		t.Fatalf("длина темы: %d", len([]rune(got)))
	}
	if got := firstLine("  первая\r\nвторая  "); got != "первая" {
		t.Fatalf("тема: %q", got)
	}
}

func TestAccountTitlePrefersUsername(t *testing.T) {
	cases := []struct {
		self *tg.User
		want string
	}{
		{&tg.User{Username: "maxorlov", FirstName: "Максим"}, "@maxorlov"},
		{&tg.User{FirstName: "Максим", LastName: "Орлов"}, "Максим Орлов"},
		{&tg.User{Phone: "79990000000"}, "+79990000000"},
		{&tg.User{}, "аккаунт Telegram"},
		{nil, "аккаунт Telegram"},
	}
	for _, item := range cases {
		if got := accountTitle(item.self); got != item.want {
			t.Fatalf("подпись аккаунта: %q вместо %q", got, item.want)
		}
	}
}

func TestSessionSurvivesRoundTrip(t *testing.T) {
	ctx := context.Background()
	storage := &session.StorageMemory{}
	if err := storage.StoreSession(ctx, []byte(`{"версия": 1}`)); err != nil {
		t.Fatal(err)
	}

	encoded, err := encodeSession(ctx, storage)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := storageFrom(encoded)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := restored.LoadSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"версия": 1}` {
		t.Fatalf("сессия изменилась: %s", raw)
	}
}

func TestBrokenSessionIsRejected(t *testing.T) {
	if _, err := storageFrom("это не base64 !!!"); err == nil {
		t.Fatal("повреждённая сессия должна давать ошибку, а не молча пропадать")
	}
	// Пустая строка — это не поломка, а «сессии ещё нет».
	if _, err := storageFrom(""); err != nil {
		t.Fatalf("пустая сессия: %v", err)
	}
}

func TestSessionRevokedDistinguishesReauthFromGlitch(t *testing.T) {
	revoked := []string{
		"rpc error code 401: AUTH_KEY_UNREGISTERED",
		"SESSION_REVOKED (401)",
		"session_expired",
		"USER_DEACTIVATED",
	}
	for _, text := range revoked {
		if !sessionRevoked(errorText(text)) {
			t.Fatalf("%q — это «войдите заново»", text)
		}
	}
	glitch := []string{"dial tcp: i/o timeout", "FLOOD_WAIT_30", "connection reset by peer"}
	for _, text := range glitch {
		if sessionRevoked(errorText(text)) {
			t.Fatalf("%q — временный сбой, переподключение не требуется", text)
		}
	}
}

type errorText string

func (e errorText) Error() string { return string(e) }

func TestClientWithoutKeysIsNotConfigured(t *testing.T) {
	cases := []struct {
		client Client
		want   bool
	}{
		{Client{}, false},
		{Client{APIID: 1}, false},
		{Client{APIHash: "hash"}, false},
		{Client{APIID: 1, APIHash: "hash"}, true},
	}
	for _, item := range cases {
		if got := item.client.Configured(); got != item.want {
			t.Fatalf("%+v: Configured() = %v", item.client, got)
		}
	}
}
