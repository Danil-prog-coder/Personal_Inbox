package sqlite

import (
	"database/sql"
	"errors"
	"personalinbox/internal/exceptions"
	"strings"
)

const userColumns = `id, email, password_hash, criteria, theme, density, created_at`

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var user User
	var createdAt dbTime
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Criteria,
		&user.Theme, &user.Density, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, exceptions.ErrNotFound
		}
		return nil, err
	}
	user.CreatedAt = createdAt.Time
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
	result, err := db.Exec(
		`INSERT INTO user (email, password_hash, criteria, theme, density, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		user.Email, user.PasswordHash, user.Criteria, user.Theme, user.Density,
		ToDBTime(user.CreatedAt),
	)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	user.ID = id
	return user, nil
}

// UserByEmail ищет пользователя без учёта регистра — «Max@…» и «max@…» это один вход.
func (db *DB) UserByEmail(email string) (*User, error) {
	needle := strings.ToLower(strings.TrimSpace(email))
	return scanUser(db.QueryRow(
		`SELECT `+userColumns+` FROM user WHERE unilower(email) = ?`, needle))
}

// UserByID — пользователь сессии.
func (db *DB) UserByID(id int64) (*User, error) {
	return scanUser(db.QueryRow(`SELECT `+userColumns+` FROM user WHERE id = ?`, id))
}

// SaveUser сохраняет изменяемые поля профиля.
func (db *DB) SaveUser(user *User) error {
	_, err := db.Exec(
		`UPDATE user SET password_hash = ?, criteria = ?, theme = ?, density = ? WHERE id = ?`,
		user.PasswordHash, user.Criteria, user.Theme, user.Density, user.ID)
	return err
}
