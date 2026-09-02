"""Регистрация, вход, выход, защита ручек."""


def test_register_creates_user_and_session(client):
    response = client.post(
        "/api/auth/register", json={"email": "new@northline.io", "password": "пароль12345"}
    )
    assert response.status_code == 201
    assert response.json()["email"] == "new@northline.io"
    # Сессия выдана сразу — на фронте после регистрации идёт онбординг критериев.
    assert client.get("/api/me").status_code == 200


def test_register_rejects_duplicate_email(client, user):
    response = client.post(
        "/api/auth/register", json={"email": user.email, "password": "пароль12345"}
    )
    assert response.status_code == 409
    assert response.json()["detail"] == "Этот email уже занят"


def test_register_rejects_short_password(client):
    response = client.post(
        "/api/auth/register", json={"email": "short@northline.io", "password": "1234567"}
    )
    assert response.status_code == 422


def test_register_normalizes_email_case(client):
    client.post("/api/auth/register", json={"email": "MAX@Northline.io", "password": "пароль12345"})
    response = client.post(
        "/api/auth/register", json={"email": "max@northline.io", "password": "пароль12345"}
    )
    assert response.status_code == 409


def test_login_with_wrong_password(client, user):
    response = client.post("/api/auth/login", json={"email": user.email, "password": "неверный"})
    assert response.status_code == 401
    assert response.json()["detail"] == "Неверный email или пароль"


def test_login_with_unknown_email(client):
    response = client.post(
        "/api/auth/login", json={"email": "нет@northline.io", "password": "qwerty12345"}
    )
    assert response.status_code == 401


def test_logout_clears_session(auth_client):
    assert auth_client.post("/api/auth/logout").status_code == 204
    assert auth_client.get("/api/me").status_code == 401


def test_me_requires_auth(client):
    assert client.get("/api/me").status_code == 401
