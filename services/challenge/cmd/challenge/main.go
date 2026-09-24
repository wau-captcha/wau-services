package main

import (
	"errors"
	"flag"
	"io/fs"
	"log/slog"
	"os"

	"challenge/internal/conf"

	_ "common/codec"
	"common/envconf"

	"github.com/go-kratos/kratos/contrib/otel/v3/tracing"
	"github.com/go-kratos/kratos/v3"
	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/transport/grpc"
	"github.com/go-kratos/kratos/v3/transport/http"
	"github.com/joho/godotenv"

	_ "go.uber.org/automaxprocs"
)

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
	)
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
		config.WithResolver(envconf.Resolve),
	)
	defer configSource.Close()

	if err := configSource.Load(); err != nil {
		panic(err)
	}

	var bootstrap conf.Bootstrap
	if err := configSource.Scan(&bootstrap); err != nil {
		panic(err)
	}

	app, cleanup, err := wireApp(bootstrap.Server, bootstrap.Data, bootstrap.Challenge, bootstrap.Quota, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	if err := app.Run(); err != nil {
		panic(err)
	}
}
