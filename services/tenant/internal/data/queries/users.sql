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

-- A paid tier past tier_expires_at is already free here, before billing's sweep stores it.
-- name: GetUserProfile :one
SELECT u.user_id, u.username, u.email, u.create_date, t.tier_id, t.tier_name, t.daily_quota, u.is_banned,
       (CASE WHEN u.tier_expires_at > now() THEN u.tier_expires_at END)::timestamptz AS tier_expires_at
FROM users u
JOIN tiers t ON t.tier_id = CASE WHEN u.tier_expires_at <= now() THEN 'free' ELSE u.tier_id END
WHERE u.user_id = $1;

-- Access tokens cannot be revoked, so every request checks that its account still exists
-- and is not banned; no row means the account is gone.
-- name: GetAccountStatus :one
SELECT is_banned
FROM users
WHERE user_id = $1;
