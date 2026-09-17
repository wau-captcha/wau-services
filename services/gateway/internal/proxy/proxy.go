// Package proxy forwards each request to the service that owns its path prefix.
package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strings"

	"gateway/internal/httpx"

	"github.com/go-kratos/kratos/v3/log"
)

// Route maps a path prefix onto an upstream service.
type Route struct {
	Prefix   string
	Upstream *url.URL
}

type upstreamKey struct{}

type proxy struct {
	// routes are sorted longest prefix first, so /v1/auth/ wins over /v1/.
	routes  []Route
	reverse *httputil.ReverseProxy
}

// New builds a handler forwarding by longest matching prefix.
func New(routes []Route) (http.Handler, error) {
	if len(routes) == 0 {
		return nil, fmt.Errorf("at least one route is required")
	}
	sorted := make([]Route, len(routes))
	copy(sorted, routes)
	for _, route := range sorted {
		if route.Prefix == "" || route.Upstream == nil {
			return nil, fmt.Errorf("route needs a prefix and an upstream")
		}
	}
	sort.SliceStable(sorted, func(i, j int) bool { return len(sorted[i].Prefix) > len(sorted[j].Prefix) })

	p := &proxy{routes: sorted}
	p.reverse = &httputil.ReverseProxy{Rewrite: rewrite, ErrorHandler: upstreamFailed}
	return p, nil
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route, ok := p.match(r.URL.Path)
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "ROUTE_NOT_FOUND", "no service handles this path")
		return
	}
	p.reverse.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), upstreamKey{}, route.Upstream)))
}

func (p *proxy) match(path string) (Route, bool) {
	for _, route := range p.routes {
		if strings.HasPrefix(path, route.Prefix) {
			return route, true
		}
	}
	return Route{}, false
}

func rewrite(r *httputil.ProxyRequest) {
	upstream, _ := r.In.Context().Value(upstreamKey{}).(*url.URL)
	r.SetURL(upstream)
	// SetURL would join the upstream path with ours; services own their routes,
	// so the incoming path is forwarded unchanged.
	r.Out.URL.Path = r.In.URL.Path
	r.Out.URL.RawPath = r.In.URL.RawPath
	r.SetXForwarded()
}

func upstreamFailed(w http.ResponseWriter, r *http.Request, err error) {
	log.Error("upstream request failed", "path", r.URL.Path, "err", err)
	httpx.WriteError(w, http.StatusBadGateway, "UPSTREAM_UNAVAILABLE", "the service is unavailable, try again later")
}
