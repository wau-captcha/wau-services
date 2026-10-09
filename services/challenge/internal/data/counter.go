package data

import (
	"context"
	"errors"

	"challenge/internal/biz"
	"challenge/internal/pkg/quota"

	"github.com/redis/go-redis/v9"
)

type counterRepo struct {
	data *Data
}

func NewCounterRepo(data *Data) biz.CounterRepo {
	return &counterRepo{data: data}
}

func (r *counterRepo) QuotaUsed(ctx context.Context, userID, day string) (int64, error) {
	return readCounter(ctx, r.data.redis, quota.DailyQuotaKey(userID, day))
}

func (r *counterRepo) CountChallenge(ctx context.Context, apiKeyID, day string) (biz.DailyCounts, error) {
	issued, err := incrementDailyCounter(ctx, r.data.redis, quota.IssuedChallengesKey(apiKeyID, day))
	if err != nil {
		return biz.DailyCounts{}, err
	}
	// The other three are written by verify; they are on different nodes of the cluster, so they
	// are read in one pipeline rather than one MGET.
	keys := []string{quota.CheckedAnswersKey(apiKeyID, day), quota.SolvedAnswersKey(apiKeyID, day), quota.AutomatedAnswersKey(apiKeyID, day)}
	values, err := readCounters(ctx, r.data.redis, keys)
	if err != nil {
		return biz.DailyCounts{}, err
	}
	return biz.DailyCounts{Issued: issued, Answered: values[0], Solved: values[1], Automated: values[2]}, nil
}

// incrementDailyCounter adds one to a daily counter. The first increment creates it, and only
// then does the expiry get set, so the counter lives for quota.CounterTTL from its creation.
func incrementDailyCounter(ctx context.Context, client redis.Cmdable, key string) (int64, error) {
	var count *redis.IntCmd
	_, err := client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		count = pipe.Incr(ctx, key)
		pipe.ExpireNX(ctx, key, quota.CounterTTL)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count.Val(), nil
}

// readCounter reads a counter; one that does not exist yet counts as zero.
func readCounter(ctx context.Context, client redis.Cmdable, key string) (int64, error) {
	count, err := client.Get(ctx, key).Int64()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	return count, err
}

// readCounters reads counters in one pipeline; one that does not exist yet counts as zero.
func readCounters(ctx context.Context, client redis.Cmdable, keys []string) ([]int64, error) {
	cmds := make([]*redis.StringCmd, len(keys))
	_, err := client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
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
