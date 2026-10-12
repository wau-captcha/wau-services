package biz

import (
	"context"
	"net/netip"
	"time"

	"challenge/internal/conf"

	"github.com/go-kratos/kratos/v3/log"
)

// HistoryBucket is the window the history of an address is counted in; an hour is six of them.
const HistoryBucket = 10 * time.Minute

// IPHistory keeps, per client address, the wrong answers, the answers judged automated and the
// auto_check passes of the last buckets, in Redis (see data/iphistory.go).
type IPHistory interface {
	// WrongAnswers counts the wrong answers from addr in the bucket of now.
	WrongAnswers(ctx context.Context, addr string, now time.Time) (int64, error)
	// AutomatedLastHour counts the answers from addr judged automated in the six buckets up to now.
	AutomatedLastHour(ctx context.Context, addr string, now time.Time) (int64, error)
	CountWrong(ctx context.Context, addr string, now time.Time) error
	CountAutomated(ctx context.Context, addr string, now time.Time) error
	// AutoPassesLastHour counts the auto_check passes of addr in the six buckets up to now.
	AutoPassesLastHour(ctx context.Context, addr string, now time.Time) (int64, error)
	CountAutoPass(ctx context.Context, addr string, now time.Time) error
}

// ipLimits are the thresholds of conf.IPHistory; 0 turns one off.
type ipLimits struct {
	wrongPerBucket, automatedPerHour int64
}

func newIPLimits(historyConf *conf.IPHistory) ipLimits {
	return ipLimits{
		wrongPerBucket:   int64(historyConf.GetWrongPer_10Minutes()),
		automatedPerHour: int64(historyConf.GetAutomatedPerHour()),
	}
}

// blockedFor is how long addr is refused for too many wrong answers: until the end of the bucket
// once it has more than wrongPerBucket. A failure of Redis lets the request through.
func (uc *ChallengeUsecase) blockedFor(ctx context.Context, addr netip.Addr) time.Duration {
	if uc.ipLimits.wrongPerBucket <= 0 || !addr.IsValid() {
		return 0
	}
	now := uc.now()
	wrong, err := uc.history.WrongAnswers(ctx, addr.String(), now)
	if err != nil {
		log.WarnContext(ctx, "address history not read, request let through", "err", err)
		return 0
	}
	if wrong <= uc.ipLimits.wrongPerBucket {
		return 0
	}
	return now.Truncate(HistoryBucket).Add(HistoryBucket).Sub(now)
}

// badHistory reports whether addr had more answers judged automated in the last hour than
// automatedPerHour.
func (uc *ChallengeUsecase) badHistory(ctx context.Context, addr netip.Addr) bool {
	if uc.ipLimits.automatedPerHour <= 0 || !addr.IsValid() {
		return false
	}
	automated, err := uc.history.AutomatedLastHour(ctx, addr.String(), uc.now())
	if err != nil {
		log.WarnContext(ctx, "address history not read", "err", err)
		return false
	}
	return automated > uc.ipLimits.automatedPerHour
}

// recordOutcome adds an answer from addr to its history: wrong when it counts against the address's
// limit of wrong answers. A failure is logged: the answer has been judged already.
func (uc *ChallengeUsecase) recordOutcome(ctx context.Context, addr netip.Addr, wrong bool, verdict string) {
	if !addr.IsValid() {
		return
	}
	now := uc.now()
	if wrong {
		if err := uc.history.CountWrong(ctx, addr.String(), now); err != nil {
			log.WarnContext(ctx, "wrong answer not counted for its address", "err", err)
		}
	}
	if verdict == VerdictAutomated {
		if err := uc.history.CountAutomated(ctx, addr.String(), now); err != nil {
			log.WarnContext(ctx, "automated answer not counted for its address", "err", err)
		}
	}
}
