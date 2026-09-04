// Package telegram — источник Telegram через Bot API: проверка токена
// и long polling getUpdates.
//
// Только Bot API — личный аккаунт через MTProto не используем (решение №4).
// Следствие: бот видит только те чаты, куда его добавили, и историю
// до добавления не отдаёт (решение №16).
package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"personalinbox/internal/exceptions"
	"strconv"
	"strings"
	"time"

	"personalinbox/internal/postgres"
	"personalinbox/internal/services/ingest"
)

const defaultBaseURL = "https://api.telegram.org"

// Client — вызовы Bot API. BaseURL подменяется в тестах.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient собирает клиента с разумным таймаутом.
func NewClient() *Client {
	return &Client{BaseURL: defaultBaseURL, HTTP: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) baseURL() string {
	if c.BaseURL == "" {
		return defaultBaseURL
	}
	return c.BaseURL
}

func (c *Client) client() *http.Client {
	if c.HTTP == nil {
		return &http.Client{Timeout: 20 * time.Second}
	}
	return c.HTTP
}

// Call — один вызов метода Bot API.
func (c *Client) Call(token, method string, params map[string]any) (json.RawMessage, error) {
	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", exceptions.ErrTelegram, err)
	}
	url := fmt.Sprintf("%s/bot%s/%s", c.baseURL(), token, method)
	response, err := c.client().Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("%w: недоступен: %v", exceptions.ErrTelegram, err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: недоступен: %v", exceptions.ErrTelegram, err)
	}
	var payload struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("%w: недоступен: %v", exceptions.ErrTelegram, err)
	}
	if !payload.OK {
		description := payload.Description
		if description == "" {
			description = "Telegram вернул ошибку"
		}
		return nil, fmt.Errorf("%w: %s", exceptions.ErrTelegram, description)
	}
	return payload.Result, nil
}

// VerifyToken проверяет токен через getMe. Возвращает @username бота —
// его показываем в UI.
func (c *Client) VerifyToken(token string) (string, error) {
	raw, err := c.Call(token, "getMe", map[string]any{})
	if err != nil {
		return "", err
	}
	var bot struct {
		Username  string `json:"username"`
		FirstName string `json:"first_name"`
	}
	if err := json.Unmarshal(raw, &bot); err != nil {
		return "", fmt.Errorf("%w: ответ getMe не разобрать", exceptions.ErrTelegram)
	}
	if bot.Username != "" {
		return "@" + bot.Username, nil
	}
	if bot.FirstName != "" {
		return bot.FirstName, nil
	}
	return "бот", nil
}

type chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type update struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		MessageID int64  `json:"message_id"`
		Date      int64  `json:"date"`
		Text      string `json:"text"`
		Caption   string `json:"caption"`
		Chat      chat   `json:"chat"`
	} `json:"message"`
}

// chatTitle возвращает (имя отправителя, адрес). Для групп адрес —
// «групповой чат, N участников». cache хранит уже запрошенные счётчики:
// в одном чате обычно приходит пачка сообщений, и спрашивать Telegram
// про каждое незачем.
func (c *Client) chatTitle(token string, source chat, cache map[int64]string) (string, string) {
	if source.Type == "group" || source.Type == "supergroup" || source.Type == "channel" {
		name := source.Title
		if name == "" {
			name = "Групповой чат"
		}
		if addr, ok := cache[source.ID]; ok {
			return name, addr
		}
		addr := "групповой чат"
		if raw, err := c.Call(token, "getChatMemberCount",
			map[string]any{"chat_id": source.ID}); err == nil {
			var count int
			if err := json.Unmarshal(raw, &count); err == nil {
				addr = fmt.Sprintf("групповой чат, %d участников", count)
			}
		}
		cache[source.ID] = addr
		return name, addr
	}

	name := strings.TrimSpace(strings.Join(
		[]string{source.FirstName, source.LastName}, " "))
	if name == "" {
		name = "Без имени"
	}
	if source.Username != "" {
		return name, "@" + source.Username
	}
	return name, "личный чат"
}

// externalURL — прямая ссылка есть только у чатов с username;
// иначе кнопка не показывается.
func externalURL(source chat, messageID int64) string {
	if source.Username == "" {
		return ""
	}
	return fmt.Sprintf("https://t.me/%s/%d", source.Username, messageID)
}

// Sync забирает новые сообщения. Возвращает число сохранённых.
func (c *Client) Sync(ingestor *ingest.Ingestor, connection *postgres.Connection) (int, error) {
	var credentials struct {
		BotToken string `json:"bot_token"`
	}
	if connection.Credentials != "" {
		_ = json.Unmarshal([]byte(connection.Credentials), &credentials)
	}
	if credentials.BotToken == "" {
		connection.State = "reauth"
		return 0, ingestor.DB.SaveConnection(connection)
	}

	params := map[string]any{"timeout": 0, "allowed_updates": []string{"message"}}
	if connection.SyncCursor != "" {
		if cursor, err := strconv.ParseInt(connection.SyncCursor, 10, 64); err == nil {
			params["offset"] = cursor + 1
		}
	}

	raw, err := c.Call(credentials.BotToken, "getUpdates", params)
	if err != nil {
		// Неверный токен — просим переподключить; сетевой сбой — просто ждём
		// следующего цикла.
		log.Printf("синхронизация telegram: %v", err)
		if strings.Contains(strings.ToLower(err.Error()), "unauthorized") {
			connection.State = "reauth"
			return 0, ingestor.DB.SaveConnection(connection)
		}
		return 0, nil
	}

	var updates []update
	if err := json.Unmarshal(raw, &updates); err != nil {
		log.Printf("синхронизация telegram: ответ getUpdates не разобрать: %v", err)
		return 0, nil
	}

	saved := 0
	var lastUpdateID *int64
	memberCounts := map[int64]string{}
	for _, item := range updates {
		id := item.UpdateID
		lastUpdateID = &id
		if item.Message == nil {
			continue
		}
		text := item.Message.Text
		if text == "" {
			text = item.Message.Caption
		}
		if text == "" {
			// Медиа без подписи оценивать нечем — пропускаем.
			continue
		}
		senderName, senderAddr := c.chatTitle(credentials.BotToken, item.Message.Chat, memberCounts)
		message, err := ingestor.Store(connection, ingest.Incoming{
			ExternalID: fmt.Sprintf("%d:%d", item.Message.Chat.ID, item.Message.MessageID),
			SenderName: senderName,
			SenderAddr: senderAddr,
			// Для Telegram тема — первая строка сообщения (docs/03-data-model.md).
			Subject:     firstLine(text),
			Body:        text,
			ReceivedAt:  time.Unix(item.Message.Date, 0).UTC(),
			ExternalURL: externalURL(item.Message.Chat, item.Message.MessageID),
		})
		if err != nil {
			return saved, err
		}
		if message != nil {
			saved++
		}
	}

	if lastUpdateID != nil {
		connection.SyncCursor = strconv.FormatInt(*lastUpdateID, 10)
	}
	now := postgres.UTCNow()
	connection.LastSyncAt = &now
	connection.State = "active"
	return saved, ingestor.DB.SaveConnection(connection)
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
