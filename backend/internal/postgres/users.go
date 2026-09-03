package postgres

import (
	"database/sql"
	"errors"
	"personalinbox/internal/exceptions"
)

const userColumns = `id, criteria, theme, density, created_at`

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var user User
	if err := row.Scan(&user.ID, &user.Criteria,
		&user.Theme, &user.Density, &user.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, exceptions.ErrNotFound
		}
		return nil, err
	}
	user.CreatedAt = user.CreatedAt.UTC()
	return &user, nil
}

// LocalUser — единственный пользователь этой установки. Приложение локальное:
// входа нет, заводить кого-то ещё некому, поэтому строка создаётся при первом
// обращении и дальше переиспользуется (решение №50).
//
// Порядок по id, а не LIMIT 1 наугад: если в базе осталось несколько строк
// от прежней версии со входом, берётся всегда одна и та же — самая ранняя,
// к ней привязаны накопленные сообщения.
func (db *DB) LocalUser() (*User, error) {
	user, err := scanUser(db.QueryRow(
		`SELECT ` + userColumns + ` FROM users ORDER BY id LIMIT 1`))
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, exceptions.ErrNotFound) {
		return nil, err
	}
	return db.CreateUser("")
}

// CreateUser заводит пользователя с настройками по умолчанию.
func (db *DB) CreateUser(criteria string) (*User, error) {
	user := &User{
		Criteria:  criteria,
		Theme:     "dark",
		Density:   "spacious",
		CreatedAt: UTCNow(),
	}
	// RETURNING вместо LastInsertId: Postgres его не поддерживает.
	if err := db.QueryRow(
		`INSERT INTO users (criteria, theme, density, created_at)
		 VALUES (?, ?, ?, ?) RETURNING id`,
		user.Criteria, user.Theme, user.Density, user.CreatedAt,
	).Scan(&user.ID); err != nil {
		return nil, err
	}
	return user, nil
}

// UserByID нужен фоновым задачам: они держат id, а не саму строку.
func (db *DB) UserByID(id int64) (*User, error) {
	return scanUser(db.QueryRow(`SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

// SaveUser сохраняет изменяемые поля профиля.
func (db *DB) SaveUser(user *User) error {
	_, err := db.Exec(
		`UPDATE users SET criteria = ?, theme = ?, density = ? WHERE id = ?`,
		user.Criteria, user.Theme, user.Density, user.ID)
	return err
}
