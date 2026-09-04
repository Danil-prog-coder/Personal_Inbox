// Package redis — сессии и кэш. Сессия живёт на сервере, а не в подписанной
// cookie: только так выход по-настоящему гасит сессию, а не полагается на то,
// что браузер выбросит куку. Кэш снимает повторный пересчёт сводки и счётчиков
// источников, которые фронт дёргает на каждом переключении вкладки.
package redis

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client — тонкая обёртка над go-redis: наружу торчат только те операции,
// которые нужны приложению.
type Client struct {
	rdb *redis.Client
	// prefix — пространство имён ключей. Пустой в приложении; тесты задают
	// свой, чтобы параллельные прогоны не видели чужие ключи в общем Redis.
	prefix string
}

// Session — то, что сервер помнит о вошедшем пользователе.
type Session struct {
	UserID     int64  `json:"user_id"`
	OAuthState string `json:"oauth_state,omitempty"`
}

// Open разбирает адрес, подключается и проверяет связь. Redis обязателен:
// без него нельзя ни войти, ни удержать сессию, поэтому падаем на старте,
// а не на первом запросе пользователя.
func Open(url string) (*Client, error) {
	return OpenWithPrefix(url, "")
}

// OpenWithPrefix — то же, но все ключи получают общий префикс.
func OpenWithPrefix(url, prefix string) (*Client, error) {
	options, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("адрес redis: %w", err)
	}
	rdb := redis.NewClient(options)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, fmt.Errorf("redis недоступен: %w", err)
	}
	return &Client{rdb: rdb, prefix: prefix}, nil
}

// Close закрывает пул подключений.
func (c *Client) Close() error {
	return c.rdb.Close()
}

// ── Сессии ──────────────────────────────────────────────────────────────

func (c *Client) sessionKey(token string) string {
	return c.prefix + "session:" + token
}

// NewSession заводит сессию и возвращает непрозрачный токен для cookie.
// Токен случайный: по нему нельзя ничего восстановить, вся полезная нагрузка
// лежит на сервере.
func (c *Client) NewSession(ctx context.Context, value Session, ttl time.Duration) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("генерация токена: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	if err := c.rdb.Set(ctx, c.sessionKey(token), payload, ttl).Err(); err != nil {
		return "", fmt.Errorf("запись сессии: %w", err)
	}
	return token, nil
}

// Session читает сессию по токену. Второе значение — false, если сессии нет
// или она истекла: для вызывающего это одно и то же «входа нет».
func (c *Client) Session(ctx context.Context, token string) (Session, bool) {
	if token == "" {
		return Session{}, false
	}
	payload, err := c.rdb.Get(ctx, c.sessionKey(token)).Bytes()
	if err != nil {
		return Session{}, false
	}
	var value Session
	if err := json.Unmarshal(payload, &value); err != nil {
		return Session{}, false
	}
	return value, true
}

// SaveSession перезаписывает сессию, не меняя токен: нужно, чтобы положить
// oauth_state в уже существующую сессию.
func (c *Client) SaveSession(ctx context.Context, token string, value Session, ttl time.Duration) error {
	if token == "" {
		return fmt.Errorf("пустой токен сессии")
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, c.sessionKey(token), payload, ttl).Err()
}

// DropSession гасит сессию при выходе — здесь и заканчивается вход,
// а не в момент, когда браузер решит удалить cookie.
func (c *Client) DropSession(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return c.rdb.Del(ctx, c.sessionKey(token)).Err()
}

// ── Кэш ─────────────────────────────────────────────────────────────────

// CacheTTL — короткий срок жизни кэша. Даже если инвалидация где-то не
// сработает, расхождение живёт меньше минуты.
const CacheTTL = 45 * time.Second

func (c *Client) cacheKey(userID int64, name string) string {
	return fmt.Sprintf("%scache:%d:%s", c.prefix, userID, name)
}

// GetCached достаёт значение и разбирает его в out. false — промах кэша,
// недоступный Redis или мусор в значении: во всех случаях считаем заново.
func (c *Client) GetCached(ctx context.Context, userID int64, name string, out any) bool {
	payload, err := c.rdb.Get(ctx, c.cacheKey(userID, name)).Bytes()
	if err != nil {
		return false
	}
	return json.Unmarshal(payload, out) == nil
}

// SetCached кладёт значение в кэш. Ошибка кэша не должна ломать ответ
// пользователю, поэтому она только пишется в лог.
func (c *Client) SetCached(ctx context.Context, userID int64, name string, value any) {
	payload, err := json.Marshal(value)
	if err != nil {
		log.Printf("кэш %s: %v", name, err)
		return
	}
	if err := c.rdb.Set(ctx, c.cacheKey(userID, name), payload, CacheTTL).Err(); err != nil {
		log.Printf("кэш %s: %v", name, err)
	}
}

// DropCache сбрасывает весь кэш пользователя. Вызывается на любой записи,
// которая меняет ленту: пришло сообщение, поменялся уровень, прочитали письмо.
// Ключей на пользователя единицы, поэтому SCAN здесь дешевле, чем вести
// отдельный список.
func (c *Client) DropCache(ctx context.Context, userID int64) {
	pattern := c.cacheKey(userID, "*")
	iterator := c.rdb.Scan(ctx, 0, pattern, 64).Iterator()
	var keys []string
	for iterator.Next(ctx) {
		keys = append(keys, iterator.Val())
	}
	if err := iterator.Err(); err != nil {
		log.Printf("сброс кэша: %v", err)
		return
	}
	if len(keys) == 0 {
		return
	}
	if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
		log.Printf("сброс кэша: %v", err)
	}
}

// DropPrefix стирает все ключи своего пространства имён. Нужен тестам, чтобы
// прогон не оставлял мусор в общем Redis. Без префикса не делает ничего:
// иначе метод стирал бы всю базу приложения.
func (c *Client) DropPrefix(ctx context.Context) {
	if c.prefix == "" {
		return
	}
	iterator := c.rdb.Scan(ctx, 0, c.prefix+"*", 128).Iterator()
	var keys []string
	for iterator.Next(ctx) {
		keys = append(keys, iterator.Val())
	}
	if err := iterator.Err(); err != nil || len(keys) == 0 {
		return
	}
	_ = c.rdb.Del(ctx, keys...).Err()
}
