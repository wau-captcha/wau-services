package biz

import (
	stderrors "errors"

	v1 "billing/api/billing/v1"

	"github.com/go-kratos/kratos/v3/errors"
)

// Errors crossing the API boundary. Reasons mirror v1.ErrorReason so clients can branch on
// them; UNAUTHENTICATED and ACCOUNT_BANNED mean what they mean in wau-tenant.
var (
	ErrInvalidArgument     = errors.BadRequest(v1.ErrorReason_INVALID_ARGUMENT.String(), "invalid argument")
	ErrUnauthenticated     = errors.Unauthorized(v1.ErrorReason_UNAUTHENTICATED.String(), "access token is missing or invalid")
	ErrAccountBanned       = errors.Forbidden(v1.ErrorReason_ACCOUNT_BANNED.String(), "account is banned")
	ErrInternal            = errors.InternalServer(v1.ErrorReason_INTERNAL.String(), "internal error")
	ErrPlanNotFound        = errors.NotFound(v1.ErrorReason_PLAN_NOT_FOUND.String(), "plan not found")
	ErrPlanDowngrade       = errors.Conflict(v1.ErrorReason_PLAN_DOWNGRADE_NOT_ALLOWED.String(), "a plan can only be renewed or upgraded")
	ErrPlanNotRenewable    = errors.Conflict(v1.ErrorReason_PLAN_NOT_RENEWABLE.String(), "the current plan never ends")
	ErrTransactionNotFound = errors.NotFound(v1.ErrorReason_TRANSACTION_NOT_FOUND.String(), "transaction not found")
)

// InvalidField names the input that broke a validation rule. It matches ErrInvalidArgument under
// errors.Is, so callers can test for either.
func InvalidField(field, message string) error {
	return errors.BadRequest(v1.ErrorReason_INVALID_ARGUMENT.String(), message).
		WithMetadata(map[string]string{"field": field})
}

// Payments SePay reports that cannot be applied. Retrying the report does not change them, so
// the notification is acknowledged and the payment is left for a person to settle.
var (
	ErrUnknownPayment = stderrors.New("no transaction has this payment code")
	ErrAmountMismatch = stderrors.New("paid amount differs from the transaction amount")
	// The transaction was paid by another SePay transaction, or this SePay transaction paid
	// another transaction.
	ErrPaymentConflict = stderrors.New("payment conflicts with one already applied")
)
