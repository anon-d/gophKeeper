-- Создание пользователя
-- name: CreateUser :exec
INSERT INTO users (id, username, password, created_at)
VALUES ($1, $2, $3, now());

-- Получение пользователя по username (для логина — проверка пароля)
-- name: GetUserByUsername :one
SELECT id, username, password, created_at
FROM users
WHERE username = $1;

-- Создание секрета
-- name: CreateSecret :exec
INSERT INTO secrets (id, user_id, type, title, metadata, payload, ref, version, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, 1, now(), now());

-- Получение секрета по ID (с проверкой владельца)
-- name: GetSecretById :one
SELECT id, type, title, metadata, payload, ref, version, created_at, updated_at
FROM secrets
WHERE id = $1 AND user_id = $2;

-- Получение списка секретов пользователя (без payload — для ListSecrets)
-- name: ListSecrets :many
SELECT id, type, title, version, created_at, updated_at
FROM secrets
WHERE user_id = $1;

-- Удаление секрета (с проверкой владельца)
-- name: DeleteSecret :exec
DELETE FROM secrets
WHERE id = $1 AND user_id = $2;
