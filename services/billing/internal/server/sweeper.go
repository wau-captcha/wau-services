package server

import (
	"context"
	"fmt"
	"time"

	"billing/internal/biz"
	"billing/internal/conf"

	"github.com/go-kratos/kratos/v3/log"
)

// Sweeper periodically moves accounts whose paid tier ended back to free and expires unpaid
// checkouts. Reads never wait for it: every query already treats both that way.
type Sweeper struct {
	billing  *biz.BillingUsecase
	interval time.Duration
	stop     chan struct{}
}

func NewSweeper(billing *biz.BillingUsecase, billingConf *conf.Billing) (*Sweeper, error) {
	interval := billingConf.GetSweepInterval().AsDuration()
	if interval <= 0 {
		return nil, fmt.Errorf("billing sweep_interval must be positive")
	}
	return &Sweeper{billing: billing, interval: interval, stop: make(chan struct{})}, nil
}

// Start sweeps at once, then every interval until Stop.
func (s *Sweeper) Start(ctx context.Context) error {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		s.sweep(ctx)
		select {
		case <-ticker.C:
		case <-s.stop:
			return nil
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *Sweeper) Stop(context.Context) error {
	close(s.stop)
	return nil
}

func (s *Sweeper) sweep(ctx context.Context) {
	sweepCtx, cancel := context.WithTimeout(ctx, s.interval)
	defer cancel()
	endedPlans, expiredTransactions, err := s.billing.Sweep(sweepCtx)
	if err != nil {
		log.ErrorContext(ctx, "billing sweep failed", "err", err)
		return
	}
	if endedPlans > 0 || expiredTransactions > 0 {
		log.InfoContext(ctx, "billing sweep", "ended_plans", endedPlans, "expired_transactions", expiredTransactions)
	}
}
