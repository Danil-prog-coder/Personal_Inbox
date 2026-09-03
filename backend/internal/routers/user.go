package routers

import (
	"net/http"

	"personalinbox/internal/postgres"
)

// Входа в приложение нет: оно локальное, работает на машине владельца и
// обслуживает ровно одного пользователя (решение №50). Раньше здесь жили
// сессии в Redis и cookie с токеном — вместе с формой входа они больше
// не нужны. Redis остался под кэш и под одноразовый state OAuth.

// currentUser — тот единственный, от чьего имени идут все запросы. Строка
// заводится при первом обращении, так что чистая база не требует никаких
// действий от пользователя: открыл приложение и работаешь.
func (s *Server) currentUser(w http.ResponseWriter, _ *http.Request) (*postgres.User, bool) {
	user, err := s.db.LocalUser()
	if err != nil {
		fail(w, http.StatusInternalServerError, "Не удалось получить профиль")
		return nil, false
	}
	return user, true
}
