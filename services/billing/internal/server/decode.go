package server

import (
	"bytes"
	"io"
	stdhttp "net/http"

	"billing/internal/biz"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport/http"
)

// decodeRequest reads the body within the cap the LimitBody filter set, answering a larger one
// with 413 PAYLOAD_TOO_LARGE, then decodes it as Kratos does. An empty body decodes to nothing.
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
	if len(data) == 0 {
		return nil
	}
	return http.DefaultRequestDecoder(r, v)
}
