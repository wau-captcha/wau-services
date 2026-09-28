package data

import (
	"context"

	challengev1 "verify/api/challenge/v1"
	"verify/internal/conf"
	"verify/internal/data/db"
	"verify/internal/pkg/envconf"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/transport/grpc"
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var ProviderSet = wire.NewSet(NewData, NewChallengeServiceClient, NewChallengeClient, NewGrantRepo, NewSiteRepo, NewCounterRepo)

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

func NewChallengeServiceClient(clientConf *conf.Client) (challengev1.ChallengeServiceClient, func(), error) {
	conn, err := grpc.NewClient(context.Background(),
		grpc.WithEndpoint(clientConf.GetChallenge().GetEndpoint()),
		grpc.WithTimeout(clientConf.GetChallenge().GetTimeout().AsDuration()),
	)
	if err != nil {
		return nil, nil, err
	}
	return challengev1.NewChallengeServiceClient(conn), func() { conn.Close() }, nil
}
