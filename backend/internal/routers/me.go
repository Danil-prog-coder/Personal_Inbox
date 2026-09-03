package routers

import (
	"net/http"

	"personalinbox/internal/postgres"
	"personalinbox/internal/schemas"
	"personalinbox/internal/services/analysis"
	"personalinbox/internal/utils/security"
)

type meUpdate struct {
	Criteria        *string `json:"criteria"`
	Theme           *string `json:"theme"`
	Density         *string `json:"density"`
	CurrentPassword *string `json:"current_password"`
	NewPassword     *string `json:"new_password"`
}

func (s *Server) handleGetMe(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	respond(w, http.StatusOK, schemas.UserOut(user))
}

func (s *Server) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(w, r)
	if !ok {
		return
	}
	var payload meUpdate
	if !readJSON(w, r, &payload) {
		return
	}

	if payload.Theme != nil && !postgres.Contains(postgres.Themes, *payload.Theme) {
		fail(w, http.StatusUnprocessableEntity, "Неизвестная тема")
		return
	}
	if payload.Density != nil && !postgres.Contains(postgres.Densities, *payload.Density) {
		fail(w, http.StatusUnprocessableEntity, "Неизвестная плотность")
		return
	}
	if payload.NewPassword != nil && len(*payload.NewPassword) < PasswordMinLength {
		fail(w, http.StatusUnprocessableEntity, "Пароль короче 8 символов")
		return
	}

	if payload.NewPassword != nil {
		if payload.CurrentPassword == nil ||
			!security.VerifyPassword(*payload.CurrentPassword, user.PasswordHash) {
			fail(w, http.StatusBadRequest, "Текущий пароль неверен")
			return
		}
		hash, err := security.HashPassword(*payload.NewPassword)
		if err != nil {
			fail(w, http.StatusInternalServerError, "Не удалось сменить пароль")
			return
		}
		user.PasswordHash = hash
	}
	if payload.Theme != nil {
		user.Theme = *payload.Theme
	}
	if payload.Density != nil {
		user.Density = *payload.Density
	}

	// Сохранение новых критериев ставит все сообщения на переоценку.
	criteriaChanged := payload.Criteria != nil && *payload.Criteria != user.Criteria
	if payload.Criteria != nil {
		user.Criteria = *payload.Criteria
	}

	if err := s.db.SaveUser(user); err != nil {
		fail(w, http.StatusInternalServerError, "Не удалось сохранить профиль")
		return
	}

	queued := 0
	if criteriaChanged {
		var err error
		queued, err = analysis.QueueReanalysis(s.db, s.enqueuer, user.ID)
		if err != nil {
			fail(w, http.StatusInternalServerError, "Не удалось поставить переоценку")
			return
		}
		// Все сообщения ушли в PROCESSING — фильтр «Обработано» и сводка
		// в кэше больше не соответствуют базе.
		s.cache.DropCache(r.Context(), user.ID)
	}
	respond(w, http.StatusOK, schemas.MeUpdateResult{
		User:            schemas.UserOut(user),
		ReanalyzeQueued: queued,
	})
}
