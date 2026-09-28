-- Access tokens cannot be revoked, so every request checks that its account still exists
-- and is not banned; no row means the account is gone.
-- name: GetAccountStatus :one
SELECT is_banned
FROM users
WHERE user_id = $1;

-- The tier a user has now: a paid tier past its end is already free, as for every service.
-- name: GetAccountPlan :one
SELECT t.tier_id,
       t.monthly_price,
       (CASE WHEN u.tier_expires_at > now() THEN u.tier_expires_at END)::timestamptz AS tier_expires_at
FROM users u
JOIN tiers t ON t.tier_id = CASE WHEN u.tier_expires_at <= now() THEN 'free' ELSE u.tier_id END
WHERE u.user_id = $1;

-- GetAccountPlan that also locks the user until the transaction ends, so payments applied at
-- the same time extend one another instead of overwriting.
-- name: LockAccountPlan :one
SELECT t.tier_id,
       t.monthly_price,
       (CASE WHEN u.tier_expires_at > now() THEN u.tier_expires_at END)::timestamptz AS tier_expires_at
FROM users u
JOIN tiers t ON t.tier_id = CASE WHEN u.tier_expires_at <= now() THEN 'free' ELSE u.tier_id END
WHERE u.user_id = $1
FOR UPDATE OF u;

-- name: SetAccountPlan :exec
UPDATE users
SET tier_id = @tier_id,
    tier_expires_at = @tier_expires_at
WHERE user_id = @user_id;

-- Paid tiers past their end go back to free. A row locked by a payment is checked again once
-- the payment commits, so a tier it just extended is left alone.
-- name: EndLapsedTiers :execrows
UPDATE users
SET tier_id = 'free',
    tier_expires_at = NULL
WHERE tier_expires_at <= now();
