package server

import (
	"context"

	"tenant/internal/auth"
	"tenant/internal/conf"
	"tenant/internal/service"

	"common/filter"
	v1 "common/proto/tenant/v1"

	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/middleware/selector"
	"github.com/go-kratos/kratos/v3/middleware/validate"
	"github.com/go-kratos/kratos/v3/transport/http"
	"go.einride.tech/aip/fieldbehavior"
	"google.golang.org/protobuf/proto"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(
	c *conf.Server,
	jwt *auth.JWT,
	authService *service.AuthService,
	apiKeys *service.ApiKeyService,
	usage *service.UsageService,
	catalog *service.CatalogService,
) *http.Server {
	var opts = []http.ServerOption{
		http.Filter(filter.CORS),
		http.Middleware(
			recovery.Recovery(),
			sanitizeErrors(),
			selector.Server(auth.Middleware(jwt)).Match(protected).Build(),
			validate.Validator(func(req any) error {
				if msg, ok := req.(proto.Message); ok {
					return fieldbehavior.ValidateRequiredFields(msg)
				}
				return nil
			}),
		),
	}
	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)
	v1.RegisterAuthServiceHTTPServer(srv, authService)
	v1.RegisterApiKeyServiceHTTPServer(srv, apiKeys)
	v1.RegisterUsageServiceHTTPServer(srv, usage)
	v1.RegisterCatalogServiceHTTPServer(srv, catalog)
	return srv
}

// protected requires an access token everywhere but on registration and login.
func protected(_ context.Context, operation string) bool {
	switch operation {
	case v1.OperationAuthServiceRegister, v1.OperationAuthServiceLogin:
		return false
	default:
		return true
	}
}
