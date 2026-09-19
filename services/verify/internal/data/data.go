package data

import (
	"context"

	"verify/internal/conf"
	"verify/internal/data/db"

	"common/envconf"
	challengev1 "common/proto/challenge/v1"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/transport/grpc"
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var ProviderSet = wire.NewSet(NewData, NewChallengeServiceClient, NewChallengeClient, NewGrantRepo, NewSiteRepo, NewCounterRepo)

type Data struct {
	queries *db.Queries
	rdb     *redis.ClusterClient
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

func NewChallengeServiceClient(c *conf.Client) (challengev1.ChallengeServiceClient, func(), error) {
	conn, err := grpc.NewClient(context.Background(),
		grpc.WithEndpoint(c.GetChallenge().GetEndpoint()),
		grpc.WithTimeout(c.GetChallenge().GetTimeout().AsDuration()),
	)
	if err != nil {
		return nil, nil, err
	}
	return challengev1.NewChallengeServiceClient(conn), func() { conn.Close() }, nil
}
