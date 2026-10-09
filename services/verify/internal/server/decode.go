package server

import (
	"bytes"
	"io"
	stdhttp "net/http"

	v1 "verify/api/verify/v1"
	"verify/internal/biz"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport/http"
)

// decodeRequest reads the body within the cap the LimitBody filter set, answering a larger one
// with 413 PAYLOAD_TOO_LARGE, then decodes it as Kratos does: JSON, or a form for
// application/x-www-form-urlencoded. An empty body decodes to nothing. /siteverify takes its token
// from the body and, for SDKs before 1.3.0 that send none, from ?token=; the body wins when both
// carry one.
func decodeRequest(r *stdhttp.Request, v any) error {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *stdhttp.MaxBytesError
		if errors.As(err, &tooLarge) {
			return biz.ErrPayloadTooLarge
		}
		return errors.BadRequest("CODEC", "the request body could not be read")
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	if len(data) > 0 {
		if err := http.DefaultRequestDecoder(r, v); err != nil {
			return err
		}
	}
	if request, ok := v.(*v1.SiteVerifyRequest); ok && request.GetToken() == "" {
		request.Token = r.URL.Query().Get("token")
	}
	return nil
}
