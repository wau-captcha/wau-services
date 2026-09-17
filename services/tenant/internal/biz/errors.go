package biz

import (
	v1 "common/proto/tenant/v1"

	"github.com/go-kratos/kratos/v3/errors"
)

// Errors crossing the API boundary. Reasons mirror v1.ErrorReason so clients can
// branch on them; messages stay generic on purpose.
var (
	ErrInvalidArgument    = errors.BadRequest(v1.ErrorReason_INVALID_ARGUMENT.String(), "invalid argument")
	ErrUsernameTaken      = errors.Conflict(v1.ErrorReason_USERNAME_TAKEN.String(), "username is already taken")
	ErrEmailTaken         = errors.Conflict(v1.ErrorReason_EMAIL_TAKEN.String(), "email is already registered")
	ErrInvalidCredentials = errors.Unauthorized(v1.ErrorReason_INVALID_CREDENTIALS.String(), "identifier or password is wrong")
	ErrUnauthenticated    = errors.Unauthorized(v1.ErrorReason_UNAUTHENTICATED.String(), "access token is missing or invalid")
	ErrApiKeyNotFound     = errors.NotFound(v1.ErrorReason_API_KEY_NOT_FOUND.String(), "api key not found")
	ErrApiKeyLimitReached = errors.Forbidden(v1.ErrorReason_API_KEY_LIMIT_REACHED.String(), "api key limit reached")
	ErrInternal           = errors.InternalServer(v1.ErrorReason_INTERNAL.String(), "internal error")
)

// InvalidField names the input that broke a validation rule. It matches
// ErrInvalidArgument under errors.Is, so callers can test for either.
func InvalidField(field, message string) error {
	return errors.BadRequest(v1.ErrorReason_INVALID_ARGUMENT.String(), message).
		WithMetadata(map[string]string{"field": field})
}
