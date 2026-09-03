// Package testenv поднимает стенд для тестов: своя схема в Postgres и своё
// пространство ключей в Redis на каждый тест. Изоляция нужна, чтобы тесты
// разных пакетов не видели чужие строки и ключи, когда go test гоняет их
// параллельно.
//
// Адреса берутся из TEST_DATABASE_URL и TEST_REDIS_URL, затем из DATABASE_URL
// и REDIS_URL, затем из значений по умолчанию — тех же, что у приложения.
package testenv

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"personalinbox/internal/postgres"
	"personalinbox/internal/redis"
)

const (
	defaultDatabaseURL = "postgres://personalinbox:personalinbox@localhost:5432/personalinbox?sslmode=disable"
	defaultRedisURL    = "redis://localhost:6379/0"
)

func env(names []string, fallback string) string {
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return fallback
}

// token — короткий случайный суффикс для имени схемы и префикса ключей.
func token(t *testing.T) string {
	t.Helper()
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("случайное имя: %v", err)
	}
	return hex.EncodeToString(raw)
}

// DB — чистая база на тест: своя схема, накатанные миграции, снос в конце.
// Если Postgres недоступен, тест падает, а не пропускается: молча пропущенный
// тест хуже упавшего, потому что его никто не заметит.
func DB(t *testing.T) *postgres.DB {
	t.Helper()
	dsn := env([]string{"TEST_DATABASE_URL", "DATABASE_URL"}, defaultDatabaseURL)
	db, drop, err := postgres.OpenSchema(dsn, "test_"+token(t))
	if err != nil {
		t.Fatalf("postgres недоступен (%s): %v", dsn, err)
	}
	t.Cleanup(drop)
	return db
}

// Cache — Redis со своим префиксом ключей на тест.
func Cache(t *testing.T) *redis.Client {
	t.Helper()
	url := env([]string{"TEST_REDIS_URL", "REDIS_URL"}, defaultRedisURL)
	client, err := redis.OpenWithPrefix(url, "test:"+token(t)+":")
	if err != nil {
		t.Fatalf("redis недоступен (%s): %v", url, err)
	}
	t.Cleanup(func() {
		client.DropPrefix(context.Background())
		client.Close()
	})
	return client
}
