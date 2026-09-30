package server

import (
	v1 "billing/api/billing/v1"
	"billing/internal/auth"
	"billing/internal/biz"
	"billing/internal/conf"
	"billing/internal/service"

	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/middleware/validate"
	"github.com/go-kratos/kratos/v3/transport/http"
	"go.einride.tech/aip/fieldbehavior"
	"google.golang.org/protobuf/proto"
)

// NewHTTPServer serves BillingService, every operation of which takes a user token, the SePay
// IPN and /healthz. Kratos middleware does not see the last two: the IPN checks SePay's secret
// key itself.
func NewHTTPServer(
	serverConf *conf.Server,
	tokens *auth.Tokens,
	accounts biz.AccountRepo,
	billing *service.BillingService,
	sepayIPN *service.SepayIPN,
) *http.Server {
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			sanitizeErrors(),
			auth.Middleware(tokens, accounts),
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
	v1.RegisterBillingServiceHTTPServer(httpServer, billing)
	httpServer.Handle(service.SepayIPNPath, sepayIPN)
	httpServer.HandleFunc("/healthz", healthz)
	return httpServer
}
