-- A /challenge/new call, not yet answered (success stays null). The verify service writes the
-- same row when the answer arrives first, so an existing row is left as it is.
-- name: CreateIssuedUsageLog :exec
INSERT INTO usage_logs (challenge_id, api_key_id, challenge_type_id)
VALUES (@challenge_id::uuid, @api_key_id, @challenge_type_id)
ON CONFLICT (challenge_id) DO NOTHING;
