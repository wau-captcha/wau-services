package server

import (
	"fmt"
	"net/http"
	"net/url"

	"gateway/internal/conf"
	"gateway/internal/pkg/clientip"
	"gateway/internal/pkg/envconf"
	"gateway/internal/proxy"
	"gateway/internal/ratelimit"
)

// NewHandler builds the proxy described by the config, wrapped in the rate limit
// when one is configured.
func NewHandler(bootstrap *conf.Bootstrap) (http.Handler, error) {
	routes := make([]proxy.Route, 0, len(bootstrap.GetRoutes()))
	for _, route := range bootstrap.GetRoutes() {
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

	rateLimit := bootstrap.GetRateLimit()
	if len(rateLimit.GetPrefixes()) == 0 || rateLimit.GetRequests() <= 0 || rateLimit.GetWindow().AsDuration() <= 0 {
		return handler, nil
	}
	trustedProxies, err := clientip.ParsePrefixes(envconf.SplitList(rateLimit.GetTrustedProxies()))
	if err != nil {
		return nil, err
	}
	limiter := ratelimit.New(int(rateLimit.GetRequests()), rateLimit.GetWindow().AsDuration())
	return ratelimit.Middleware(limiter, rateLimit.GetPrefixes(), trustedProxies, handler), nil
}
