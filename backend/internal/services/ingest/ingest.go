// Package ingest — приём сообщения из источника: дедупликация, событие
// в ленту, очередь на оценку.
package ingest

import (
	"errors"
	"personalinbox/internal/exceptions"
	"time"

	"personalinbox/internal/events"
	"personalinbox/internal/postgres"
	"personalinbox/internal/schemas"
	"personalinbox/internal/services/analysis"
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
	DB       *postgres.DB
	Bus      *events.Bus
	Enqueuer analysis.Enqueuer
}

// New собирает приёмник.
func New(db *postgres.DB, bus *events.Bus, enqueuer analysis.Enqueuer) *Ingestor {
	return &Ingestor{DB: db, Bus: bus, Enqueuer: enqueuer}
}

// Store сохраняет входящее сообщение. Дубль (тот же external_id) — nil без ошибки.
func (i *Ingestor) Store(connection *postgres.Connection, incoming Incoming) (*postgres.Message, error) {
	return i.store(connection, incoming, true)
}

// StoreWithoutAnalysis нужен демо-данным: карточка появляется в ленте,
// но модель не вызывается.
func (i *Ingestor) StoreWithoutAnalysis(connection *postgres.Connection, incoming Incoming) (*postgres.Message, error) {
	return i.store(connection, incoming, false)
}

func (i *Ingestor) store(connection *postgres.Connection, incoming Incoming, analyze bool) (*postgres.Message, error) {
	_, err := i.DB.MessageByExternalID(connection.ID, incoming.ExternalID)
	if err == nil {
		return nil, nil
	}
	if !errors.Is(err, exceptions.ErrNotFound) {
		return nil, err
	}

	message := &postgres.Message{
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
	i.Bus.Publish(connection.UserID, "message.created", schemas.MessageOut(message))
	if analyze && i.Enqueuer != nil {
		i.Enqueuer.Enqueue(message.ID)
	}
	return message, nil
}
