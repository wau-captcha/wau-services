package server

import (
	"context"
	"strings"

	v1 "tenant/api/tenant/v1"
	"tenant/internal/auth"
	"tenant/internal/biz"
	"tenant/internal/conf"
	"tenant/internal/service"

	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/middleware/selector"
	"github.com/go-kratos/kratos/v3/middleware/validate"
	"github.com/go-kratos/kratos/v3/transport/http"
	"go.einride.tech/aip/fieldbehavior"
	"google.golang.org/protobuf/proto"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(
	serverConf *conf.Server,
	tokens *auth.JWT,
	users biz.UserRepo,
	admins biz.AdminAccountRepo,
	authService *service.AuthService,
	apiKeys *service.ApiKeyService,
	usage *service.UsageService,
	catalog *service.CatalogService,
	admin *service.AdminService,
) *http.Server {
	// No CORS filter: only the dashboard's server calls tenant, never a browser.
	var opts = []http.ServerOption{
		http.ErrorEncoder(encodeError),
		http.Middleware(
			recovery.Recovery(),
			sanitizeErrors(),
			selector.Server(auth.Middleware(tokens, users)).Match(userOperation).Build(),
			selector.Server(auth.AdminMiddleware(tokens, admins)).Prefix(adminOperations).Build(),
			validate.Validator(func(req any) error {
				if message, ok := req.(proto.Message); ok {
					return fieldbehavior.ValidateRequiredFields(message)
				}
				return nil
			}),
		),
	}
	if serverConf.Http.Network != "" {
		opts = append(opts, http.Network(serverConf.Http.Network))
	}
	if serverConf.Http.Addr != "" {
		opts = append(opts, http.Address(serverConf.Http.Addr))
	}
	if serverConf.Http.Timeout != nil {
		opts = append(opts, http.Timeout(serverConf.Http.Timeout.AsDuration()))
	}
	httpServer := http.NewServer(opts...)
	v1.RegisterAuthServiceHTTPServer(httpServer, authService)
	v1.RegisterApiKeyServiceHTTPServer(httpServer, apiKeys)
	v1.RegisterUsageServiceHTTPServer(httpServer, usage)
	v1.RegisterCatalogServiceHTTPServer(httpServer, catalog)
	v1.RegisterAdminServiceHTTPServer(httpServer, admin)
	httpServer.HandleFunc("/healthz", healthz)
	return httpServer
}

// adminOperations prefixes every AdminService operation; they take administrator tokens.
const adminOperations = "/tenant.v1.AdminService/"

// public operations need no token: sign-in hands tokens out, and the plans are for anyone.
var public = map[string]bool{
	v1.OperationAuthServiceSignInWithGoogle:      true,
	v1.OperationAuthServiceSignInAdminWithGoogle: true,
	v1.OperationCatalogServiceListPublicPlans:    true,
}

// userOperation requires a user's token everywhere but on the public operations and in AdminService.
func userOperation(_ context.Context, operation string) bool {
	return !public[operation] && !strings.HasPrefix(operation, adminOperations)
}
