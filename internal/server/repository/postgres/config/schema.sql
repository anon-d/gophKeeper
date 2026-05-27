CREATE TABLE IF NOT EXISTS users (
    id         UUID PRIMARY KEY,
    username   VARCHAR(255) NOT NULL UNIQUE,
    password   TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS secrets (
    id         UUID PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       SMALLINT NOT NULL,
    title      VARCHAR(255) NOT NULL,
    metadata   TEXT,
    payload    BYTEA,
    ref        TEXT,
    version    INT NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX idx_secrets_user_id ON secrets(user_id);
