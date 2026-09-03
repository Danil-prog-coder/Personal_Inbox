package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"personalinbox/internal/config"
	"personalinbox/internal/store"
)

// session — содержимое подписанной cookie. JWT сознательно не заводим:
// одностраничное приложение и один бэкенд, куки проще и безопаснее
// (docs/03-data-model.md, п. 6).
type session struct {
	UserID     int64  `json:"user_id,omitempty"`
	OAuthState string `json:"oauth_state,omitempty"`
	IssuedAt   int64  `json:"issued_at"`
}

// encodeSession подписывает содержимое ключом из настроек: подмена значения
// делает cookie недействительной, смена ключа разлогинивает всех.
func (s *Server) encodeSession(value session) string {
	value.IssuedAt = time.Now().Unix()
	payload, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	return body + "." + s.sign(body)
}

func (s *Server) sign(body string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.SessionSecret))
	mac.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// decodeSession читает cookie запроса. Битая подпись или просроченная
// cookie — пустая сессия.
func (s *Server) decodeSession(r *http.Request) session {
	cookie, err := r.Cookie(config.SessionCookie)
	if err != nil {
		return session{}
	}
	body, signature, ok := strings.Cut(cookie.Value, ".")
	if !ok || !hmac.Equal([]byte(signature), []byte(s.sign(body))) {
		return session{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return session{}
	}
	var value session
	if err := json.Unmarshal(payload, &value); err != nil {
		return session{}
	}
	if time.Since(time.Unix(value.IssuedAt, 0)) > config.SessionMaxAge {
		return session{}
	}
	return value
}

// setSession кладёт cookie: httpOnly, SameSite=Lax.
func (s *Server) setSession(w http.ResponseWriter, value session) {
	http.SetCookie(w, &http.Cookie{
		Name:     config.SessionCookie,
		Value:    s.encodeSession(value),
		Path:     "/",
		MaxAge:   int(config.SessionMaxAge.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearSession гасит cookie при выходе.
func (s *Server) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     config.SessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// currentUser — пользователь сессии. Сессия есть, а пользователя нет —
// чистим cookie, иначе она будет вечно битой.
func (s *Server) currentUser(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	value := s.decodeSession(r)
	if value.UserID == 0 {
		fail(w, http.StatusUnauthorized, "Требуется вход")
		return nil, false
	}
	user, err := s.db.UserByID(value.UserID)
	if err != nil {
		s.clearSession(w)
		fail(w, http.StatusUnauthorized, "Требуется вход")
		return nil, false
	}
	return user, true
}
