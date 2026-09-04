// Package redis — кэш и одноразовый state OAuth. Кэш снимает повторный
// пересчёт сводки и счётчиков источников, которые фронт дёргает на каждом
// переключении вкладки. Сессий здесь больше нет: входа в приложение нет тоже
// (решение №50).
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

// Open разбирает адрес, подключается и проверяет связь. Redis обязателен:
// на нём держится кэш и подтверждение подключения Gmail, поэтому падаем
// на старте, а не на первом запросе пользователя.
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

// ── Одноразовый state OAuth ─────────────────────────────────────────────

// OAuthStateTTL — сколько ждём возврата от Google. Больше не нужно: дольше
// пользователь по вкладке согласия не ходит, а протухший state безопаснее
// висящего.
const OAuthStateTTL = 10 * time.Minute

func (c *Client) oauthKey(userID int64) string {
	return fmt.Sprintf("%soauth:%d", c.prefix, userID)
}

// NewOAuthState заводит случайный state и запоминает его за пользователем.
// Возвращается сама строка: её кладут в ссылку на согласие Google.
func (c *Client) NewOAuthState(ctx context.Context, userID int64) (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("генерация state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(raw)
	if err := c.rdb.Set(ctx, c.oauthKey(userID), state, OAuthStateTTL).Err(); err != nil {
		return "", fmt.Errorf("запись state: %w", err)
	}
	return state, nil
}

// TakeOAuthState забирает state и сразу удаляет его: он одноразовый, повтор
// того же ответа Google второй раз не пройдёт. Пусто — значит запроса не было
// или он протух.
func (c *Client) TakeOAuthState(ctx context.Context, userID int64) string {
	state, err := c.rdb.GetDel(ctx, c.oauthKey(userID)).Result()
	if err != nil {
		return ""
	}
	return state
}

// ── Незавершённый вход в Telegram ───────────────────────────────────────

// PendingTTL — сколько ждём ввода кода из Telegram. Внутри лежит сессия
// с ключом шифрования, поэтому хранится она ровно до конца входа.
const PendingTTL = 10 * time.Minute

func (c *Client) pendingKey(userID int64) string {
	return fmt.Sprintf("%stg-login:%d", c.prefix, userID)
}

// SavePending запоминает состояние между «отправили код» и «ввели код».
func (c *Client) SavePending(ctx context.Context, userID int64, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, c.pendingKey(userID), payload, PendingTTL).Err()
}

// TakePending забирает состояние и сразу удаляет его: код одноразовый,
// второй попытки с тем же состоянием быть не должно.
func (c *Client) TakePending(ctx context.Context, userID int64, out any) bool {
	payload, err := c.rdb.GetDel(ctx, c.pendingKey(userID)).Bytes()
	if err != nil {
		return false
	}
	return json.Unmarshal(payload, out) == nil
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
