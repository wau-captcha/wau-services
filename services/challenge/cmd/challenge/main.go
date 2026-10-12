package main

import (
	"context"
	"errors"
	"flag"
	"io/fs"
	"log/slog"
	"os"
	"time"

	"challenge/internal/conf"
	"challenge/internal/pkg/afterresponse"
	_ "challenge/internal/pkg/codec"
	"challenge/internal/pkg/envconf"

	"github.com/go-kratos/kratos/contrib/otel/v3/tracing"
	"github.com/go-kratos/kratos/v3"
	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/transport/grpc"
	"github.com/go-kratos/kratos/v3/transport/http"
	"github.com/joho/godotenv"

	_ "go.uber.org/automaxprocs"
	// The daily quota follows the calendar of quota.time_zone, and debian:stable-slim may have
	// no zone database, so it travels with the binary.
	_ "time/tzdata"
)

// drainTimeout bounds the wait at shutdown for the usage writes still running. Each gives up
// after 10s by itself (afterresponse), well within Kubernetes' default 30s grace period.
const drainTimeout = 15 * time.Second

var (
	Name       string
	Version    string
	configPath string

	hostname, _ = os.Hostname()
)

func init() {
	flag.StringVar(&configPath, "conf", "", "config path, eg: -conf config.yaml; default: the service's configs directory")
}

func newApp(logger *slog.Logger, grpcServer *grpc.Server, httpServer *http.Server) *kratos.App {
	return kratos.New(
		kratos.ID(hostname),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			grpcServer,
			httpServer,
		),
		kratos.AfterStop(drainAfterResponse),
	)
}

// drainAfterResponse lets the usage writes started by the last requests finish once the servers
// have stopped, before main closes Postgres and Redis. Kratos hands AfterStop hooks a context
// that is already cancelled, hence the fresh deadline.
func drainAfterResponse(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), drainTimeout)
	defer cancel()
	if err := afterresponse.Wait(ctx); err != nil {
		log.Warn("usage writes still running at shutdown", "err", err)
	}
	return nil
}

func main() {
	flag.Parse()
	// Load this service's .env; real environment variables take precedence. Both files are
	// found from the service directory, so `kratos run` works from cmd/<name> too.
	if err := godotenv.Load(envconf.DotEnvPath()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		panic(err)
	}
	if configPath == "" {
		configPath = envconf.ConfigDir()
	}
	logger := log.NewLogger(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelInfo,
		}),
		log.WithExtractor(tracing.TraceAttrs),
	).With(
		slog.String("service.id", hostname),
		slog.String("service.name", Name),
		slog.String("service.version", Version),
	)
	log.SetDefault(logger)
	configSource := config.New(
		config.WithSource(file.NewSource(configPath)),
		// ${NAME} placeholders are filled from the environment; the values of rate_limit, scraping,
		// ip_history and auto_check may be replaced from it.
		config.WithResolver(envconf.Resolver(&conf.Bootstrap{}, "rate_limit", "scraping", "ip_history", "auto_check")),
	)
	defer configSource.Close()

	if err := configSource.Load(); err != nil {
		panic(err)
	}

	var bootstrap conf.Bootstrap
	if err := configSource.Scan(&bootstrap); err != nil {
		panic(err)
	}

	app, cleanup, err := wireApp(bootstrap.Server, bootstrap.Data, bootstrap.Challenge, bootstrap.Quota, bootstrap.RateLimit,
		bootstrap.Scraping, bootstrap.IpHistory, bootstrap.AutoCheck, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	if err := app.Run(); err != nil {
		panic(err)
	}
}
