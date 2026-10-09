package data

import (
	"context"
	"errors"

	"tenant/internal/biz"
	"tenant/internal/pkg/quota"

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
		keys[i] = quota.DailyQuotaKey(id.String(), day)
	}
	return r.counts(ctx, keys)
}

// scrapingKeys are the four counters of the scraping check of a key on a day, in the order of
// biz.ScrapingCount.
func scrapingKeys(apiKeyID uuid.UUID, day string) []string {
	id := apiKeyID.String()
	return []string{quota.IssuedChallengesKey(id, day), quota.CheckedAnswersKey(id, day), quota.SolvedAnswersKey(id, day),
		quota.AutomatedAnswersKey(id, day)}
}

func (r *counterRepo) ScrapingCounts(ctx context.Context, apiKeyIDs []uuid.UUID, day string) ([]biz.ScrapingCount, error) {
	keys := make([]string, 0, 4*len(apiKeyIDs))
	for _, id := range apiKeyIDs {
		keys = append(keys, scrapingKeys(id, day)...)
	}
	values, err := r.counts(ctx, keys)
	if err != nil {
		return nil, err
	}
	counts := make([]biz.ScrapingCount, len(apiKeyIDs))
	for i := range counts {
		counts[i] = biz.ScrapingCount{Challenges: values[4*i], Verifications: values[4*i+1], Solved: values[4*i+2], Automated: values[4*i+3]}
	}
	return counts, nil
}

func (r *counterRepo) ResetScraping(ctx context.Context, apiKeyID uuid.UUID, day string) error {
	// One DEL per key: in a cluster the counters may live on different nodes.
	_, err := r.data.redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, key := range scrapingKeys(apiKeyID, day) {
			pipe.Del(ctx, key)
		}
		return nil
	})
	return err
}

// counts reads counters in one round trip per node. A pipeline rather than MGET, which a
// cluster refuses for keys in different slots. A missing counter reads as zero.
func (r *counterRepo) counts(ctx context.Context, keys []string) ([]int64, error) {
	cmds := make([]*redis.StringCmd, len(keys))
	_, err := r.data.redis.Pipelined(ctx, func(pipe redis.Pipeliner) error {
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
