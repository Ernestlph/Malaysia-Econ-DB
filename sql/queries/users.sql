-- name: CreateUser :one
INSERT INTO users (
    id,
    username,
    email,
    hashed_password,
    created_at
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

