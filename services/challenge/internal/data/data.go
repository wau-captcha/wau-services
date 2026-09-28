package data

import (
	"context"

	"challenge/internal/conf"
	"challenge/internal/data/db"
	"challenge/internal/pkg/envconf"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var ProviderSet = wire.NewSet(NewData, NewSiteRepo, NewChallengeRepo, NewCounterRepo, NewUsageRepo)

type Data struct {
	queries *db.Queries
	redis   *redis.ClusterClient
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
