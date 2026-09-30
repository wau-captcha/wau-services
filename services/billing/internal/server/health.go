package server

import (
	stdhttp "net/http"
)

// healthz answers GET /healthz, the load balancer's and Kubernetes' health check, with 200
// while the process serves HTTP. It does not look at Postgres or Redis: a store that is
// briefly away must not get every replica restarted at once.
func healthz(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}
