package data

import (
	"context"
	"errors"
	"time"

	"billing/internal/biz"
	"billing/internal/conf"
	"billing/internal/data/db"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/google/wire"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(NewData, NewAccountRepo, NewPlanRepo, NewTransactionRepo, NewPaymentGateway,
	wire.Bind(new(biz.TxRunner), new(*Data)))

// uniqueViolation is the PostgreSQL SQLSTATE for a duplicate key.
const uniqueViolation = "23505"

type Data struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewData(dataConf *conf.Data) (*Data, func(), error) {
	pool, err := pgxpool.New(context.Background(), dataConf.GetDatabase().GetSource())
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		log.Info("closing the data resources")
		pool.Close()
	}
	return &Data{pool: pool, queries: db.New(pool)}, cleanup, nil
}

type txKey struct{}

// InTx runs fn in one database transaction, committed only when fn returns nil. Row locks
// taken inside hold until then; READ COMMITTED is enough because every row a decision depends
// on is read with FOR UPDATE.
func (d *Data) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return pgx.BeginFunc(ctx, d.pool, func(tx pgx.Tx) error {
		return fn(context.WithValue(ctx, txKey{}, d.queries.WithTx(tx)))
	})
}

// query returns the queries of the database transaction in ctx, or of the pool outside one.
func (d *Data) query(ctx context.Context) *db.Queries {
	if queries, ok := ctx.Value(txKey{}).(*db.Queries); ok {
		return queries
	}
	return d.queries
}

func timestamp(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

// timeOf is the zero time for NULL.
func timeOf(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation && pgErr.ConstraintName == constraint
}
