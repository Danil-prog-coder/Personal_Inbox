// Команда server — точка входа приложения: go run ./cmd/server
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"personalinbox/internal/core"
	"personalinbox/internal/events"
	"personalinbox/internal/gmail"
	"personalinbox/internal/openai"
	"personalinbox/internal/postgres"
	"personalinbox/internal/redis"
	"personalinbox/internal/routers"
	"personalinbox/internal/services/analysis"
	"personalinbox/internal/services/ingest"
	"personalinbox/internal/services/scheduler"
	"personalinbox/internal/services/seed"
	"personalinbox/internal/telegram"
)

func main() {
	log.SetFlags(log.LstdFlags)
	cfg := core.Load()

	db, err := postgres.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("база данных: %v", err)
	}
	defer db.Close()

	// Redis обязателен: в нём живут сессии, без него нельзя войти.
	cache, err := redis.Open(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer cache.Close()

	bus := events.New(200)
	// Любое изменение ленты проходит через шину — на нём и сбрасываем кэш.
	bus.OnPublish(func(userID int64) { cache.DropCache(context.Background(), userID) })
	worker := analysis.NewWorker(db, bus, openai.NewOpenAI(cfg))
	ingestor := ingest.New(db, bus, worker)
	gmailClient := gmail.NewClient(cfg)
	telegramClient := telegram.NewClient()

	var sync *scheduler.Scheduler
	if cfg.EnableScheduler {
		worker.Start()
		sync = scheduler.New(ingestor, gmailClient, telegramClient)
		sync.Start()
	}
	if cfg.DemoLive {
		// Доигрывает три «новых» сообщения из референса: на них видно
		// появление карточек в реальном времени через SSE.
		go seed.PlayLiveQueue(db, bus, 6*time.Second, 16*time.Second, 2600*time.Millisecond)
	}

	server := &http.Server{
		Addr:    cfg.Addr,
		Handler: routers.New(cfg, db, cache, bus, ingestor, worker, gmailClient, telegramClient).Handler(),
		// Поток SSE живёт долго, поэтому таймаута на запись нет намеренно.
		ReadHeaderTimeout: 10 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		log.Printf("Personal Inbox слушает %s", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("сервер: %v", err)
		}
	}()

	<-stop
	log.Print("останавливаемся")
	if sync != nil {
		sync.Stop()
	}
	worker.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}
