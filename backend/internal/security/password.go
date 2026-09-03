// Package security — пароли. Сессия лежит в подписанной cookie (internal/api),
// JWT сознательно не заводим (docs/03-data-model.md, п. 6).
package security

import "golang.org/x/crypto/bcrypt"

// bcrypt учитывает только первые 72 байта пароля и на более длинном падает,
// поэтому режем сами — одинаково при регистрации и при проверке.
const bcryptMaxBytes = 72

func prepare(password string) []byte {
	raw := []byte(password)
	if len(raw) > bcryptMaxBytes {
		raw = raw[:bcryptMaxBytes]
	}
	return raw
}

// HashPassword — хеш для хранения в базе.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword(prepare(password), bcrypt.DefaultCost)
	return string(hash), err
}

// VerifyPassword: битый или пустой хеш в базе — это не 500,
// это «пароль не подошёл».
func VerifyPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), prepare(password)) == nil
}
