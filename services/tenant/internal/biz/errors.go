package biz

import (
	v1 "tenant/api/tenant/v1"

	"github.com/go-kratos/kratos/v3/errors"
)

// Errors crossing the API boundary. Reasons mirror v1.ErrorReason so clients can
// branch on them; messages stay generic on purpose. Google sign-in errors live in google.go.
var (
	ErrInvalidArgument    = errors.BadRequest(v1.ErrorReason_INVALID_ARGUMENT.String(), "invalid argument")
	ErrUnauthenticated    = errors.Unauthorized(v1.ErrorReason_UNAUTHENTICATED.String(), "access token is missing or invalid")
	ErrApiKeyNotFound     = errors.NotFound(v1.ErrorReason_API_KEY_NOT_FOUND.String(), "api key not found")
	ErrApiKeyLimitReached = errors.Forbidden(v1.ErrorReason_API_KEY_LIMIT_REACHED.String(), "api key limit reached")
	ErrInternal           = errors.InternalServer(v1.ErrorReason_INTERNAL.String(), "internal error")
	ErrAccountBanned      = errors.Forbidden(v1.ErrorReason_ACCOUNT_BANNED.String(), "account is banned")
	ErrNotAdmin           = errors.Forbidden(v1.ErrorReason_PERMISSION_DENIED.String(), "google account is not an administrator")
	ErrNoSuchUser         = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
)

// InvalidField names the input that broke a validation rule. It matches
// ErrInvalidArgument under errors.Is, so callers can test for either.
func InvalidField(field, message string) error {
	return errors.BadRequest(v1.ErrorReason_INVALID_ARGUMENT.String(), message).
		WithMetadata(map[string]string{"field": field})
}
