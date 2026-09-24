package server

import (
	stdhttp "net/http"

	"gateway/internal/conf"

	"github.com/go-kratos/kratos/v3/transport/http"
)

// NewHTTPServer answers /healthz itself and proxies everything else.
func NewHTTPServer(serverConf *conf.Server, handler stdhttp.Handler) *http.Server {
	var opts []http.ServerOption
	if serverConf.GetHttp().GetNetwork() != "" {
		opts = append(opts, http.Network(serverConf.GetHttp().GetNetwork()))
	}
	if serverConf.GetHttp().GetAddr() != "" {
		opts = append(opts, http.Address(serverConf.GetHttp().GetAddr()))
	}
	if serverConf.GetHttp().GetTimeout() != nil {
		opts = append(opts, http.Timeout(serverConf.GetHttp().GetTimeout().AsDuration()))
	}
	httpServer := http.NewServer(opts...)
	httpServer.HandleFunc("/healthz", func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	// Registered last so the health route keeps its own path.
	httpServer.HandlePrefix("/", handler)
	return httpServer
}
