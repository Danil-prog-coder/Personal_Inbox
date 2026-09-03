package postgres

import (
	"database/sql"
	"errors"
	"personalinbox/internal/exceptions"
)

const messageColumns = `m.id, m.connection_id, m.external_id, m.sender_name, m.sender_addr,
	m.subject, m.body, m.received_at, m.is_read, m.status, m.level, m.level_override,
	m.category, m.deadline_text, m.needs_reply, m.needs_action, m.summary, m.external_url,
	m.analyzed_at, m.analysis_failed, c.kind`

// effectiveLevelSQL — тот же расчёт, что и Message.EffectiveLevel, но для фильтров
// и группировок: ручное исправление важнее оценки модели.
const effectiveLevelSQL = `COALESCE(NULLIF(m.level_override, ''), NULLIF(m.level, ''), 'NORMAL')`

func scanMessage(row interface{ Scan(...any) error }) (*Message, error) {
	var message Message
	var analyzedAt sql.NullTime
	var level, override sql.NullString
	if err := row.Scan(&message.ID, &message.ConnectionID, &message.ExternalID,
		&message.SenderName, &message.SenderAddr, &message.Subject, &message.Body,
		&message.ReceivedAt, &message.IsRead, &message.Status, &level, &override,
		&message.Category, &message.DeadlineText, &message.NeedsReply, &message.NeedsAction,
		&message.Summary, &message.ExternalURL, &analyzedAt, &message.AnalysisFailed,
		&message.Kind); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, exceptions.ErrNotFound
		}
		return nil, err
	}
	message.ReceivedAt = message.ReceivedAt.UTC()
	message.Level = stringFromNull(level)
	message.LevelOverride = stringFromNull(override)
	message.AnalyzedAt = nullTime(analyzedAt)
	return &message, nil
}

func (db *DB) queryMessages(query string, args ...any) ([]*Message, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Message
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, message)
	}
	return result, rows.Err()
}

// InsertMessage сохраняет сообщение и проставляет ему id.
func (db *DB) InsertMessage(message *Message) error {
	return db.QueryRow(
		`INSERT INTO message (connection_id, external_id, sender_name, sender_addr, subject,
			body, received_at, is_read, status, level, level_override, category, deadline_text,
			needs_reply, needs_action, summary, external_url, analyzed_at, analysis_failed)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		message.ConnectionID, message.ExternalID, message.SenderName, message.SenderAddr,
		message.Subject, message.Body, message.ReceivedAt.UTC(), message.IsRead,
		message.Status, nullString(message.Level), nullString(message.LevelOverride),
		message.Category, message.DeadlineText, message.NeedsReply, message.NeedsAction,
		message.Summary, message.ExternalURL, timePtr(message.AnalyzedAt),
		message.AnalysisFailed).Scan(&message.ID)
}

// SaveMessage обновляет всё, что может меняться после приёма: оценку, статус,
// прочитанность и ручное исправление.
func (db *DB) SaveMessage(message *Message) error {
	_, err := db.Exec(
		`UPDATE message SET is_read = ?, status = ?, level = ?, level_override = ?,
			category = ?, deadline_text = ?, needs_reply = ?, needs_action = ?, summary = ?,
			analyzed_at = ?, analysis_failed = ?
		 WHERE id = ?`,
		message.IsRead, message.Status, nullString(message.Level),
		nullString(message.LevelOverride), message.Category, message.DeadlineText,
		message.NeedsReply, message.NeedsAction, message.Summary,
		timePtr(message.AnalyzedAt), message.AnalysisFailed, message.ID)
	return err
}

// MessageByID — детали сообщения вместе с видом источника.
func (db *DB) MessageByID(id int64) (*Message, error) {
	return scanMessage(db.QueryRow(
		`SELECT `+messageColumns+` FROM message m JOIN connection c ON c.id = m.connection_id
		 WHERE m.id = ?`, id))
}

// MessageByExternalID — защита от дублей при повторной синхронизации.
func (db *DB) MessageByExternalID(connectionID int64, externalID string) (*Message, error) {
	return scanMessage(db.QueryRow(
		`SELECT `+messageColumns+` FROM message m JOIN connection c ON c.id = m.connection_id
		 WHERE m.connection_id = ? AND m.external_id = ?`, connectionID, externalID))
}

// MessagesOfConnection — все сообщения источника, новые первыми.
func (db *DB) MessagesOfConnection(connectionID int64) ([]*Message, error) {
	return db.queryMessages(
		`SELECT `+messageColumns+` FROM message m JOIN connection c ON c.id = m.connection_id
		 WHERE m.connection_id = ?
		 ORDER BY m.received_at DESC, m.id DESC`, connectionID)
}

// MessagesOfUser — все сообщения пользователя, новые первыми.
func (db *DB) MessagesOfUser(userID int64) ([]*Message, error) {
	return db.queryMessages(
		`SELECT `+messageColumns+` FROM message m JOIN connection c ON c.id = m.connection_id
		 WHERE c.user_id = ?
		 ORDER BY m.received_at DESC, m.id DESC`, userID)
}

// MarkAllProcessing ставит все сообщения пользователя в очередь на переоценку.
// Возвращает id в том же порядке, в каком они попадут в очередь.
func (db *DB) MarkAllProcessing(userID int64) ([]int64, error) {
	rows, err := db.Query(
		`SELECT m.id FROM message m JOIN connection c ON c.id = m.connection_id
		 WHERE c.user_id = ? ORDER BY m.received_at DESC, m.id DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	if _, err := db.Exec(
		`UPDATE message SET status = 'PROCESSING'
		 WHERE connection_id IN (SELECT id FROM connection WHERE user_id = ?)`, userID); err != nil {
		return nil, err
	}
	return ids, nil
}

// AddOverrideLog пишет ручное исправление уровня — обратную связь для модели.
func (db *DB) AddOverrideLog(messageID int64, from, to string) error {
	_, err := db.Exec(
		`INSERT INTO override_log (message_id, from_level, to_level, created_at)
		 VALUES (?, ?, ?, ?)`,
		messageID, nullString(from), to, UTCNow())
	return err
}

// Override — пара «тема сообщения → уровень, который поставил пользователь».
type Override struct {
	Subject string
	Level   string
}

// RecentOverrides — последние ручные исправления пользователя, новые первыми.
func (db *DB) RecentOverrides(userID int64, limit int) ([]Override, error) {
	rows, err := db.Query(
		`SELECT m.subject, o.to_level
		 FROM override_log o
		 JOIN message m ON m.id = o.message_id
		 JOIN connection c ON c.id = m.connection_id
		 WHERE c.user_id = ?
		 ORDER BY o.created_at DESC, o.id DESC
		 LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Override
	for rows.Next() {
		var item Override
		if err := rows.Scan(&item.Subject, &item.Level); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// CountOverrides нужен тестам и отладке: сколько исправлений накопилось.
func (db *DB) CountOverrides(messageID int64) (int, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM override_log WHERE message_id = ?`, messageID).Scan(&count)
	return count, err
}
