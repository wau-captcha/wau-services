package data

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"challenge/internal/biz"

	"github.com/redis/go-redis/v9"
)

const challengeKeyPrefix = "wau:challenge:"

// challengePO is the Redis value of a challenge. Fields after issued_at are absent from records
// earlier versions wrote; read as zero, they turn the checks that need them off.
type challengePO struct {
	Type     string `json:"type"`
	APIKeyID string `json:"api_key_id"`
	UserID   string `json:"user_id"`
	Answer   string `json:"answer"`
	// Unix milliseconds.
	IssuedAt        int64  `json:"issued_at,omitempty"`
	SiteKey         string `json:"site_key,omitempty"`
	Origin          string `json:"origin,omitempty"`
	ClientIP        string `json:"client_ip,omitempty"`
	UserAgentHash   string `json:"ua_hash,omitempty"`
	RequireBehavior bool   `json:"require_behavior,omitempty"`
	// Base64 in JSON.
	PowSalt       []byte `json:"pow_salt,omitempty"`
	PowDifficulty int    `json:"pow_difficulty,omitempty"`
}

type challengeRepo struct {
	data *Data
}

func NewChallengeRepo(data *Data) biz.ChallengeRepo {
	return &challengeRepo{data: data}
}

func (r *challengeRepo) Save(ctx context.Context, challenge *biz.Challenge) error {
	payload, err := json.Marshal(challengePO{
		Type:            challenge.Type,
		APIKeyID:        challenge.APIKeyID,
		UserID:          challenge.UserID,
		Answer:          challenge.Answer,
		IssuedAt:        challenge.IssuedAt.UnixMilli(),
		SiteKey:         challenge.SiteKey,
		Origin:          challenge.Origin,
		ClientIP:        challenge.ClientIP,
		UserAgentHash:   challenge.UserAgentHash,
		RequireBehavior: challenge.RequireBehavior,
		PowSalt:         challenge.PowSalt,
		PowDifficulty:   challenge.PowDifficulty,
	})
	if err != nil {
		return err
	}
	return r.data.redis.Set(ctx, challengeKeyPrefix+challenge.ID, payload, challenge.TTL).Err()
}

func (r *challengeRepo) Take(ctx context.Context, id string) (*biz.Challenge, error) {
	payload, err := r.data.redis.GetDel(ctx, challengeKeyPrefix+id).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, biz.ErrChallengeNotFound
	}
	if err != nil {
		return nil, err
	}
	var record challengePO
	if err := json.Unmarshal(payload, &record); err != nil {
		return nil, err
	}
	challenge := &biz.Challenge{
		ID:              id,
		Type:            record.Type,
		APIKeyID:        record.APIKeyID,
		SiteKey:         record.SiteKey,
		UserID:          record.UserID,
		Answer:          record.Answer,
		Origin:          record.Origin,
		ClientIP:        record.ClientIP,
		UserAgentHash:   record.UserAgentHash,
		RequireBehavior: record.RequireBehavior,
		PowSalt:         record.PowSalt,
		PowDifficulty:   record.PowDifficulty,
	}
	if record.IssuedAt > 0 {
		challenge.IssuedAt = time.UnixMilli(record.IssuedAt)
	}
	return challenge, nil
}
