package service

import (
	"context"

	v1 "billing/api/billing/v1"
	"billing/internal/biz"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// BillingService implements the billing API of the signed-in user.
type BillingService struct {
	v1.UnimplementedBillingServiceServer

	billing *biz.BillingUsecase
}

func NewBillingService(billing *biz.BillingUsecase) *BillingService {
	return &BillingService{billing: billing}
}

func (s *BillingService) ListPlans(ctx context.Context, _ *v1.ListPlansRequest) (*v1.PlanSet, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	set, err := s.billing.ListPlans(ctx, userID)
	if err != nil {
		return nil, err
	}
	reply := &v1.PlanSet{
		CurrentTierId:        set.Current.TierID,
		CurrentTierExpiresAt: optionalTime(set.Current.ExpiresAt),
		Plans:                make([]*v1.Plan, 0, len(set.Plans)),
	}
	for _, plan := range set.Plans {
		reply.Plans = append(reply.Plans, &v1.Plan{
			TierId:         plan.TierID,
			TierName:       plan.TierName,
			Description:    plan.Description,
			MonthlyPrice:   plan.MonthlyPrice,
			YearlyPrice:    plan.YearlyPrice,
			DailyQuota:     plan.DailyQuota,
			ChallengeTypes: plan.ChallengeTypes,
			Purchase:       string(plan.Purchase),
		})
	}
	return reply, nil
}

func (s *BillingService) CreateCheckout(ctx context.Context, req *v1.CreateCheckoutRequest) (*v1.Checkout, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	period, err := biz.ParsePeriod(req.GetPeriod())
	if err != nil {
		return nil, err
	}
	checkout, err := s.billing.CreateCheckout(ctx, userID, req.GetTierId(), period)
	if err != nil {
		return nil, err
	}
	reply := &v1.Checkout{
		Transaction: convertTransaction(checkout.Transaction),
		ActionUrl:   checkout.ActionURL,
		Fields:      make([]*v1.FormField, 0, len(checkout.Fields)),
	}
	for _, field := range checkout.Fields {
		reply.Fields = append(reply.Fields, &v1.FormField{Name: field.Name, Value: field.Value})
	}
	return reply, nil
}

func (s *BillingService) ListTransactions(ctx context.Context, _ *v1.ListTransactionsRequest) (*v1.TransactionSet, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	transactions, err := s.billing.ListTransactions(ctx, userID)
	if err != nil {
		return nil, err
	}
	reply := &v1.TransactionSet{Transactions: make([]*v1.Transaction, 0, len(transactions))}
	for _, transaction := range transactions {
		reply.Transactions = append(reply.Transactions, convertTransaction(transaction))
	}
	return reply, nil
}

func (s *BillingService) GetTransaction(ctx context.Context, req *v1.GetTransactionRequest) (*v1.Transaction, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	// An unparseable id is as unknown as another user's transaction.
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, biz.ErrTransactionNotFound
	}
	transaction, err := s.billing.GetTransaction(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	return convertTransaction(transaction), nil
}

func convertTransaction(transaction *biz.Transaction) *v1.Transaction {
	reply := &v1.Transaction{
		Id:                 transaction.ID.String(),
		TierId:             transaction.TierID,
		TierName:           transaction.TierName,
		Amount:             transaction.Amount,
		Period:             string(transaction.Period),
		Status:             transaction.Status,
		PaymentCode:        transaction.PaymentCode,
		CreateDate:         timestamppb.New(transaction.CreateDate),
		ExpiresAt:          timestamppb.New(transaction.ExpiresAt),
		PaidAt:             optionalTime(transaction.PaidAt),
		SepayTransactionId: transaction.GatewayTransactionID,
	}
	if grant := transaction.Grant; grant.Kind != "" {
		reply.Grant = &v1.PlanGrant{
			Kind:     grant.Kind,
			TierId:   grant.TierID,
			TierName: transaction.GrantTierName,
			StartsAt: optionalTime(grant.StartsAt),
			EndsAt:   optionalTime(grant.ExpiresAt),
		}
	}
	return reply
}
