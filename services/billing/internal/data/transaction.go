package data

import (
	"context"
	"errors"
	"time"

	"billing/internal/biz"
	"billing/internal/data/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// sepayTransactionKey is the unique index that keeps one SePay payment on one transaction.
const sepayTransactionKey = "billing_transactions_sepay_transaction_id_key"

type transactionRepo struct {
	data *Data
}

func NewTransactionRepo(data *Data) biz.TransactionRepo { return &transactionRepo{data: data} }

func (r *transactionRepo) Create(ctx context.Context, transaction *biz.Transaction) (*biz.Transaction, error) {
	row, err := r.data.query(ctx).CreateTransaction(ctx, db.CreateTransactionParams{
		UserID:      transaction.UserID,
		TierID:      transaction.TierID,
		PaymentCode: transaction.PaymentCode,
		Amount:      int32(transaction.Amount),
		Period:      string(transaction.Period),
		ExpiresAt:   timestamp(transaction.ExpiresAt),
	})
	if err != nil {
		return nil, err
	}
	created := *transaction
	created.ID = row.BillingTransactionID
	created.CreateDate = row.CreateDate.Time
	return &created, nil
}

func (r *transactionRepo) Get(ctx context.Context, userID, id uuid.UUID) (*biz.Transaction, error) {
	row, err := r.data.query(ctx).GetTransaction(ctx, db.GetTransactionParams{BillingTransactionID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, biz.ErrTransactionNotFound
	}
	if err != nil {
		return nil, err
	}
	return toTransaction(userID, db.ListTransactionsRow(row)), nil
}

func (r *transactionRepo) List(ctx context.Context, userID uuid.UUID, limit int) ([]*biz.Transaction, error) {
	rows, err := r.data.query(ctx).ListTransactions(ctx, db.ListTransactionsParams{UserID: userID, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	transactions := make([]*biz.Transaction, 0, len(rows))
	for _, row := range rows {
		transactions = append(transactions, toTransaction(userID, row))
	}
	return transactions, nil
}

func (r *transactionRepo) LockByCode(ctx context.Context, paymentCode string) (*biz.Transaction, error) {
	row, err := r.data.query(ctx).LockTransactionByCode(ctx, paymentCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, biz.ErrUnknownPayment
	}
	if err != nil {
		return nil, err
	}
	return &biz.Transaction{
		ID:                   row.BillingTransactionID,
		UserID:               row.UserID,
		TierID:               row.TierID,
		TierPrice:            int64(row.MonthlyPrice),
		Amount:               int64(row.Amount),
		Period:               biz.Period(row.Period),
		Status:               row.Status,
		PaymentCode:          paymentCode,
		GatewayTransactionID: row.SepayTransactionID.String,
	}, nil
}

func (r *transactionRepo) MarkPaid(ctx context.Context, id uuid.UUID, gatewayTransactionID string, paidAt time.Time) error {
	err := r.data.query(ctx).MarkTransactionPaid(ctx, db.MarkTransactionPaidParams{
		BillingTransactionID: id,
		SepayTransactionID:   gatewayTransactionID,
		PaidAt:               timestamp(paidAt),
	})
	if isUniqueViolation(err, sepayTransactionKey) {
		return biz.ErrPaymentConflict
	}
	return err
}

func (r *transactionRepo) ExpirePending(ctx context.Context) (int64, error) {
	return r.data.query(ctx).ExpireTransactions(ctx)
}

func toTransaction(userID uuid.UUID, row db.ListTransactionsRow) *biz.Transaction {
	return &biz.Transaction{
		ID:          row.BillingTransactionID,
		UserID:      userID,
		TierID:      row.TierID,
		TierName:    row.TierName,
		TierPrice:   int64(row.MonthlyPrice),
		Amount:      int64(row.Amount),
		Period:      biz.Period(row.Period),
		Status:      row.Status,
		PaymentCode: row.PaymentCode,
		CreateDate:  row.CreateDate.Time,
		ExpiresAt:   row.ExpiresAt.Time,
		PaidAt:      timeOf(row.PaidAt),
	}
}
