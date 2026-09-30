package data

import (
	"context"
	"crypto/tls"
	"net"

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
	addrs := envconf.SplitList(redisConf.GetAddrs())
	redisClient := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        addrs,
		Password:     redisConf.GetPassword(),
		ReadTimeout:  redisConf.GetReadTimeout().AsDuration(),
		WriteTimeout: redisConf.GetWriteTimeout().AsDuration(),
		TLSConfig:    redisTLS(redisConf.GetTls(), addrs),
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

// redisTLS is nil, plain TCP, unless tls is on. Every node is then checked against the name of
// the first seed node, as go-redis does for rediss:// cluster URLs: nodes may announce themselves
// by IP address, which the cluster's certificate does not name (ElastiCache issues one for
// *.<cluster>.<id>.<region>.cache.amazonaws.com, covering the configuration endpoint and nodes).
func redisTLS(on bool, addrs []string) *tls.Config {
	if !on {
		return nil
	}
	config := &tls.Config{MinVersion: tls.VersionTLS12}
	if len(addrs) > 0 {
		if host, _, err := net.SplitHostPort(addrs[0]); err == nil {
			config.ServerName = host
		}
	}
	return config
}
