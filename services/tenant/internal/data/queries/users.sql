-- name: FindUserByGoogleSub :one
SELECT user_id
FROM users
WHERE google_sub = $1;

-- Claims an account created before Google sign-in; accounts already linked are left alone.
-- name: LinkGoogleAccount :one
UPDATE users
SET google_sub = @google_sub,
    username = @username
WHERE email = @email
  AND google_sub IS NULL
RETURNING user_id;

-- name: CreateGoogleUser :one
INSERT INTO users (google_sub, email, username)
VALUES (@google_sub, @email, @username)
RETURNING user_id;

-- Empty values keep what is stored; the email only changes when no other account uses it.
-- name: UpdateGoogleProfile :exec
UPDATE users AS u
SET username = COALESCE(NULLIF(@username::text, ''), u.username),
    email = CASE
        WHEN @email::text = '' THEN u.email
        WHEN EXISTS (SELECT 1 FROM users o WHERE o.email = @email::text AND o.user_id <> u.user_id) THEN u.email
        ELSE @email::text
    END
WHERE u.user_id = @user_id;

-- name: GetUserProfile :one
SELECT u.user_id, u.username, u.email, u.create_date, u.tier_id, t.tier_name
FROM users u
JOIN tiers t ON t.tier_id = u.tier_id
WHERE u.user_id = $1;

-- Access tokens cannot be revoked, so every request checks that its account still exists.
-- name: UserExists :one
SELECT EXISTS (SELECT 1 FROM users WHERE user_id = $1);
