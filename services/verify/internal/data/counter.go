package data

import (
	"context"

	"verify/internal/biz"

	"common/quota"

	"github.com/redis/go-redis/v9"
)

type counterRepo struct {
	data *Data
}

func NewCounterRepo(data *Data) biz.CounterRepo {
	return &counterRepo{data: data}
}

func (r *counterRepo) CountVerification(ctx context.Context, apiKeyID, day string) error {
	return incrDaily(ctx, r.data.rdb, quota.VerificationsKey(apiKeyID, day))
}

func (r *counterRepo) CountSuccess(ctx context.Context, userID, day string) error {
	return incrDaily(ctx, r.data.rdb, quota.UsedKey(userID, day))
}

// incrDaily adds one to a daily counter. The first increment creates it, and only
// then does the expiry get set, so the counter lives for quota.TTL from its creation.
func incrDaily(ctx context.Context, rdb redis.Cmdable, key string) error {
	_, err := rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Incr(ctx, key)
		pipe.ExpireNX(ctx, key, quota.TTL)
		return nil
	})
	return err
}
