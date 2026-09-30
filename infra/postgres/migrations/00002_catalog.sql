-- +goose Up
INSERT INTO tiers (tier_id, tier_name, description, is_active, daily_quota, monthly_price, yearly_price) VALUES
    ('free',        'Free',    'For trying out and small personal projects. Basic challenge types with a limited daily quota.', true, 5000, 0, 0),
    ('plus',        'Plus',    'For growing websites. Unlocks advanced challenge types with a higher daily quota.', true, 100000, 10000, 100000),
    ('pro',         'Pro',     'For established, higher-traffic sites. Larger daily quota with all advanced challenge types.', true, 1000000, 30000, 30000),
    ('enterprise',  'Enterprise', 'For high-volume platforms. Highest daily quota with full access to every challenge type.', true, 10000000, 50000, 50000);

INSERT INTO challenge_types (challenge_type_id, code, is_active) VALUES
    ('image_distort',   'IMAGE_DISTORT',    true),
    ('motion_noise',    'MOTION_NOISE',     true);

INSERT INTO tier_challenge_types (tier_id, challenge_type_id) VALUES
    ('free',        'image_distort'),
    ('free',        'motion_noise'),
    ('plus',        'image_distort'),
    ('plus',        'motion_noise'),
    ('pro',         'image_distort'),
    ('pro',         'motion_noise'),
    ('enterprise',  'image_distort'),
    ('enterprise',  'motion_noise');

-- +goose Down
DELETE FROM challenge_types WHERE challenge_type_id IN ('image_distort', 'motion_noise');
DELETE FROM tiers WHERE tier_id IN ('free', 'plus', 'pro', 'enterprise');
