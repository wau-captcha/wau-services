package data

import (
	"context"

	"tenant/internal/conf"
	"tenant/internal/data/db"

	"common/envconf"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewUserRepo, NewApiKeyRepo, NewUsageRepo, NewCatalogRepo,
	NewCounterRepo, NewAdminRepo, NewAdminAccountRepo)

type Data struct {
	queries *db.Queries
	// The Redis Cluster where challenge and verify keep the daily counters.
	rdb *redis.ClusterClient
}

func NewData(c *conf.Data) (*Data, func(), error) {
	pool, err := pgxpool.New(context.Background(), c.GetDatabase().GetSource())
	if err != nil {
		return nil, nil, err
	}
	r := c.GetRedis()
	rdb := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        envconf.List(r.GetAddrs()),
		Password:     r.GetPassword(),
		ReadTimeout:  r.GetReadTimeout().AsDuration(),
		WriteTimeout: r.GetWriteTimeout().AsDuration(),
	})
	cleanup := func() {
		log.Info("closing the data resources")
		pool.Close()
		if err := rdb.Close(); err != nil {
			log.Error("failed closing redis", "err", err)
		}
	}
	return &Data{queries: db.New(pool), rdb: rdb}, cleanup, nil
}
