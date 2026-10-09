package server

import (
	v1 "challenge/api/challenge/v1"
	"challenge/internal/conf"
	"challenge/internal/pkg/afterresponse"
	"challenge/internal/pkg/filter"
	"challenge/internal/service"

	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/middleware/validate"
	"github.com/go-kratos/kratos/v3/transport/http"
	"go.einride.tech/aip/fieldbehavior"
	"google.golang.org/protobuf/proto"
)

// maxBodyBytes caps request bodies: /challenge/new takes none.
const maxBodyBytes = 4 << 10

// NewHTTPServer new an HTTP server.
func NewHTTPServer(serverConf *conf.Server, challenge *service.ChallengeService) *http.Server {
	var opts = []http.ServerOption{
		http.Filter(
			filter.NoStore,
			// Access-Control-Allow-Origin is "*" here; for a key bound to its domains NewChallenge
			// names the calling origin instead.
			filter.CORS(map[string]filter.Route{"/challenge/new": {Methods: "GET, OPTIONS"}}),
			filter.LimitBody(nil, maxBodyBytes),
			// The usage history is written once the response is out.
			afterresponse.Filter,
		),
		http.ErrorEncoder(encodeError),
		http.Middleware(
			recovery.Recovery(),
			sanitizeErrors(),
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
	v1.RegisterChallengeServiceHTTPServer(httpServer, challenge)
	httpServer.HandleFunc("/healthz", healthz)
	return httpServer
}
