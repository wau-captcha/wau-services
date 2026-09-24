package data

import (
	"context"
	"encoding/json"
	"errors"

	"challenge/internal/biz"

	"github.com/redis/go-redis/v9"
)

const challengeKeyPrefix = "wau:challenge:"

type challengePO struct {
	Type     string `json:"type"`
	APIKeyID string `json:"api_key_id"`
	UserID   string `json:"user_id"`
	Answer   string `json:"answer"`
}

type challengeRepo struct {
	data *Data
}

func NewChallengeRepo(data *Data) biz.ChallengeRepo {
	return &challengeRepo{data: data}
}

func (r *challengeRepo) Save(ctx context.Context, challenge *biz.Challenge) error {
	payload, err := json.Marshal(challengePO{Type: challenge.Type, APIKeyID: challenge.APIKeyID, UserID: challenge.UserID, Answer: challenge.Answer})
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
	return &biz.Challenge{ID: id, Type: record.Type, APIKeyID: record.APIKeyID, UserID: record.UserID, Answer: record.Answer}, nil
}
