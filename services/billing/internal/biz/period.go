package biz

// Period is how long one purchase extends a tier, as stored in billing_transactions.period.
type Period string

const (
	PeriodMonth Period = "month"
	PeriodYear  Period = "year"
)

// ParsePeriod reads the period of a checkout; empty means a month.
func ParsePeriod(value string) (Period, error) {
	switch Period(value) {
	case "", PeriodMonth:
		return PeriodMonth, nil
	case PeriodYear:
		return PeriodYear, nil
	}
	return "", InvalidField("period", `period must be "month" or "year"`)
}

// Months is the length of the period in calendar months.
func (p Period) Months() int {
	if p == PeriodYear {
		return 12
	}
	return 1
}
