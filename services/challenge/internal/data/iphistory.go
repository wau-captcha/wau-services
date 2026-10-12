package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"challenge/internal/biz"

	"github.com/redis/go-redis/v9"
)

// The history of an address lives in one bucket key per kind and 10-minute window,
// wau:ip:{<address>}:<kind>:<bucket>. The hash tag keeps all of an address on one node of the
// cluster, so its buckets can be read with one MGET.
const historyBuckets = 6

type ipHistory struct {
	data *Data
}

func NewIPHistory(data *Data) biz.IPHistory {
	return &ipHistory{data: data}
}

func historyKey(addr, kind string, bucket int64) string {
	return fmt.Sprintf("wau:ip:{%s}:%s:%d", addr, kind, bucket)
}

func bucketOf(t time.Time) int64 {
	return t.Unix() / int64(biz.HistoryBucket/time.Second)
}

func (h *ipHistory) WrongAnswers(ctx context.Context, addr string, now time.Time) (int64, error) {
	return readCounter(ctx, h.data.redis, historyKey(addr, "wrong", bucketOf(now)))
}

func (h *ipHistory) AutomatedLastHour(ctx context.Context, addr string, now time.Time) (int64, error) {
	return h.lastHour(ctx, addr, "auto", now)
}

func (h *ipHistory) AutoPassesLastHour(ctx context.Context, addr string, now time.Time) (int64, error) {
	return h.lastHour(ctx, addr, "pass", now)
}

// lastHour adds up the buckets of one kind of addr over the hour up to now.
func (h *ipHistory) lastHour(ctx context.Context, addr, kind string, now time.Time) (int64, error) {
	current := bucketOf(now)
	keys := make([]string, historyBuckets)
	for i := range keys {
		keys[i] = historyKey(addr, kind, current-int64(i))
	}
	values, err := h.data.redis.MGet(ctx, keys...).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return 0, err
	}
	var total int64
	for _, value := range values {
		if text, ok := value.(string); ok {
			var count int64
			if _, err := fmt.Sscan(text, &count); err == nil {
				total += count
			}
		}
	}
	return total, nil
}

// A wrong-answer bucket is only read while it is current; an automated or a pass one for an hour.
func (h *ipHistory) CountWrong(ctx context.Context, addr string, now time.Time) error {
	return h.count(ctx, historyKey(addr, "wrong", bucketOf(now)), biz.HistoryBucket)
}

func (h *ipHistory) CountAutomated(ctx context.Context, addr string, now time.Time) error {
	return h.count(ctx, historyKey(addr, "auto", bucketOf(now)), historyBuckets*biz.HistoryBucket)
}

func (h *ipHistory) CountAutoPass(ctx context.Context, addr string, now time.Time) error {
	return h.count(ctx, historyKey(addr, "pass", bucketOf(now)), historyBuckets*biz.HistoryBucket)
}

func (h *ipHistory) count(ctx context.Context, key string, ttl time.Duration) error {
	_, err := h.data.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Incr(ctx, key)
		pipe.ExpireNX(ctx, key, ttl)
		return nil
	})
	return err
}
