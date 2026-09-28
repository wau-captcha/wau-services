package biz

import (
	"context"
	"fmt"
	"time"

	"billing/internal/conf"

	"github.com/google/uuid"
)

// transactionListLimit is how many transactions ListTransactions returns.
const transactionListLimit = 50

// AccountStatus is what the account an access token names is now.
type AccountStatus struct {
	Exists bool
	Banned bool
}

// AccountRepo reads and changes the tier of a user's account.
type AccountRepo interface {
	Status(ctx context.Context, userID uuid.UUID) (AccountStatus, error)
	Plan(ctx context.Context, userID uuid.UUID) (AccountPlan, error)
	// LockPlan is Plan, locking the account until the database transaction of ctx ends.
	LockPlan(ctx context.Context, userID uuid.UUID) (AccountPlan, error)
	// SetPlan stores the tier; a zero expiresAt means it never ends.
	SetPlan(ctx context.Context, userID uuid.UUID, tierID string, expiresAt time.Time) error
	// EndLapsedPlans moves accounts whose paid tier has ended back to free and returns how many.
	EndLapsedPlans(ctx context.Context) (int64, error)
}

// PlanRepo reads the tiers on sale.
type PlanRepo interface {
	// List returns the active tiers, cheapest first.
	List(ctx context.Context) ([]*Plan, error)
	// Get returns an active tier, or ErrPlanNotFound.
	Get(ctx context.Context, tierID string) (*Plan, error)
}

// TxRunner runs fn in one database transaction: repos called with the ctx it passes take part
// in it, and it commits only when fn returns nil.
type TxRunner interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// FormField is one field of the checkout form, sent in order.
type FormField struct {
	Name  string
	Value string
}

// Checkout is a pending transaction and the signed form that takes the payer to pay it.
type Checkout struct {
	Transaction *Transaction
	ActionURL   string
	Fields      []FormField
}

// PaymentGateway builds the form the browser posts to the payment page.
type PaymentGateway interface {
	CheckoutForm(transaction *Transaction) (actionURL string, fields []FormField)
}

// PlanSet is the tiers on sale as the caller sees them.
type PlanSet struct {
	Plans   []*Plan
	Current AccountPlan
}

// BillingUsecase sells tiers by the month or the year and applies the payments SePay reports.
type BillingUsecase struct {
	plans        PlanRepo
	accounts     AccountRepo
	transactions TransactionRepo
	gateway      PaymentGateway
	tx           TxRunner
	calendar     *Calendar
	checkoutTTL  time.Duration
	now          func() time.Time
}

func NewBillingUsecase(plans PlanRepo, accounts AccountRepo, transactions TransactionRepo, gateway PaymentGateway,
	tx TxRunner, calendar *Calendar, billingConf *conf.Billing) (*BillingUsecase, error) {
	checkoutTTL := billingConf.GetCheckoutTtl().AsDuration()
	if checkoutTTL <= 0 {
		return nil, fmt.Errorf("billing checkout_ttl must be positive")
	}
	return &BillingUsecase{plans: plans, accounts: accounts, transactions: transactions, gateway: gateway,
		tx: tx, calendar: calendar, checkoutTTL: checkoutTTL, now: time.Now}, nil
}

// ListPlans returns the active tiers and what buying each would do for the user.
func (uc *BillingUsecase) ListPlans(ctx context.Context, userID uuid.UUID) (*PlanSet, error) {
	plans, err := uc.plans.List(ctx)
	if err != nil {
		return nil, err
	}
	current, err := uc.accounts.Plan(ctx, userID)
	if err != nil {
		return nil, err
	}
	for _, plan := range plans {
		plan.Purchase = PurchaseOf(current, plan.TierID, plan.MonthlyPrice)
	}
	return &PlanSet{Plans: plans, Current: current}, nil
}

// CreateCheckout opens a pending transaction for one period of tierID at today's price. Whether
// the purchase is allowed is checked again when the payment arrives, against the tier then.
func (uc *BillingUsecase) CreateCheckout(ctx context.Context, userID uuid.UUID, tierID string, period Period) (*Checkout, error) {
	plan, err := uc.plans.Get(ctx, tierID)
	if err != nil {
		return nil, err
	}
	current, err := uc.accounts.Plan(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := checkPurchase(current, plan, period); err != nil {
		return nil, err
	}
	code, err := newPaymentCode()
	if err != nil {
		return nil, err
	}
	transaction, err := uc.transactions.Create(ctx, &Transaction{
		UserID:      userID,
		TierID:      plan.TierID,
		TierName:    plan.TierName,
		TierPrice:   plan.MonthlyPrice,
		Amount:      plan.PriceFor(period),
		Period:      period,
		Status:      StatusPending,
		PaymentCode: code,
		ExpiresAt:   uc.now().Add(uc.checkoutTTL),
	})
	if err != nil {
		return nil, err
	}
	actionURL, fields := uc.gateway.CheckoutForm(transaction)
	return &Checkout{Transaction: transaction, ActionURL: actionURL, Fields: fields}, nil
}

func (uc *BillingUsecase) ListTransactions(ctx context.Context, userID uuid.UUID) ([]*Transaction, error) {
	return uc.transactions.List(ctx, userID, transactionListLimit)
}

func (uc *BillingUsecase) GetTransaction(ctx context.Context, userID, id uuid.UUID) (*Transaction, error) {
	return uc.transactions.Get(ctx, userID, id)
}
