package data

import (
	"context"

	"tenant/internal/conf"
	"tenant/internal/data/db"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/google/wire"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewUserRepo, NewApiKeyRepo, NewUsageRepo, NewCatalogRepo)

type Data struct {
	queries *db.Queries
}

func NewData(c *conf.Data) (*Data, func(), error) {
	pool, err := pgxpool.New(context.Background(), c.GetDatabase().GetSource())
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		log.Info("closing the data resources")
		pool.Close()
	}
	return &Data{queries: db.New(pool)}, cleanup, nil
}
