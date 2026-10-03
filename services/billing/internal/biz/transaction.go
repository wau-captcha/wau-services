package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Transaction statuses, as stored in billing_transactions.
const (
	StatusPending   = "pending"
	StatusSucceeded = "succeeded"
	// Not paid before ExpiresAt. SePay may still report it paid later, and it then succeeds.
	StatusExpired = "expired"
)

// Transaction is one month or one year of a tier, bought or being bought.
type Transaction struct {
	ID       uuid.UUID
	UserID   uuid.UUID
	TierID   string
	TierName string
	// Monthly price of the tier now, which ranks it against the account's tier.
	TierPrice int64
	// VND, fixed at checkout.
	Amount int64
	// How long the payment extends the tier.
	Period Period
	Status string
	// The order invoice number SePay knows the transaction by.
	PaymentCode string
	// The SePay transaction that paid it; empty until then.
	GatewayTransactionID string
	CreateDate           time.Time
	ExpiresAt            time.Time
	// Zero until paid.
	PaidAt time.Time
	// What applying the payment did to the account; zero Kind until then, and for payments
	// applied before wau-billing recorded it.
	Grant Grant
	// Name of Grant.TierID.
	GrantTierName string
}

// TransactionRepo stores transactions. Get and List only return the user's own.
type TransactionRepo interface {
	Create(ctx context.Context, transaction *Transaction) (*Transaction, error)
	Get(ctx context.Context, userID, id uuid.UUID) (*Transaction, error)
	// List returns the user's latest transactions, newest first.
	List(ctx context.Context, userID uuid.UUID, limit int) ([]*Transaction, error)
	// LockByCode reads the transaction with this payment code and locks it until the database
	// transaction of ctx ends; ErrUnknownPayment when there is none.
	LockByCode(ctx context.Context, paymentCode string) (*Transaction, error)
	// MarkPaid records the payment and what applying it did to the account.
	MarkPaid(ctx context.Context, id uuid.UUID, gatewayTransactionID string, paidAt time.Time, grant Grant) error
	// ExpirePending marks pending transactions past their expiry and returns how many.
	ExpirePending(ctx context.Context) (int64, error)
}

// paymentCodePrefix starts every order invoice number, so WAU orders stand out in SePay.
const paymentCodePrefix = "WAU"

// newPaymentCode returns a unique order invoice number: SePay takes letters, digits, "-" and
// "_", up to 100 characters. The unique index on payment_code backs the 64 random bits.
func newPaymentCode() (string, error) {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return paymentCodePrefix + strings.ToUpper(hex.EncodeToString(random)), nil
}
