package data

import (
	"challenge/internal/biz"
	"challenge/internal/pkg/ratelimit"
)

// NewRateLimiter counts the rate limits in the Redis Cluster the challenges live in.
func NewRateLimiter(data *Data) biz.RateLimiter {
	return ratelimit.New(data.redis)
}
