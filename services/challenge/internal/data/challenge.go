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
	Answer   string `json:"answer"`
}

type challengeRepo struct {
	data *Data
}

func NewChallengeRepo(data *Data) biz.ChallengeRepo {
	return &challengeRepo{data: data}
}

func (r *challengeRepo) Save(ctx context.Context, c *biz.Challenge) error {
	v, err := json.Marshal(challengePO{Type: c.Type, APIKeyID: c.APIKeyID, Answer: c.Answer})
	if err != nil {
		return err
	}
	return r.data.rdb.Set(ctx, challengeKeyPrefix+c.ID, v, c.TTL).Err()
}

func (r *challengeRepo) Take(ctx context.Context, id string) (*biz.Challenge, error) {
	v, err := r.data.rdb.GetDel(ctx, challengeKeyPrefix+id).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, biz.ErrChallengeNotFound
	}
	if err != nil {
		return nil, err
	}
	var po challengePO
	if err := json.Unmarshal(v, &po); err != nil {
		return nil, err
	}
	return &biz.Challenge{ID: id, Type: po.Type, APIKeyID: po.APIKeyID, Answer: po.Answer}, nil
}
