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

// Request body caps. An answer with its behaviour is a few KiB, a slide_puzzle drag up to about
// 9 KiB, animal_jump's two clicks under 1 KiB, object_rotate's presses under 2 KiB; /siteverify
// carries one token.
var bodyLimits = map[string]int64{
	"/challenge/verify": 64 << 10,
	"/siteverify":       4 << 10,
}

const maxBodyBytes = 4 << 10

// NewHTTPServer new an HTTP server.
func NewHTTPServer(serverConf *conf.Server, verify *service.VerifyService) *http.Server {
	var opts = []http.ServerOption{
		http.Filter(
			filter.NoStore,
			// Browsers may call /challenge/verify only; /siteverify is for servers and gets no CORS
			// headers. /challenge/verify names the caller's origin back: the challenge service then
			// refuses an answer from another website than the one the challenge was asked for from.
			filter.CORS(map[string]filter.Route{"/challenge/verify": {Methods: "POST, OPTIONS", ReflectOrigin: true}}),
			filter.LimitBody(bodyLimits, maxBodyBytes),
			// The usage history is written once the response is out.
			afterresponse.Filter,
		),
		http.RequestDecoder(decodeRequest),
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
	v1.RegisterVerifyServiceHTTPServer(httpServer, verify)
	httpServer.HandleFunc("/healthz", healthz)
	return httpServer
}
