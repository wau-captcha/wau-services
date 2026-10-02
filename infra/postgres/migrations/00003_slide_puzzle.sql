-- +goose Up
INSERT INTO challenge_types (challenge_type_id, code, is_active) VALUES
    ('slide_puzzle',    'SLIDE_PUZZLE',     true)
ON CONFLICT DO NOTHING;

INSERT INTO tier_challenge_types (tier_id, challenge_type_id) VALUES
    ('free',        'slide_puzzle'),
    ('plus',        'slide_puzzle'),
    ('pro',         'slide_puzzle'),
    ('enterprise',  'slide_puzzle')
ON CONFLICT DO NOTHING;

-- +goose Down
DELETE FROM tier_challenge_types
WHERE challenge_type_id = 'slide_puzzle'
  AND tier_id IN ('free', 'plus', 'pro', 'enterprise');

DELETE FROM challenge_types
WHERE challenge_type_id = 'slide_puzzle'
  AND NOT EXISTS (SELECT 1 FROM usage_logs WHERE challenge_type_id = 'slide_puzzle');
