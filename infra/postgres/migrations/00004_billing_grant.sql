-- +goose Up
ALTER TABLE billing_transactions
    ADD COLUMN grant_kind       TEXT CHECK (grant_kind IN ('upgrade', 'renew', 'extend')),
    ADD COLUMN granted_tier_id  TEXT REFERENCES tiers(tier_id),
    ADD COLUMN plan_starts_at   TIMESTAMPTZ,
    ADD COLUMN plan_ends_at     TIMESTAMPTZ,
    ADD CONSTRAINT billing_transactions_grant_check CHECK (grant_kind IS NULL OR (status = 'succeeded' AND granted_tier_id IS NOT NULL)),
    ADD CONSTRAINT billing_transactions_grant_period_check CHECK ((plan_starts_at IS NULL) = (plan_ends_at IS NULL) AND (plan_starts_at IS NULL OR grant_kind IS NOT NULL));

-- +goose Down
ALTER TABLE billing_transactions
    DROP CONSTRAINT billing_transactions_grant_period_check,
    DROP CONSTRAINT billing_transactions_grant_check,
    DROP COLUMN plan_ends_at,
    DROP COLUMN plan_starts_at,
    DROP COLUMN granted_tier_id,
    DROP COLUMN grant_kind;
