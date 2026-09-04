package postgres

import (
	"database/sql"
	"errors"
	"personalinbox/internal/exceptions"
	"strings"
)

const userColumns = `id, email, password_hash, criteria, theme, density, created_at`

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var user User
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Criteria,
		&user.Theme, &user.Density, &user.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, exceptions.ErrNotFound
		}
		return nil, err
	}
	user.CreatedAt = user.CreatedAt.UTC()
	return &user, nil
}

// CreateUser заводит пользователя. Email хранится в нижнем регистре.
func (db *DB) CreateUser(email, passwordHash, criteria string) (*User, error) {
	user := &User{
		Email:        strings.ToLower(strings.TrimSpace(email)),
		PasswordHash: passwordHash,
		Criteria:     criteria,
		Theme:        "dark",
		Density:      "spacious",
		CreatedAt:    UTCNow(),
	}
	// RETURNING вместо LastInsertId: Postgres его не поддерживает.
	if err := db.QueryRow(
		`INSERT INTO users (email, password_hash, criteria, theme, density, created_at)
		 VALUES (?, ?, ?, ?, ?, ?) RETURNING id`,
		user.Email, user.PasswordHash, user.Criteria, user.Theme, user.Density,
		user.CreatedAt,
	).Scan(&user.ID); err != nil {
		return nil, err
	}
	return user, nil
}

// UserByEmail ищет пользователя без учёта регистра — «Max@…» и «max@…» это один вход.
func (db *DB) UserByEmail(email string) (*User, error) {
	needle := strings.ToLower(strings.TrimSpace(email))
	return scanUser(db.QueryRow(
		`SELECT `+userColumns+` FROM users WHERE lower(email) = ?`, needle))
}

// UserByID — пользователь сессии.
func (db *DB) UserByID(id int64) (*User, error) {
	return scanUser(db.QueryRow(`SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

// SaveUser сохраняет изменяемые поля профиля.
func (db *DB) SaveUser(user *User) error {
	_, err := db.Exec(
		`UPDATE users SET password_hash = ?, criteria = ?, theme = ?, density = ? WHERE id = ?`,
		user.PasswordHash, user.Criteria, user.Theme, user.Density, user.ID)
	return err
}
