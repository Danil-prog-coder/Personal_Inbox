package telegram

import (
	"fmt"
	"strings"
	"time"

	"github.com/gotd/td/tg"

	"personalinbox/internal/services/ingest"
)

// peer — один собеседник или чат в том виде, в каком он нужен ленте.
type peer struct {
	key   string
	input tg.InputPeerClass
	name  string
	addr  string
	// username — только у публичных: по нему собирается ссылка t.me.
	username string
}

// peerIndex — что удалось узнать из ответа со списком диалогов. Порядок
// сохраняется: Telegram отдаёт диалоги от свежих к старым, и обходить их
// имеет смысл в том же порядке.
type peerIndex struct {
	order []string
	byKey map[string]peer
}

// peerKey — единый ключ пира. Идентификаторы пользователей и чатов живут
// в разных пространствах и могут совпасть, поэтому нужен префикс.
func peerKey(p tg.PeerClass) string {
	switch value := p.(type) {
	case *tg.PeerUser:
		return fmt.Sprintf("u%d", value.UserID)
	case *tg.PeerChat:
		return fmt.Sprintf("c%d", value.ChatID)
	case *tg.PeerChannel:
		return fmt.Sprintf("s%d", value.ChannelID)
	default:
		return ""
	}
}

// collectPeers разбирает ответ messages.getDialogs: имена и точки входа
// лежат в отдельных списках users и chats, а сами диалоги ссылаются на них.
func collectPeers(dialogs tg.MessagesDialogsClass) peerIndex {
	index := peerIndex{byKey: map[string]peer{}}

	var list []tg.DialogClass
	var users []tg.UserClass
	var chats []tg.ChatClass
	switch value := dialogs.(type) {
	case *tg.MessagesDialogs:
		list, users, chats = value.Dialogs, value.Users, value.Chats
	case *tg.MessagesDialogsSlice:
		list, users, chats = value.Dialogs, value.Users, value.Chats
	default:
		return index
	}

	known := map[string]peer{}
	for _, item := range users {
		user, ok := item.(*tg.User)
		if !ok || user.Self {
			continue
		}
		name := strings.TrimSpace(user.FirstName + " " + user.LastName)
		if name == "" {
			name = "Без имени"
		}
		addr := "личный чат"
		if user.Username != "" {
			addr = "@" + user.Username
		}
		key := fmt.Sprintf("u%d", user.ID)
		known[key] = peer{
			key:      key,
			input:    &tg.InputPeerUser{UserID: user.ID, AccessHash: user.AccessHash},
			name:     name,
			addr:     addr,
			username: user.Username,
		}
	}
	for _, item := range chats {
		switch value := item.(type) {
		case *tg.Chat:
			key := fmt.Sprintf("c%d", value.ID)
			known[key] = peer{
				key:   key,
				input: &tg.InputPeerChat{ChatID: value.ID},
				name:  chatName(value.Title),
				addr:  groupAddr(value.ParticipantsCount),
			}
		case *tg.Channel:
			key := fmt.Sprintf("s%d", value.ID)
			addr := "канал"
			if !value.Broadcast {
				addr = groupAddr(value.ParticipantsCount)
			}
			if value.Username != "" {
				addr = "@" + value.Username
			}
			known[key] = peer{
				key:      key,
				input:    &tg.InputPeerChannel{ChannelID: value.ID, AccessHash: value.AccessHash},
				name:     chatName(value.Title),
				addr:     addr,
				username: value.Username,
			}
		}
	}

	for _, item := range list {
		dialog, ok := item.(*tg.Dialog)
		if !ok {
			continue
		}
		key := peerKey(dialog.Peer)
		found, ok := known[key]
		if !ok {
			continue
		}
		if _, already := index.byKey[key]; already {
			continue
		}
		index.order = append(index.order, key)
		index.byKey[key] = found
	}
	return index
}

func chatName(title string) string {
	if strings.TrimSpace(title) == "" {
		return "Групповой чат"
	}
	return title
}

func groupAddr(participants int) string {
	if participants > 0 {
		return fmt.Sprintf("групповой чат, %d участников", participants)
	}
	return "групповой чат"
}

// messagesOf вытаскивает сообщения из любого варианта ответа истории.
func messagesOf(response tg.MessagesMessagesClass) []tg.MessageClass {
	switch value := response.(type) {
	case *tg.MessagesMessages:
		return value.Messages
	case *tg.MessagesMessagesSlice:
		return value.Messages
	case *tg.MessagesChannelMessages:
		return value.Messages
	default:
		return nil
	}
}

// incomingFrom переводит сообщение Telegram в то, что понимает лента.
// Второе значение — false, если сообщение не для ленты: служебное, своё
// собственное или медиа без подписи, которое нечем оценивать.
func incomingFrom(raw tg.MessageClass, source peer) (ingest.Incoming, bool) {
	message, ok := raw.(*tg.Message)
	if !ok {
		return ingest.Incoming{}, false
	}
	if message.Out {
		// Свои же отправленные сообщения в ленте не нужны.
		return ingest.Incoming{}, false
	}
	text := strings.TrimSpace(message.Message)
	if text == "" {
		return ingest.Incoming{}, false
	}

	return ingest.Incoming{
		ExternalID: fmt.Sprintf("%s:%d", source.key, message.ID),
		SenderName: source.name,
		SenderAddr: source.addr,
		// Для Telegram тема — первая строка сообщения (docs/03-data-model.md).
		Subject:     firstLine(text),
		Body:        text,
		ReceivedAt:  time.Unix(int64(message.Date), 0).UTC(),
		ExternalURL: externalURL(source, message.ID),
	}, true
}

// externalURL — прямая ссылка есть только у публичных чатов с username;
// иначе кнопка не показывается (решение №35).
func externalURL(source peer, messageID int) string {
	if source.username == "" {
		return ""
	}
	return fmt.Sprintf("https://t.me/%s/%d", source.username, messageID)
}

// firstLine — тема сообщения: первая строка, не длиннее 200 символов.
func firstLine(text string) string {
	line := strings.TrimSpace(text)
	if index := strings.IndexAny(line, "\r\n"); index >= 0 {
		line = line[:index]
	}
	runes := []rune(line)
	if len(runes) > 200 {
		return string(runes[:200])
	}
	return string(runes)
}
