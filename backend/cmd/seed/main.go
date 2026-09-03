// Команда seed заливает демо-ленту из референса: go run ./cmd/seed [--live]
package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"personalinbox/internal/core"
	"personalinbox/internal/events"
	"personalinbox/internal/postgres"
	"personalinbox/internal/services/seed"
	"personalinbox/internal/utils/security"
)

func main() {
	live := flag.Bool("live", false, "доиграть очередь новых сообщений")
	flag.Parse()

	cfg := core.Load()
	db, err := postgres.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("база данных: %v", err)
	}
	defer db.Close()

	hash, err := security.HashPassword(seed.DemoPassword)
	if err != nil {
		log.Fatalf("пароль демо-пользователя: %v", err)
	}
	created, err := seed.Seed(db, hash, time.Time{})
	if err != nil {
		log.Fatalf("демо-данные: %v", err)
	}
	fmt.Printf("Демо-лента: добавлено сообщений — %d\n", created)
	fmt.Printf("Вход: %s / %s\n", seed.DemoEmail, seed.DemoPassword)

	if *live {
		fmt.Println("Очередь живой демонстрации: 3 сообщения")
		seed.PlayLiveQueue(db, events.New(200), hash, time.Second, 3*time.Second, 2600*time.Millisecond)
	}
}
