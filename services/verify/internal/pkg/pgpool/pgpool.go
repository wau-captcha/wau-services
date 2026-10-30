// Package pgpool opens the Postgres pool of a service so that the service holds no connection
// while it is idle. A database that pauses once nobody is connected, as Aurora Serverless v2
// does with a minimum of 0 ACUs, can then pause, and the next query wakes it up.
package pgpool

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// An idle connection is closed by the pool's health check, which runs every minute, once it
	// has been idle this long: within two minutes of its last use.
	maxConnIdleTime = time.Minute
	// Aurora holds a connection until a paused instance has resumed: about 15 s, 30 s or more
	// after a day asleep. A connect_timeout in the source can only make it longer.
	minConnectTimeout = 30 * time.Second
	// A failed connect is tried again this many times, first after retryDelay and then after
	// twice the previous wait, while the context allows.
	connectRetries = 3
	retryDelay     = time.Second
)

// Pool is the pool the queries use. Its Exec, Query, QueryRow and Begin first make sure it holds
// a connection, retrying a connect that failed; a statement itself is never sent twice.
type Pool struct {
	*pgxpool.Pool
}

// New opens a pool for source, a Postgres connection string, without connecting yet: none is
// opened in advance or kept idle, whatever the source says. applicationName names the
// service's connections in pg_stat_activity.
func New(source, applicationName string) (*Pool, error) {
	config, err := pgxpool.ParseConfig(source)
	if err != nil {
		return nil, err
	}
	config.MinConns = 0
	config.MinIdleConns = 0
	config.MaxConnIdleTime = maxConnIdleTime
	config.ConnConfig.ConnectTimeout = max(config.ConnConfig.ConnectTimeout, minConnectTimeout)
	config.ConnConfig.RuntimeParams["application_name"] = applicationName
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}
	return &Pool{pool}, nil
}

func (p *Pool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if err := p.connect(ctx); err != nil {
		return pgconn.CommandTag{}, err
	}
	return p.Pool.Exec(ctx, sql, args...)
}

func (p *Pool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if err := p.connect(ctx); err != nil {
		return nil, err
	}
	return p.Pool.Query(ctx, sql, args...)
}

func (p *Pool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if err := p.connect(ctx); err != nil {
		return errRow{err}
	}
	return p.Pool.QueryRow(ctx, sql, args...)
}

func (p *Pool) Begin(ctx context.Context) (pgx.Tx, error) {
	if err := p.connect(ctx); err != nil {
		return nil, err
	}
	return p.Pool.Begin(ctx)
}

// connect returns at once while the pool holds a connection, idle, in use or being opened: the
// database is awake or waking up. Otherwise it opens one and leaves it idle for the statement,
// trying again when the connect fails. A connect keeps going after ctx ends, so a later query
// may find the connection ready.
func (p *Pool) connect(ctx context.Context) error {
	if p.Stat().TotalConns() > 0 {
		return nil
	}
	delay := retryDelay
	for retry := 0; ; retry++ {
		conn, err := p.Acquire(ctx)
		if err == nil {
			conn.Release()
			return nil
		}
		if retry == connectRetries || !retryable(err) {
			return err
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return err
		case <-timer.C:
		}
		delay *= 2
	}
}

// retryable tells a connect that failed, which another try may get past, from one refused for
// its password, database or privilege to connect, which no retry changes.
func retryable(err error) bool {
	var connectErr *pgconn.ConnectError
	if !errors.As(err, &connectErr) {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "28P01", "28000", "3D000", "42501":
			return false
		}
	}
	return true
}

// errRow is the row of a QueryRow that never ran: Scan returns why.
type errRow struct{ err error }

func (r errRow) Scan(...any) error { return r.err }
