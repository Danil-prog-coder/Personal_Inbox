package routers

import (
	"net/http"
	"net/url"
	"testing"

	"personalinbox/internal/schemas"
)

func TestRegisterCreatesUserAndSession(t *testing.T) {
	e := newEnv(t)
	status, raw := e.do(http.MethodPost, "/api/auth/register",
		map[string]string{"email": "max@northline.io", "password": testPassword})
	if status != http.StatusCreated {
		t.Fatalf("регистрация вернула %d: %s", status, raw)
	}
	var user schemas.User
	e.decode(raw, &user)
	if user.Email != "max@northline.io" || user.Theme != "dark" || user.Density != "spacious" {
		t.Fatalf("профиль по умолчанию собран неверно: %+v", user)
	}

	// Сессия выдана сразу: отдельный вход не нужен.
	if status, _ := e.do(http.MethodGet, "/api/me", nil); status != http.StatusOK {
		t.Fatalf("после регистрации сессия не работает: %d", status)
	}
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	e := newEnv(t)
	e.user("max@northline.io")
	status, raw := e.do(http.MethodPost, "/api/auth/register",
		map[string]string{"email": "max@northline.io", "password": testPassword})
	if status != http.StatusConflict {
		t.Fatalf("повторная регистрация вернула %d: %s", status, raw)
	}
	if e.detail(raw) != "Этот email уже занят" {
		t.Fatalf("текст ошибки: %q", e.detail(raw))
	}
}

func TestRegisterRejectsShortPassword(t *testing.T) {
	e := newEnv(t)
	status, _ := e.do(http.MethodPost, "/api/auth/register",
		map[string]string{"email": "max@northline.io", "password": "1234"})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("короткий пароль вернул %d", status)
	}
}

func TestRegisterRejectsBrokenEmail(t *testing.T) {
	e := newEnv(t)
	for _, email := range []string{"не почта", "max@", "@northline.io", "max@northline"} {
		status, _ := e.do(http.MethodPost, "/api/auth/register",
			map[string]string{"email": email, "password": testPassword})
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("адрес %q принят со статусом %d", email, status)
		}
	}
}

func TestRegisterNormalizesEmailCase(t *testing.T) {
	e := newEnv(t)
	status, raw := e.do(http.MethodPost, "/api/auth/register",
		map[string]string{"email": "Max@Northline.IO", "password": testPassword})
	if status != http.StatusCreated {
		t.Fatalf("регистрация вернула %d", status)
	}
	var user schemas.User
	e.decode(raw, &user)
	if user.Email != "max@northline.io" {
		t.Fatalf("email не приведён к нижнему регистру: %q", user.Email)
	}
}

func TestLoginWithWrongPassword(t *testing.T) {
	e := newEnv(t)
	e.user("max@northline.io")
	status, raw := e.do(http.MethodPost, "/api/auth/login",
		map[string]string{"email": "max@northline.io", "password": "неверный"})
	if status != http.StatusUnauthorized {
		t.Fatalf("неверный пароль вернул %d", status)
	}
	if e.detail(raw) != "Неверный email или пароль" {
		t.Fatalf("текст ошибки: %q", e.detail(raw))
	}
}

func TestLoginWithUnknownEmail(t *testing.T) {
	e := newEnv(t)
	status, _ := e.do(http.MethodPost, "/api/auth/login",
		map[string]string{"email": "никого@нет.ру", "password": testPassword})
	if status != http.StatusUnauthorized {
		t.Fatalf("неизвестный email вернул %d", status)
	}
}

func TestShortPasswordAtLoginIsNotValidationError(t *testing.T) {
	e := newEnv(t)
	e.user("max@northline.io")
	// Старый короткий пароль должен получать 401, а не 422.
	status, _ := e.do(http.MethodPost, "/api/auth/login",
		map[string]string{"email": "max@northline.io", "password": "123"})
	if status != http.StatusUnauthorized {
		t.Fatalf("вход с коротким паролем вернул %d", status)
	}
}

func TestLogoutClearsSession(t *testing.T) {
	e := newEnv(t)
	e.authorized()
	if status, _ := e.do(http.MethodPost, "/api/auth/logout", nil); status != http.StatusNoContent {
		t.Fatalf("выход вернул %d", status)
	}
	if status, _ := e.do(http.MethodGet, "/api/me", nil); status != http.StatusUnauthorized {
		t.Fatalf("после выхода сессия ещё жива: %d", status)
	}
}

func TestMeRequiresAuth(t *testing.T) {
	e := newEnv(t)
	status, raw := e.do(http.MethodGet, "/api/me", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("без входа ожидался 401, получен %d", status)
	}
	if e.detail(raw) != "Требуется вход" {
		t.Fatalf("текст ошибки: %q", e.detail(raw))
	}
}

func TestForgedCookieIsRejected(t *testing.T) {
	e := newEnv(t)
	e.authorized()
	// Сессия живёт на сервере, поэтому придуманный токен не открывает ничего:
	// подобрать его нельзя, а вывести из чужого — тем более.
	for _, forged := range []string{
		"выдуманный-токен",
		"",
		"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	} {
		request, err := http.NewRequest(http.MethodGet, e.server.URL+"/api/me", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.AddCookie(&http.Cookie{Name: "pi_session", Value: forged})
		response, err := (&http.Client{}).Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("чужой токен %q принят: %d", forged, response.StatusCode)
		}
	}
}

// TestLogoutKillsSessionOnServer: старая подписанная cookie гасла только
// в браузере. Теперь выход убивает сессию на сервере — украденная cookie
// после выхода бесполезна.
func TestLogoutKillsSessionOnServer(t *testing.T) {
	e := newEnv(t)
	e.authorized()

	var stolen string
	for _, cookie := range e.client.Jar.Cookies(mustURL(t, e.server.URL)) {
		if cookie.Name == "pi_session" {
			stolen = cookie.Value
		}
	}
	if stolen == "" {
		t.Fatal("сессионная cookie не выдана")
	}
	if status, _ := e.do(http.MethodPost, "/api/auth/logout", nil); status != http.StatusNoContent {
		t.Fatalf("выход вернул %d", status)
	}

	request, err := http.NewRequest(http.MethodGet, e.server.URL+"/api/me", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.AddCookie(&http.Cookie{Name: "pi_session", Value: stolen})
	response, err := (&http.Client{}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("cookie работает после выхода: %d", response.StatusCode)
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestHealthIsPublic(t *testing.T) {
	e := newEnv(t)
	status, raw := e.do(http.MethodGet, "/api/health", nil)
	if status != http.StatusOK {
		t.Fatalf("проверка здоровья вернула %d: %s", status, raw)
	}
}
