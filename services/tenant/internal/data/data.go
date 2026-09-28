package data

import (
	"context"

	"tenant/internal/conf"
	"tenant/internal/data/db"
	"tenant/internal/pkg/envconf"

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
	redis *redis.ClusterClient
}

func NewData(dataConf *conf.Data) (*Data, func(), error) {
	pool, err := pgxpool.New(context.Background(), dataConf.GetDatabase().GetSource())
	if err != nil {
		return nil, nil, err
	}
	redisConf := dataConf.GetRedis()
	redisClient := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        envconf.SplitList(redisConf.GetAddrs()),
		Password:     redisConf.GetPassword(),
		ReadTimeout:  redisConf.GetReadTimeout().AsDuration(),
		WriteTimeout: redisConf.GetWriteTimeout().AsDuration(),
	})
	cleanup := func() {
		log.Info("closing the data resources")
		pool.Close()
		if err := redisClient.Close(); err != nil {
			log.Error("failed closing redis", "err", err)
		}
	}
	return &Data{queries: db.New(pool), redis: redisClient}, cleanup, nil
}
