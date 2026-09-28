package biz

import (
	"context"
)

// Payment is a payment SePay reports as received.
type Payment struct {
	// The order invoice number: the transaction's payment code.
	PaymentCode string
	// VND received.
	Amount int64
	// SePay's id of the payment.
	GatewayTransactionID string
}

// ConfirmPayment applies a payment in one database transaction: the transaction becomes
// succeeded and the account gets the period paid for, or nothing changes at all. Both rows are locked
// (transaction first, then account, in every caller), so the same payment reported twice at
// once is applied once, and two payments of one account extend each other. It returns nil,
// nil for a payment already applied.
func (uc *BillingUsecase) ConfirmPayment(ctx context.Context, payment Payment) (*Grant, error) {
	var grant *Grant
	err := uc.tx.InTx(ctx, func(ctx context.Context) error {
		grant = nil
		transaction, err := uc.transactions.LockByCode(ctx, payment.PaymentCode)
		if err != nil {
			return err
		}
		if transaction.Status == StatusSucceeded {
			if transaction.GatewayTransactionID == payment.GatewayTransactionID {
				return nil
			}
			return ErrPaymentConflict
		}
		if payment.Amount != transaction.Amount {
			return ErrAmountMismatch
		}
		current, err := uc.accounts.LockPlan(ctx, transaction.UserID)
		if err != nil {
			return err
		}
		now := uc.now()
		applied := uc.calendar.grantFor(current, transaction.TierID, transaction.TierPrice, transaction.Amount,
			transaction.Period.Months(), now)
		if err := uc.accounts.SetPlan(ctx, transaction.UserID, applied.TierID, applied.ExpiresAt); err != nil {
			return err
		}
		if err := uc.transactions.MarkPaid(ctx, transaction.ID, payment.GatewayTransactionID, now); err != nil {
			return err
		}
		grant = &applied
		return nil
	})
	if err != nil {
		return nil, err
	}
	return grant, nil
}

// Sweep moves accounts whose paid tier ended back to free and marks unpaid checkouts expired.
// Every read already treats both that way; the sweep brings the stored rows in line.
func (uc *BillingUsecase) Sweep(ctx context.Context) (endedPlans, expiredTransactions int64, err error) {
	if endedPlans, err = uc.accounts.EndLapsedPlans(ctx); err != nil {
		return 0, 0, err
	}
	expiredTransactions, err = uc.transactions.ExpirePending(ctx)
	return endedPlans, expiredTransactions, err
}
