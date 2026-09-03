// Package scheduler — синхронизация источников раз в 5 минут в том же
// процессе: отдельный воркер и брокер для pet-проекта избыточны (решение №2).
package scheduler

import (
	"log"
	"sync"
	"time"

	"personalinbox/internal/core"
	"personalinbox/internal/gmail"
	"personalinbox/internal/services/ingest"
	"personalinbox/internal/sqlite"
	"personalinbox/internal/telegram"
)

// Scheduler обходит активные подключения по таймеру.
type Scheduler struct {
	ingestor *ingest.Ingestor
	gmail    *gmail.Client
	telegram *telegram.Client
	interval time.Duration

	stop     chan struct{}
	stopOnce sync.Once
	started  sync.Once
}

// New собирает планировщик.
func New(ingestor *ingest.Ingestor, gmailClient *gmail.Client, telegramClient *telegram.Client) *Scheduler {
	return &Scheduler{
		ingestor: ingestor,
		gmail:    gmailClient,
		telegram: telegramClient,
		interval: core.SyncInterval,
		stop:     make(chan struct{}),
	}
}

// SyncAll обходит все активные подключения. Ошибка одного не мешает остальным.
func (s *Scheduler) SyncAll() int {
	connections, err := s.ingestor.DB.ActiveConnections()
	if err != nil {
		log.Printf("планировщик: не получить список подключений: %v", err)
		return 0
	}
	saved := 0
	for _, connection := range connections {
		count, err := s.syncOne(connection)
		if err != nil {
			log.Printf("синхронизация %s упала: %v", connection.Kind, err)
			continue
		}
		saved += count
	}
	return saved
}

func (s *Scheduler) syncOne(connection *sqlite.Connection) (int, error) {
	switch connection.Kind {
	case "gmail":
		return s.gmail.Sync(s.ingestor, connection)
	case "telegram":
		return s.telegram.Sync(s.ingestor, connection)
	default:
		return 0, nil
	}
}

// Start запускает периодическую синхронизацию.
func (s *Scheduler) Start() {
	s.started.Do(func() {
		log.Printf("планировщик запущен: синхронизация раз в %s", s.interval)
		go func() {
			ticker := time.NewTicker(s.interval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					s.SyncAll()
				case <-s.stop:
					return
				}
			}
		}()
	})
}

// Stop останавливает синхронизацию.
func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() { close(s.stop) })
}
