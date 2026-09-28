CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE tiers (
    tier_id     TEXT PRIMARY KEY,
    tier_name   TEXT NOT NULL,
    description TEXT,
    is_active   BOOLEAN NOT NULL DEFAULT true,
    daily_quota INTEGER NOT NULL CONSTRAINT tiers_daily_quota_check CHECK (daily_quota >= 0),
    monthly_price INTEGER NOT NULL DEFAULT 0 CHECK (monthly_price >= 0),
    yearly_price INTEGER NOT NULL DEFAULT 0 CHECK (yearly_price >= 0)
);

CREATE TABLE users (
    user_id     UUID PRIMARY KEY DEFAULT uuidv7(),
    username    TEXT NOT NULL,
    email       TEXT NOT NULL UNIQUE,
    create_date TIMESTAMPTZ NOT NULL DEFAULT now(),
    tier_id     TEXT NOT NULL REFERENCES tiers(tier_id) DEFAULT 'free',
    tier_expires_at TIMESTAMPTZ,
    google_sub  TEXT UNIQUE,
    is_banned   BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX idx_users_tier_id ON users(tier_id);
CREATE INDEX idx_users_tier_expires_at ON users(tier_expires_at) WHERE tier_expires_at IS NOT NULL;

CREATE TABLE admins (
    admin_id    UUID PRIMARY KEY DEFAULT uuidv7(),
    username    TEXT NOT NULL,
    email       TEXT NOT NULL UNIQUE,
    create_date TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE api_keys (
    api_key_id  UUID PRIMARY KEY DEFAULT uuidv7(),
    name        TEXT NOT NULL DEFAULT '',
    site_key    TEXT NOT NULL UNIQUE,
    secret_key  TEXT NOT NULL UNIQUE,
    is_active   BOOLEAN NOT NULL DEFAULT true,
    is_deleted  BOOLEAN NOT NULL DEFAULT false,
    create_date TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_id     UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    is_suspended   BOOLEAN NOT NULL DEFAULT false,
    suspend_reason TEXT CONSTRAINT api_keys_suspend_reason_check CHECK (suspend_reason IN ('ADMIN', 'SCRAPING')),
    previous_secret_key TEXT UNIQUE,
    previous_secret_key_expires_at TIMESTAMPTZ,
    CONSTRAINT api_keys_suspension_check CHECK (is_suspended = (suspend_reason IS NOT NULL)),
    CONSTRAINT api_keys_previous_secret_check CHECK ((previous_secret_key IS NULL) = (previous_secret_key_expires_at IS NULL))
);

CREATE INDEX idx_api_keys_site_key ON api_keys(site_key);
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);

CREATE TABLE api_key_allowed_origins (
    api_key_id  UUID NOT NULL REFERENCES api_keys(api_key_id) ON DELETE CASCADE,
    ip_address  INET NOT NULL,
    description TEXT,
    create_date TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (api_key_id, ip_address)
);

CREATE TABLE challenge_types (
    challenge_type_id TEXT PRIMARY KEY,
    code               TEXT NOT NULL,
    is_active          BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE tier_challenge_types (
    tier_id            TEXT NOT NULL REFERENCES tiers(tier_id) ON DELETE CASCADE,
    challenge_type_id  TEXT NOT NULL REFERENCES challenge_types(challenge_type_id) ON DELETE CASCADE,
    PRIMARY KEY (tier_id, challenge_type_id)
);

CREATE TABLE usage_logs (
    usage_log_id       BIGSERIAL PRIMARY KEY,
    challenge_id       UUID UNIQUE,
    api_key_id         UUID NOT NULL REFERENCES api_keys(api_key_id) ON DELETE CASCADE,
    challenge_type_id  TEXT NOT NULL REFERENCES challenge_types(challenge_type_id),
    timestamp          TIMESTAMPTZ NOT NULL DEFAULT now(),
    success            BOOLEAN
);

CREATE INDEX idx_usage_log_api_key_time ON usage_logs(api_key_id, timestamp DESC);

CREATE TABLE billing_transactions (
    billing_transaction_id  UUID PRIMARY KEY DEFAULT uuidv7(),
    user_id                 UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    tier_id                 TEXT NOT NULL REFERENCES tiers(tier_id),
    payment_code            TEXT NOT NULL UNIQUE,
    amount                  INTEGER NOT NULL CHECK (amount > 0),
    period                  TEXT NOT NULL DEFAULT 'month' CHECK (period IN ('month', 'year')),
    status                  TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'succeeded', 'expired')),
    sepay_transaction_id    TEXT UNIQUE,
    paid_at                 TIMESTAMPTZ,
    create_date             TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at              TIMESTAMPTZ NOT NULL,
    CONSTRAINT billing_transactions_payment_check CHECK ((status = 'succeeded') = (sepay_transaction_id IS NOT NULL) AND (status = 'succeeded') = (paid_at IS NOT NULL))
);

CREATE INDEX idx_billing_transactions_user_id ON billing_transactions(user_id, create_date DESC);
CREATE INDEX idx_billing_transactions_pending ON billing_transactions(expires_at) WHERE status = 'pending';