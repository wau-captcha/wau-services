package server

import (
	v1 "verify/api/verify/v1"
	"verify/internal/conf"
	"verify/internal/pkg/afterresponse"
	"verify/internal/pkg/filter"
	"verify/internal/service"

	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/middleware/validate"
	"github.com/go-kratos/kratos/v3/transport/http"
	"go.einride.tech/aip/fieldbehavior"
	"google.golang.org/protobuf/proto"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(serverConf *conf.Server, verify *service.VerifyService) *http.Server {
	var opts = []http.ServerOption{
		// The usage history is written once the response is out.
		http.Filter(filter.CORS, afterresponse.Filter),
		http.Middleware(
			recovery.Recovery(),
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
	v1.RegisterVerifyServiceHTTPServer(httpServer, verify)
	httpServer.HandleFunc("/healthz", healthz)
	return httpServer
}
