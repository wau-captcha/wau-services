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
	// Absent from tokens issued before behaviour was checked: read as not measured.
	BehaviorVerdict string   `json:"behavior_verdict,omitempty"`
	BehaviorScore   *float64 `json:"behavior_score,omitempty"`
	BehaviorReasons []string `json:"behavior_reasons,omitempty"`
}

type grantRepo struct {
	data *Data
}

// NewGrantRepo creates a GrantRepo backed by Redis.
func NewGrantRepo(data *Data) biz.GrantRepo {
	return &grantRepo{data: data}
}

func (r *grantRepo) Save(ctx context.Context, id string, grant *biz.Grant, ttl time.Duration) error {
	payload, err := json.Marshal(grantPO{
		APIKeyID:        grant.APIKeyID,
		ChallengeType:   grant.ChallengeType,
		SolvedAt:        grant.SolvedAt,
		BehaviorVerdict: grant.Behavior.Verdict,
		BehaviorScore:   grant.Behavior.Score,
		BehaviorReasons: grant.Behavior.Reasons,
	})
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
	behavior := biz.BehaviorResult{Verdict: record.BehaviorVerdict, Score: record.BehaviorScore, Reasons: record.BehaviorReasons}
	if behavior.Verdict == "" {
		behavior = biz.BehaviorResult{Verdict: biz.VerdictNotMeasured}
	}
	return &biz.Grant{APIKeyID: record.APIKeyID, ChallengeType: record.ChallengeType, SolvedAt: record.SolvedAt, Behavior: behavior}, nil
}
