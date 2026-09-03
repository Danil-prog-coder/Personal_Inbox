-- Начальная схема: пользователь, подключения, сообщения, журнал исправлений.
-- Таблица пользователей называется users: user — зарезервированное слово Postgres.
CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    email         VARCHAR(320) NOT NULL UNIQUE,
    password_hash VARCHAR(128) NOT NULL,
    criteria      TEXT NOT NULL DEFAULT '',
    theme         VARCHAR(8) NOT NULL DEFAULT 'dark',
    density       VARCHAR(16) NOT NULL DEFAULT 'spacious',
    created_at    TIMESTAMPTZ NOT NULL
);

CREATE TABLE connection (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind         VARCHAR(16) NOT NULL,
    state        VARCHAR(16) NOT NULL DEFAULT 'off',
    account      VARCHAR(320) NOT NULL DEFAULT '',
    credentials  TEXT NOT NULL DEFAULT '',
    last_sync_at TIMESTAMPTZ,
    sync_cursor  VARCHAR(64),
    CONSTRAINT uq_connection_user_kind UNIQUE (user_id, kind)
);

CREATE TABLE message (
    id              BIGSERIAL PRIMARY KEY,
    connection_id   BIGINT NOT NULL REFERENCES connection(id) ON DELETE CASCADE,
    external_id     VARCHAR(255) NOT NULL,
    sender_name     VARCHAR(255) NOT NULL DEFAULT '',
    sender_addr     VARCHAR(255) NOT NULL DEFAULT '',
    subject         TEXT NOT NULL DEFAULT '',
    body            TEXT NOT NULL DEFAULT '',
    received_at     TIMESTAMPTZ NOT NULL,
    is_read         BOOLEAN NOT NULL DEFAULT false,
    status          VARCHAR(16) NOT NULL DEFAULT 'PROCESSING',
    level           VARCHAR(16),
    level_override  VARCHAR(16),
    category        VARCHAR(255) NOT NULL DEFAULT '',
    deadline_text   VARCHAR(255) NOT NULL DEFAULT '',
    needs_reply     BOOLEAN NOT NULL DEFAULT false,
    needs_action    BOOLEAN NOT NULL DEFAULT false,
    summary         TEXT NOT NULL DEFAULT '',
    external_url    TEXT NOT NULL DEFAULT '',
    analyzed_at     TIMESTAMPTZ,
    analysis_failed BOOLEAN NOT NULL DEFAULT false,
    CONSTRAINT uq_message_conn_external UNIQUE (connection_id, external_id)
);

CREATE INDEX ix_message_conn_received ON message (connection_id, received_at);
CREATE INDEX ix_message_conn_read ON message (connection_id, is_read);

CREATE TABLE override_log (
    id         BIGSERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL REFERENCES message(id) ON DELETE CASCADE,
    from_level VARCHAR(16),
    to_level   VARCHAR(16) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
