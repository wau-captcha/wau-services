-- +goose Up
ALTER TABLE usage_logs
    ADD COLUMN behavior_verdict TEXT CHECK (behavior_verdict IN ('human', 'suspicious', 'automated', 'not_measured')),
    ADD COLUMN behavior_score   REAL CHECK (behavior_score >= 0 AND behavior_score <= 1);

-- +goose Down
ALTER TABLE usage_logs
    DROP COLUMN behavior_score,
    DROP COLUMN behavior_verdict;
