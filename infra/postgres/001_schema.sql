CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE admins (
    admin_id    UUID PRIMARY KEY DEFAULT uuidv7(),
    username    TEXT NOT NULL UNIQUE,
    password    TEXT NOT NULL,
    create_date TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE tiers (
    tier_id     TEXT PRIMARY KEY,
    tier_name   TEXT NOT NULL,
    description TEXT,
    is_active   BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE users (
    user_id     UUID PRIMARY KEY DEFAULT uuidv7(),
    username    TEXT NOT NULL,
    email       TEXT NOT NULL UNIQUE,
    create_date TIMESTAMPTZ NOT NULL DEFAULT now(),
    tier_id     TEXT NOT NULL REFERENCES tiers(tier_id) DEFAULT 'free',
    google_sub  TEXT UNIQUE
);

CREATE INDEX idx_users_tier_id ON users(tier_id);

CREATE TABLE api_keys (
    api_key_id  UUID PRIMARY KEY DEFAULT uuidv7(),
    site_key    TEXT NOT NULL UNIQUE,
    secret_key  TEXT NOT NULL UNIQUE,
    is_active   BOOLEAN NOT NULL DEFAULT true,
    is_deleted  BOOLEAN NOT NULL DEFAULT false,
    create_date TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_id     UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_api_keys_site_key ON api_keys(site_key);
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);

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
    api_key_id         UUID NOT NULL REFERENCES api_keys(api_key_id) ON DELETE CASCADE,
    challenge_type_id  TEXT NOT NULL REFERENCES challenge_types(challenge_type_id),
    timestamp          TIMESTAMPTZ NOT NULL DEFAULT now(),
    success            BOOLEAN NOT NULL
);

CREATE INDEX idx_usage_log_api_key_time ON usage_logs(api_key_id, timestamp DESC);