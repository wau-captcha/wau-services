package main

import (
	"errors"
	"flag"
	"io/fs"
	"log/slog"
	"os"

	"tenant/internal/conf"
	_ "tenant/internal/pkg/codec"
	"tenant/internal/pkg/envconf"

	"github.com/go-kratos/kratos/contrib/otel/v3/tracing"
	"github.com/go-kratos/kratos/v3"
	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/transport/http"
	"github.com/joho/godotenv"

	_ "go.uber.org/automaxprocs"
	// Usage is grouped by local day, so the zone database must travel with the binary.
	_ "time/tzdata"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name is the name of the compiled software.
	Name string
	// Version is the version of the compiled software.
	Version string
	// configPath is the -conf flag: a config file or directory; empty means the service's configs directory.
	configPath string

	hostname, _ = os.Hostname()
)

func init() {
	flag.StringVar(&configPath, "conf", "", "config path, eg: -conf config.yaml; default: the service's configs directory")
}

func newApp(logger *slog.Logger, httpServer *http.Server) *kratos.App {
	return kratos.New(
		kratos.ID(hostname),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(httpServer),
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
		// ${NAME} placeholders are filled from the environment; the file holds no fallback values.
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

	app, cleanup, err := wireApp(bootstrap.Server, bootstrap.Data, bootstrap.Auth, bootstrap.Google, bootstrap.ApiKey, bootstrap.Usage, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		panic(err)
	}
}
