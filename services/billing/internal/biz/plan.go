package biz

import (
	"fmt"
	"time"

	"billing/internal/conf"
)

// Plan is a tier as it is sold. Tiers are ranked by their monthly price, so a tier sold by the
// year has a monthly price too.
type Plan struct {
	TierID       string
	TierName     string
	Description  string
	MonthlyPrice int64
	// 0 when the tier is not sold by the year.
	YearlyPrice    int64
	DailyQuota     int64
	ChallengeTypes []string
	// Filled for the caller by ListPlans.
	Purchase Purchase
}

// PriceFor is the price of one period of the tier; 0 when it is not sold for that period.
func (p *Plan) PriceFor(period Period) int64 {
	if period == PeriodYear {
		return p.YearlyPrice
	}
	return p.MonthlyPrice
}

// AccountPlan is the tier an account has now; a paid tier past its end already counts as free.
type AccountPlan struct {
	TierID       string
	MonthlyPrice int64
	// Zero when the tier never ends: free, or a tier granted by hand.
	ExpiresAt time.Time
}

// Purchase is what buying a tier, for a month or a year, does for an account.
type Purchase string

const (
	// PurchaseNone: the tier is not sold, is below the account's, or is the account's own
	// tier with no end date.
	PurchaseNone Purchase = ""
	// PurchaseUpgrade: the tier starts now and runs for the period bought. What is left of the
	// current tier is dropped; the new one is paid in full.
	PurchaseUpgrade Purchase = "upgrade"
	// PurchaseRenew: the account's tier ends one period later than it does now.
	PurchaseRenew Purchase = "renew"
)

// PurchaseOf tells what buying the tier priced monthlyPrice a month does for an account on current.
func PurchaseOf(current AccountPlan, tierID string, monthlyPrice int64) Purchase {
	switch {
	case monthlyPrice <= 0:
		return PurchaseNone
	case tierID == current.TierID:
		if current.ExpiresAt.IsZero() {
			return PurchaseNone
		}
		return PurchaseRenew
	case monthlyPrice > current.MonthlyPrice:
		return PurchaseUpgrade
	}
	return PurchaseNone
}

// checkPurchase is the rule a checkout must pass: only renewals and upgrades are sold, and only
// for a period the tier has a price for.
func checkPurchase(current AccountPlan, plan *Plan, period Period) error {
	if plan.PriceFor(period) <= 0 {
		return ErrPlanNotFound
	}
	if PurchaseOf(current, plan.TierID, plan.MonthlyPrice) != PurchaseNone {
		return nil
	}
	if plan.TierID == current.TierID {
		return ErrPlanNotRenewable
	}
	return ErrPlanDowngrade
}

// Calendar counts months in one time zone, so a month bought on the 31st ends on the last day
// of a shorter month instead of spilling into the next one.
type Calendar struct {
	zone *time.Location
}

func NewCalendar(billingConf *conf.Billing) (*Calendar, error) {
	zone, err := time.LoadLocation(billingConf.GetTimeZone())
	if err != nil {
		return nil, fmt.Errorf("billing time zone %q: %w", billingConf.GetTimeZone(), err)
	}
	return &Calendar{zone: zone}, nil
}

// AddMonths returns the same local wall-clock time months later, on the same day of the month or
// the last day of the target month when it is shorter (29 February plus a year is 28 February).
func (c *Calendar) AddMonths(t time.Time, months int) time.Time {
	local := t.In(c.zone)
	year, month, day := local.Date()
	hour, minute, second := local.Clock()
	target := month + time.Month(months)
	if last := time.Date(year, target+1, 0, 0, 0, 0, 0, c.zone).Day(); day > last {
		day = last
	}
	return time.Date(year, target, day, hour, minute, second, local.Nanosecond(), c.zone)
}

// Grant is what a paid period does to an account.
type Grant struct {
	TierID string
	// Zero keeps a tier that never ends.
	ExpiresAt time.Time
	// "upgrade", "renew" or "extend".
	Kind string
	// Where the time paid for starts; it runs to ExpiresAt. Zero when the payment added no time:
	// it went to a tier that never ends or is not sold.
	StartsAt time.Time
}

// grantFor applies months paid of the tier paidTierID (ranked by paidTierPrice, its monthly price),
// which cost amount, to an account on current at now.
//
// A renewal starts at the current end, an upgrade starts now. A payment for a tier below the
// current one can only come from an older checkout paid late; plans are never downgraded, so
// the money buys time on the current tier at its monthly price instead.
func (c *Calendar) grantFor(current AccountPlan, paidTierID string, paidTierPrice, amount int64, months int, now time.Time) Grant {
	start := later(current.ExpiresAt, now)
	switch PurchaseOf(current, paidTierID, paidTierPrice) {
	case PurchaseRenew:
		return Grant{TierID: current.TierID, ExpiresAt: c.AddMonths(start, months), Kind: "renew", StartsAt: start}
	case PurchaseUpgrade:
		return Grant{TierID: paidTierID, ExpiresAt: c.AddMonths(now, months), Kind: "upgrade", StartsAt: now}
	}
	if current.ExpiresAt.IsZero() || current.MonthlyPrice <= 0 {
		// The current tier never ends, or the tier is not sold: there is no time to add.
		return Grant{TierID: current.TierID, ExpiresAt: current.ExpiresAt, Kind: "extend"}
	}
	// Whole months at the current price, then the share of one more month the rest pays for.
	end := c.AddMonths(start, int(amount/current.MonthlyPrice))
	if rest := amount % current.MonthlyPrice; rest > 0 {
		end = end.Add(c.AddMonths(end, 1).Sub(end) / time.Duration(current.MonthlyPrice) * time.Duration(rest))
	}
	return Grant{TierID: current.TierID, ExpiresAt: end, Kind: "extend", StartsAt: start}
}

func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
