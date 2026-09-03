// Package ingest — приём сообщения из источника: дедупликация, событие
// в ленту, очередь на оценку.
package ingest

import (
	"errors"
	"time"

	"personalinbox/internal/analysis"
	"personalinbox/internal/events"
	"personalinbox/internal/store"
	"personalinbox/internal/view"
)

// Incoming — то, что источник знает о сообщении до оценки моделью.
type Incoming struct {
	ExternalID  string
	SenderName  string
	SenderAddr  string
	Subject     string
	Body        string
	ReceivedAt  time.Time
	ExternalURL string
}

// Ingestor — общая для источников точка сохранения сообщений.
type Ingestor struct {
	DB       *store.DB
	Bus      *events.Bus
	Enqueuer analysis.Enqueuer
}

// New собирает приёмник.
func New(db *store.DB, bus *events.Bus, enqueuer analysis.Enqueuer) *Ingestor {
	return &Ingestor{DB: db, Bus: bus, Enqueuer: enqueuer}
}

// Store сохраняет входящее сообщение. Дубль (тот же external_id) — nil без ошибки.
func (i *Ingestor) Store(connection *store.Connection, incoming Incoming) (*store.Message, error) {
	return i.store(connection, incoming, true)
}

// StoreWithoutAnalysis нужен демо-данным: карточка появляется в ленте,
// но модель не вызывается.
func (i *Ingestor) StoreWithoutAnalysis(connection *store.Connection, incoming Incoming) (*store.Message, error) {
	return i.store(connection, incoming, false)
}

func (i *Ingestor) store(connection *store.Connection, incoming Incoming, analyze bool) (*store.Message, error) {
	_, err := i.DB.MessageByExternalID(connection.ID, incoming.ExternalID)
	if err == nil {
		return nil, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	message := &store.Message{
		ConnectionID: connection.ID,
		ExternalID:   incoming.ExternalID,
		SenderName:   incoming.SenderName,
		SenderAddr:   incoming.SenderAddr,
		Subject:      incoming.Subject,
		Body:         incoming.Body,
		ReceivedAt:   incoming.ReceivedAt,
		ExternalURL:  incoming.ExternalURL,
		Status:       "PROCESSING",
		IsRead:       false,
		Kind:         connection.Kind,
	}
	if err := i.DB.InsertMessage(message); err != nil {
		return nil, err
	}

	// Карточка появляется в ленте сразу, с индикатором «Определяем важность…».
	i.Bus.Publish(connection.UserID, "message.created", view.MessageOut(message))
	if analyze && i.Enqueuer != nil {
		i.Enqueuer.Enqueue(message.ID)
	}
	return message, nil
}
