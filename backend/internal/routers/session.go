package routers

import (
	"net/http"

	"personalinbox/internal/core"
	"personalinbox/internal/postgres"
	"personalinbox/internal/redis"
)

// Сессия живёт в Redis, в cookie лежит только непрозрачный токен. JWT
// сознательно не заводим: одностраничное приложение и один бэкенд, куки проще
// и безопаснее (docs/03-data-model.md, п. 6). Раньше здесь была подписанная
// cookie — она не умела гасить сессию на сервере, выход был только «на честном
// слове» браузера.

// sessionToken достаёт токен из cookie запроса.
func sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(core.SessionCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// session возвращает содержимое сессии запроса и её токен.
func (s *Server) session(r *http.Request) (redis.Session, string) {
	token := sessionToken(r)
	value, ok := s.cache.Session(r.Context(), token)
	if !ok {
		return redis.Session{}, ""
	}
	return value, token
}

// startSession заводит новую сессию и кладёт токен в cookie: httpOnly,
// SameSite=Lax. Ошибка Redis здесь фатальна для запроса — без сессии
// пользователь всё равно не войдёт.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, value redis.Session) bool {
	token, err := s.cache.NewSession(r.Context(), value, core.SessionMaxAge)
	if err != nil {
		fail(w, http.StatusServiceUnavailable, "Хранилище сессий недоступно")
		return false
	}
	setSessionCookie(w, token, int(core.SessionMaxAge.Seconds()))
	return true
}

// saveSession дописывает данные в текущую сессию, не меняя токен.
func (s *Server) saveSession(r *http.Request, token string, value redis.Session) error {
	return s.cache.SaveSession(r.Context(), token, value, core.SessionMaxAge)
}

// clearSession гасит сессию на сервере и удаляет cookie.
func (s *Server) clearSession(w http.ResponseWriter, r *http.Request) {
	if token := sessionToken(r); token != "" {
		_ = s.cache.DropSession(r.Context(), token)
	}
	setSessionCookie(w, "", -1)
}

func setSessionCookie(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     core.SessionCookie,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// currentUser — пользователь сессии. Сессия есть, а пользователя нет —
// гасим сессию, иначе она будет вечно битой.
func (s *Server) currentUser(w http.ResponseWriter, r *http.Request) (*postgres.User, bool) {
	value, _ := s.session(r)
	if value.UserID == 0 {
		fail(w, http.StatusUnauthorized, "Требуется вход")
		return nil, false
	}
	user, err := s.db.UserByID(value.UserID)
	if err != nil {
		s.clearSession(w, r)
		fail(w, http.StatusUnauthorized, "Требуется вход")
		return nil, false
	}
	return user, true
}
