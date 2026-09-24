package data

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"verify/internal/biz"

	"github.com/redis/go-redis/v9"
)

const grantKeyPrefix = "wau:token:"

// grantPO is the Redis value behind a token.
type grantPO struct {
	APIKeyID      string    `json:"api_key_id"`
	ChallengeType string    `json:"challenge_type"`
	SolvedAt      time.Time `json:"solved_at"`
}

type grantRepo struct {
	data *Data
}

// NewGrantRepo creates a GrantRepo backed by Redis.
func NewGrantRepo(data *Data) biz.GrantRepo {
	return &grantRepo{data: data}
}

func (r *grantRepo) Save(ctx context.Context, id string, grant *biz.Grant, ttl time.Duration) error {
	payload, err := json.Marshal(grantPO(*grant))
	if err != nil {
		return err
	}
	return r.data.redis.Set(ctx, grantKeyPrefix+id, payload, ttl).Err()
}

func (r *grantRepo) Take(ctx context.Context, id string) (*biz.Grant, error) {
	payload, err := r.data.redis.GetDel(ctx, grantKeyPrefix+id).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var record grantPO
	if err := json.Unmarshal(payload, &record); err != nil {
		return nil, err
	}
	grant := biz.Grant(record)
	return &grant, nil
}
