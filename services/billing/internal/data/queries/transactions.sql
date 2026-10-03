-- name: CreateTransaction :one
INSERT INTO billing_transactions (user_id, tier_id, payment_code, amount, period, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING billing_transaction_id, create_date;

-- A pending transaction past expires_at reads as expired before the sweep marks it.
-- name: ListTransactions :many
SELECT b.billing_transaction_id,
       b.tier_id,
       t.tier_name,
       t.monthly_price,
       b.payment_code,
       b.amount,
       b.period,
       (CASE WHEN b.status = 'pending' AND b.expires_at <= now() THEN 'expired' ELSE b.status END)::text AS status,
       b.paid_at,
       b.create_date,
       b.expires_at,
       b.sepay_transaction_id,
       b.grant_kind,
       b.granted_tier_id,
       g.tier_name AS granted_tier_name,
       b.plan_starts_at,
       b.plan_ends_at
FROM billing_transactions b
JOIN tiers t ON t.tier_id = b.tier_id
LEFT JOIN tiers g ON g.tier_id = b.granted_tier_id
WHERE b.user_id = $1
ORDER BY b.create_date DESC, b.billing_transaction_id DESC
LIMIT $2;

-- name: GetTransaction :one
SELECT b.billing_transaction_id,
       b.tier_id,
       t.tier_name,
       t.monthly_price,
       b.payment_code,
       b.amount,
       b.period,
       (CASE WHEN b.status = 'pending' AND b.expires_at <= now() THEN 'expired' ELSE b.status END)::text AS status,
       b.paid_at,
       b.create_date,
       b.expires_at,
       b.sepay_transaction_id,
       b.grant_kind,
       b.granted_tier_id,
       g.tier_name AS granted_tier_name,
       b.plan_starts_at,
       b.plan_ends_at
FROM billing_transactions b
JOIN tiers t ON t.tier_id = b.tier_id
LEFT JOIN tiers g ON g.tier_id = b.granted_tier_id
WHERE b.billing_transaction_id = $1
  AND b.user_id = $2;

-- Locks the transaction until the database transaction ends, so a payment reported twice at
-- once is applied once.
-- name: LockTransactionByCode :one
SELECT b.billing_transaction_id,
       b.user_id,
       b.tier_id,
       t.monthly_price,
       b.amount,
       b.period,
       b.status,
       b.sepay_transaction_id
FROM billing_transactions b
JOIN tiers t ON t.tier_id = b.tier_id
WHERE b.payment_code = $1
FOR UPDATE OF b;

-- name: MarkTransactionPaid :exec
UPDATE billing_transactions
SET status = 'succeeded',
    sepay_transaction_id = @sepay_transaction_id::text,
    paid_at = @paid_at,
    grant_kind = @grant_kind::text,
    granted_tier_id = @granted_tier_id::text,
    plan_starts_at = @plan_starts_at,
    plan_ends_at = @plan_ends_at
WHERE billing_transaction_id = @billing_transaction_id;

-- name: ExpireTransactions :execrows
UPDATE billing_transactions
SET status = 'expired'
WHERE status = 'pending'
  AND expires_at <= now();
