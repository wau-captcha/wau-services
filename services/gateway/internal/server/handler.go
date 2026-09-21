package server

import (
	"fmt"
	"net/http"
	"net/url"

	"gateway/internal/conf"
	"gateway/internal/proxy"
	"gateway/internal/ratelimit"

	"common/clientip"
	"common/envconf"
)

// NewHandler builds the proxy described by the config, wrapped in the rate limit
// when one is configured.
func NewHandler(bc *conf.Bootstrap) (http.Handler, error) {
	routes := make([]proxy.Route, 0, len(bc.GetRoutes()))
	for _, route := range bc.GetRoutes() {
		upstream, err := url.Parse(route.GetUpstream())
		if err != nil {
			return nil, fmt.Errorf("route %q: %w", route.GetPrefix(), err)
		}
		if upstream.Scheme == "" || upstream.Host == "" {
			return nil, fmt.Errorf("route %q: upstream must be an absolute URL", route.GetPrefix())
		}
		routes = append(routes, proxy.Route{Prefix: route.GetPrefix(), Upstream: upstream})
	}
	handler, err := proxy.New(routes)
	if err != nil {
		return nil, err
	}

	limit := bc.GetRateLimit()
	if len(limit.GetPrefixes()) == 0 || limit.GetRequests() <= 0 || limit.GetWindow().AsDuration() <= 0 {
		return handler, nil
	}
	trusted, err := clientip.ParsePrefixes(envconf.List(limit.GetTrustedProxies()))
	if err != nil {
		return nil, err
	}
	limiter := ratelimit.New(int(limit.GetRequests()), limit.GetWindow().AsDuration())
	return ratelimit.Middleware(limiter, limit.GetPrefixes(), trusted, handler), nil
}
