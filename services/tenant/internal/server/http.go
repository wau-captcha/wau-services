package server

import (
	"context"
	"strings"

	"tenant/internal/auth"
	"tenant/internal/biz"
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
	users biz.UserRepo,
	admins biz.AdminAccountRepo,
	authService *service.AuthService,
	apiKeys *service.ApiKeyService,
	usage *service.UsageService,
	catalog *service.CatalogService,
	admin *service.AdminService,
) *http.Server {
	var opts = []http.ServerOption{
		http.Filter(filter.CORS),
		http.Middleware(
			recovery.Recovery(),
			sanitizeErrors(),
			selector.Server(auth.Middleware(jwt, users)).Match(userOperation).Build(),
			selector.Server(auth.AdminMiddleware(jwt, admins)).Prefix(adminOperations).Build(),
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
	v1.RegisterAdminServiceHTTPServer(srv, admin)
	return srv
}

// adminOperations prefixes every AdminService operation; they take administrator tokens.
const adminOperations = "/tenant.v1.AdminService/"

// signIn operations hand out tokens, so they need none.
var signIn = map[string]bool{
	v1.OperationAuthServiceSignInWithGoogle:      true,
	v1.OperationAuthServiceSignInAdminWithGoogle: true,
}

// userOperation requires a user's token everywhere but on sign-in and in AdminService.
func userOperation(_ context.Context, operation string) bool {
	return !signIn[operation] && !strings.HasPrefix(operation, adminOperations)
}
