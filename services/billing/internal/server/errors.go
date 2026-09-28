package server

import (
	"context"

	"billing/internal/biz"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/middleware"
)

// sanitizeErrors keeps unexpected failures out of responses: Kratos encodes any
// plain error with its own text, which would leak driver and query details.
func sanitizeErrors() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			reply, err := handler(ctx, req)
			if err == nil {
				return reply, nil
			}
			var known *errors.Error
			if !errors.As(err, &known) {
				log.ErrorContext(ctx, "unhandled error", "err", err)
				return nil, biz.ErrInternal
			}
			return reply, err
		}
	}
}
