package data

import (
	"context"
	"errors"

	"tenant/internal/biz"

	"common/quota"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type counterRepo struct {
	data *Data
}

func NewCounterRepo(data *Data) biz.CounterRepo { return &counterRepo{data: data} }

func (r *counterRepo) QuotaUsed(ctx context.Context, userIDs []uuid.UUID, day string) ([]int64, error) {
	keys := make([]string, len(userIDs))
	for i, id := range userIDs {
		keys[i] = quota.UsedKey(id.String(), day)
	}
	return r.counts(ctx, keys)
}

func (r *counterRepo) ScrapingCounts(ctx context.Context, apiKeyIDs []uuid.UUID, day string) ([]biz.ScrapingCount, error) {
	keys := make([]string, 0, 2*len(apiKeyIDs))
	for _, id := range apiKeyIDs {
		keys = append(keys, quota.ChallengesKey(id.String(), day), quota.VerificationsKey(id.String(), day))
	}
	values, err := r.counts(ctx, keys)
	if err != nil {
		return nil, err
	}
	counts := make([]biz.ScrapingCount, len(apiKeyIDs))
	for i := range counts {
		counts[i] = biz.ScrapingCount{Challenges: values[2*i], Verifications: values[2*i+1]}
	}
	return counts, nil
}

func (r *counterRepo) ResetScraping(ctx context.Context, apiKeyID uuid.UUID, day string) error {
	id := apiKeyID.String()
	// One DEL per key: in a cluster the two counters may live on different nodes.
	_, err := r.data.rdb.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx, quota.ChallengesKey(id, day))
		pipe.Del(ctx, quota.VerificationsKey(id, day))
		return nil
	})
	return err
}

// counts reads counters in one round trip per node. A pipeline rather than MGET, which a
// cluster refuses for keys in different slots. A missing counter reads as zero.
func (r *counterRepo) counts(ctx context.Context, keys []string) ([]int64, error) {
	cmds := make([]*redis.StringCmd, len(keys))
	_, err := r.data.rdb.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for i, key := range keys {
			cmds[i] = pipe.Get(ctx, key)
		}
		return nil
	})
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	values := make([]int64, len(keys))
	for i, cmd := range cmds {
		value, err := cmd.Int64()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, err
		}
		values[i] = value
	}
	return values, nil
}
