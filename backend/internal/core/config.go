// Package config — настройки приложения. Читаются из переменных окружения,
// всё имеет разумный дефолт: pet-проект должен запускаться без единой настройки.
package core

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	LLMMaxBodyChars    = 4000 // обрезка текста сообщения перед отправкой в модель
	LLMOverrideHistory = 20   // сколько ручных исправлений уходит в промпт

	GmailImportDays  = 30 // глубина первого импорта
	GmailImportLimit = 200
	SyncInterval     = 5 * time.Minute // частота синхронизации источников
)

// Адреса по умолчанию — для запуска без контейнеров: docker compose передаёт
// свои через окружение.
const (
	defaultDatabaseURL = "postgres://personalinbox:personalinbox@localhost:5432/personalinbox?sslmode=disable"
	defaultRedisURL    = "redis://localhost:6379/0"
)

// LLMRetryDelays — три повтора: 2с / 8с / 30с (docs/00-product-spec.md, п. 6.1).
var LLMRetryDelays = []time.Duration{2 * time.Second, 8 * time.Second, 30 * time.Second}

// Config — всё, что читается из окружения. Собирается один раз при старте.
type Config struct {
	DatabaseURL string
	RedisURL    string

	OpenAIKey   string
	OpenAIModel string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURI  string
	AppBaseURL         string

	// Ключи клиентского API Telegram с my.telegram.org. Личную переписку
	// отдаёт только он: бот её не видит в принципе (решение №52).
	TelegramAPIID   int
	TelegramAPIHash string

	FrontendOrigin  string
	Addr            string
	EnableScheduler bool
	DemoLive        bool
}

// Load собирает настройки: сначала .env в корне репозитория, затем окружение
// (переменная окружения сильнее строки в файле).
func Load() Config {
	loadDotEnv(dotEnvPath())

	appBase := env("APP_BASE_URL", "http://localhost:8000")
	c := Config{
		DatabaseURL:        env("DATABASE_URL", defaultDatabaseURL),
		RedisURL:           env("REDIS_URL", defaultRedisURL),
		OpenAIKey:          env("OPENAI_API_KEY", ""),
		OpenAIModel:        env("OPENAI_MODEL", "gpt-4o-mini"),
		GoogleClientID:     env("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: env("GOOGLE_CLIENT_SECRET", ""),
		AppBaseURL:         appBase,
		GoogleRedirectURI:  env("GOOGLE_REDIRECT_URI", appBase+"/api/connections/gmail/callback"),
		TelegramAPIID:      envInt("TELEGRAM_API_ID", 0),
		TelegramAPIHash:    env("TELEGRAM_API_HASH", ""),
		FrontendOrigin:     env("FRONTEND_ORIGIN", "http://localhost:5173"),
		Addr:               env("ADDR", ":8000"),
		EnableScheduler:    flag("ENABLE_SCHEDULER", true),
		// Проигрывает очередь «новых» сообщений из референса при старте сервера —
		// нужно, чтобы посмотреть появление карточек без подключённых источников.
		DemoLive: flag("DEMO_LIVE", false),
	}
	return c
}

// envInt читает число; пустое или мусорное значение — как не заданное.
func envInt(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil {
		return fallback
	}
	return value
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func flag(name string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(name)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func dotEnvPath() string {
	if dir := repoRoot(); dir != "" {
		return filepath.Join(dir, ".env")
	}
	return ".env"
}

// repoRoot ищет корень репозитория вверх от рабочего каталога: сервер одинаково
// запускается и из корня, и из backend/, и из каталога пакета в тестах.
func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// loadDotEnv — минимальный разбор .env: KEY=VALUE, строки с # пропускаются.
// Значения из окружения не перетираются.
func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
}
