package postgres

import (
	"database/sql"
	"errors"
	"personalinbox/internal/exceptions"
)

const connectionColumns = `id, user_id, kind, state, account, credentials, last_sync_at, sync_cursor`

func scanConnection(row interface{ Scan(...any) error }) (*Connection, error) {
	var connection Connection
	var lastSync sql.NullTime
	var cursor sql.NullString
	if err := row.Scan(&connection.ID, &connection.UserID, &connection.Kind, &connection.State,
		&connection.Account, &connection.Credentials, &lastSync, &cursor); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, exceptions.ErrNotFound
		}
		return nil, err
	}
	connection.LastSyncAt = nullTime(lastSync)
	connection.SyncCursor = stringFromNull(cursor)
	return &connection, nil
}

// ConnectionsOf — все подключения пользователя, по виду источника.
func (db *DB) ConnectionsOf(userID int64) ([]*Connection, error) {
	rows, err := db.Query(
		`SELECT `+connectionColumns+` FROM connection WHERE user_id = ? ORDER BY kind`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Connection
	for rows.Next() {
		connection, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, connection)
	}
	return result, rows.Err()
}

// VisibleConnectionsOf — то, что показывается в ленте уровня 1:
// отключённые источники не показываются вообще (решение №22).
func (db *DB) VisibleConnectionsOf(userID int64) ([]*Connection, error) {
	rows, err := db.Query(
		`SELECT `+connectionColumns+` FROM connection
		 WHERE user_id = ? AND state IN ('active', 'reauth') ORDER BY kind`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Connection
	for rows.Next() {
		connection, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, connection)
	}
	return result, rows.Err()
}

// ActiveConnections — что обходит планировщик.
func (db *DB) ActiveConnections() ([]*Connection, error) {
	rows, err := db.Query(
		`SELECT ` + connectionColumns + ` FROM connection WHERE state = 'active' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Connection
	for rows.Next() {
		connection, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, connection)
	}
	return result, rows.Err()
}

// Connection ищет подключение пользователя к одному сервису.
func (db *DB) Connection(userID int64, kind string) (*Connection, error) {
	return scanConnection(db.QueryRow(
		`SELECT `+connectionColumns+` FROM connection WHERE user_id = ? AND kind = ?`,
		userID, kind))
}

// ConnectionByID нужен приёму сообщений: у сообщения есть только connection_id.
func (db *DB) ConnectionByID(id int64) (*Connection, error) {
	return scanConnection(db.QueryRow(
		`SELECT `+connectionColumns+` FROM connection WHERE id = ?`, id))
}

// GetOrCreateConnection — подключение появляется при первом обращении к сервису.
func (db *DB) GetOrCreateConnection(userID int64, kind string) (*Connection, error) {
	connection, err := db.Connection(userID, kind)
	if err == nil {
		return connection, nil
	}
	if !errors.Is(err, exceptions.ErrNotFound) {
		return nil, err
	}
	var id int64
	if err := db.QueryRow(
		`INSERT INTO connection (user_id, kind, state, account, credentials)
		 VALUES (?, ?, 'off', '', '') RETURNING id`, userID, kind).Scan(&id); err != nil {
		return nil, err
	}
	return &Connection{ID: id, UserID: userID, Kind: kind, State: "off"}, nil
}

// SaveConnection сохраняет состояние, доступы и курсор синхронизации.
func (db *DB) SaveConnection(connection *Connection) error {
	_, err := db.Exec(
		`UPDATE connection
		 SET state = ?, account = ?, credentials = ?, last_sync_at = ?, sync_cursor = ?
		 WHERE id = ?`,
		connection.State, connection.Account, connection.Credentials,
		timePtr(connection.LastSyncAt), nullString(connection.SyncCursor), connection.ID)
	return err
}
