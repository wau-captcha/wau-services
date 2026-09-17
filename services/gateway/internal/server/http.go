package server

import (
	stdhttp "net/http"

	"gateway/internal/conf"

	"github.com/go-kratos/kratos/v3/transport/http"
)

// NewHTTPServer answers /healthz itself and proxies everything else.
func NewHTTPServer(c *conf.Server, handler stdhttp.Handler) *http.Server {
	var opts []http.ServerOption
	if c.GetHttp().GetNetwork() != "" {
		opts = append(opts, http.Network(c.GetHttp().GetNetwork()))
	}
	if c.GetHttp().GetAddr() != "" {
		opts = append(opts, http.Address(c.GetHttp().GetAddr()))
	}
	if c.GetHttp().GetTimeout() != nil {
		opts = append(opts, http.Timeout(c.GetHttp().GetTimeout().AsDuration()))
	}
	srv := http.NewServer(opts...)
	srv.HandleFunc("/healthz", func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	// Registered last so the health route keeps its own path.
	srv.HandlePrefix("/", handler)
	return srv
}
