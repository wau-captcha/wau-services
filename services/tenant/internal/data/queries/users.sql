-- name: CreateUser :one
INSERT INTO users (username, email, password)
VALUES ($1, $2, $3)
RETURNING user_id;

-- name: GetUserCredentialsByUsername :one
SELECT user_id, password
FROM users
WHERE username = $1;

-- name: GetUserCredentialsByEmail :one
SELECT user_id, password
FROM users
WHERE email = $1;

-- name: GetUserProfile :one
SELECT u.user_id, u.username, u.email, u.create_date, u.tier_id, t.tier_name
FROM users u
JOIN tiers t ON t.tier_id = u.tier_id
WHERE u.user_id = $1;
