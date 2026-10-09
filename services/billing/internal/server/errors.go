package server

import (
	"context"
	stdhttp "net/http"

	"billing/internal/biz"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/go-kratos/kratos/v3/transport/http"
)

// sanitizeErrors keeps internal failures out of replies: Kratos sends a plain error with its own
// text, which would show Redis, Postgres or gRPC details. Errors the service meant to send pass;
// anything else, and any 5xx but ErrInternal, becomes ErrInternal and is logged with the
// operation it failed in.
func sanitizeErrors() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			reply, err := handler(ctx, req)
			if err == nil || meant(err) {
				return reply, err
			}
			operation := ""
			if tr, ok := transport.FromServerContext(ctx); ok {
				operation = tr.Operation()
			}
			log.ErrorContext(ctx, "unhandled error", "operation", operation, "err", err)
			return nil, biz.ErrInternal
		}
	}
}

// meant reports whether err is one the service sends on purpose: a Kratos error below 500, or
// ErrInternal itself.
func meant(err error) bool {
	var known *errors.Error
	if !errors.As(err, &known) {
		return false
	}
	return known.Code < stdhttp.StatusInternalServerError || errors.Is(err, biz.ErrInternal)
}

// encodeError is Kratos's error encoder plus a Retry-After header on 429 replies, taken from the
// "retry_after" metadata the rate limits attach: a Kratos error cannot carry headers itself. An
// error that never went through sanitizeErrors, e.g. one of the encoder, is hidden all the same.
func encodeError(w stdhttp.ResponseWriter, r *stdhttp.Request, err error) {
	if !meant(err) {
		log.ErrorContext(r.Context(), "unhandled error", "path", r.URL.Path, "err", err)
		err = biz.ErrInternal
	}
	if known := errors.FromError(err); known.Code == stdhttp.StatusTooManyRequests && known.Metadata["retry_after"] != "" {
		w.Header().Set("Retry-After", known.Metadata["retry_after"])
	}
	http.DefaultErrorEncoder(w, r, err)
}
