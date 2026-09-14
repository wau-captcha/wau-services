package data

import (
	"context"

	"challenge/internal/conf"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewSiteRepo, NewChallengeRepo)

// Data holds the long-lived storage clients shared by repos.
type Data struct {
	db  *pgxpool.Pool
	rdb *redis.ClusterClient
}

// NewData opens the PostgreSQL pool and the Redis Cluster client.
func NewData(c *conf.Data) (*Data, func(), error) {
	db, err := pgxpool.New(context.Background(), c.GetDatabase().GetSource())
	if err != nil {
		return nil, nil, err
	}
	r := c.GetRedis()
	rdb := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        r.GetAddrs(),
		Password:     r.GetPassword(),
		ReadTimeout:  r.GetReadTimeout().AsDuration(),
		WriteTimeout: r.GetWriteTimeout().AsDuration(),
	})
	cleanup := func() {
		log.Info("closing the data resources")
		db.Close()
		if err := rdb.Close(); err != nil {
			log.Error("failed closing redis", "err", err)
		}
	}
	return &Data{db: db, rdb: rdb}, cleanup, nil
}
