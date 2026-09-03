package routers

import (
	"errors"
	"net/http"
	"personalinbox/internal/exceptions"
	"strings"

	"personalinbox/internal/schemas"
	"personalinbox/internal/utils/security"
)

// PasswordMinLength — минимальная длина пароля при регистрации и смене.
const PasswordMinLength = 8

type credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// validEmail — проверка «на глаз»: одна собака, точка в домене, без пробелов.
// Полноценная валидация адреса на pet-проекте не окупается.
func validEmail(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" || strings.ContainsAny(email, " \t\r\n") {
		return false
	}
	name, domain, ok := strings.Cut(email, "@")
	if !ok || name == "" || domain == "" || strings.Contains(domain, "@") {
		return false
	}
	return strings.Contains(domain, ".") && !strings.HasSuffix(domain, ".")
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var payload credentials
	if !readJSON(w, r, &payload) {
		return
	}
	if !validEmail(payload.Email) {
		fail(w, http.StatusUnprocessableEntity, "Неверный адрес почты")
		return
	}
	if len(payload.Password) < PasswordMinLength {
		fail(w, http.StatusUnprocessableEntity, "Пароль короче 8 символов")
		return
	}
	if _, err := s.db.UserByEmail(payload.Email); err == nil {
		fail(w, http.StatusConflict, "Этот email уже занят")
		return
	} else if !errors.Is(err, exceptions.ErrNotFound) {
		fail(w, http.StatusInternalServerError, "Не удалось создать пользователя")
		return
	}

	hash, err := security.HashPassword(payload.Password)
	if err != nil {
		fail(w, http.StatusInternalServerError, "Не удалось создать пользователя")
		return
	}
	user, err := s.db.CreateUser(payload.Email, hash, "")
	if err != nil {
		fail(w, http.StatusInternalServerError, "Не удалось создать пользователя")
		return
	}
	s.setSession(w, session{UserID: user.ID})
	respond(w, http.StatusCreated, schemas.UserOut(user))
}

// handleLogin: на входе пароль не проверяется по длине — старый короткий
// пароль должен получать «неверный email или пароль», а не 422.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var payload credentials
	if !readJSON(w, r, &payload) {
		return
	}
	user, err := s.db.UserByEmail(payload.Email)
	if err != nil || !security.VerifyPassword(payload.Password, user.PasswordHash) {
		fail(w, http.StatusUnauthorized, "Неверный email или пароль")
		return
	}
	s.setSession(w, session{UserID: user.ID})
	respond(w, http.StatusOK, schemas.UserOut(user))
}

func (s *Server) handleLogout(w http.ResponseWriter, _ *http.Request) {
	s.clearSession(w)
	w.WriteHeader(http.StatusNoContent)
}
