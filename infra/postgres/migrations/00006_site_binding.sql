-- +goose Up
CREATE TABLE api_key_allowed_domains (
    api_key_id  UUID NOT NULL REFERENCES api_keys(api_key_id) ON DELETE CASCADE,
    domain      TEXT NOT NULL,
    create_date TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (api_key_id, domain)
);

ALTER TABLE api_keys
    ADD COLUMN scraping_flagged_at TIMESTAMPTZ,
    ADD COLUMN require_behavior    BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE api_keys ALTER COLUMN require_behavior SET DEFAULT true;

CREATE TABLE api_key_appeals (
    api_key_appeal_id UUID PRIMARY KEY DEFAULT uuidv7(),
    api_key_id        UUID NOT NULL REFERENCES api_keys(api_key_id) ON DELETE CASCADE,
    user_id           UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    message           TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    admin_note        TEXT,
    create_date       TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at        TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_api_key_appeals_pending ON api_key_appeals(api_key_id) WHERE status = 'pending';
CREATE INDEX idx_api_key_appeals_key_time ON api_key_appeals(api_key_id, create_date DESC);

CREATE TABLE api_key_events (
    api_key_event_id BIGSERIAL PRIMARY KEY,
    api_key_id       UUID NOT NULL REFERENCES api_keys(api_key_id) ON DELETE CASCADE,
    actor            TEXT NOT NULL CHECK (actor IN ('user', 'admin', 'system')),
    actor_user_id    UUID,
    action           TEXT NOT NULL,
    detail           JSONB NOT NULL DEFAULT '{}',
    create_date      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_api_key_events_key_time ON api_key_events(api_key_id, create_date DESC);

-- +goose Down
DROP TABLE api_key_events;
DROP TABLE api_key_appeals;

ALTER TABLE api_keys
    DROP COLUMN require_behavior,
    DROP COLUMN scraping_flagged_at;

DROP TABLE api_key_allowed_domains;
