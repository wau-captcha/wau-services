package biz

import (
	"context"
	"time"

	"challenge/internal/conf"
	"challenge/internal/pkg/quota"

	"github.com/go-kratos/kratos/v3/log"
)

// CounterRepo keeps the daily counters shared with the verify service (see internal/pkg/quota).
type CounterRepo interface {
	// QuotaUsed returns the successful verifications of a user on a day, 0 when none.
	QuotaUsed(ctx context.Context, userID, day string) (int64, error)
	// CountChallenge records a challenge issued for an API key on a day and returns the counters of
	// the key on that day, this challenge included.
	CountChallenge(ctx context.Context, apiKeyID, day string) (DailyCounts, error)
}

// DailyCounts are what the scraping check counted for one key on one day: challenges issued,
// answers checked, right answers and answers judged automated.
type DailyCounts struct {
	Issued, Answered, Solved, Automated int64
}

// scrapingRules are the thresholds of conf.Scraping; a threshold of 0 turns its rule off.
type scrapingRules struct {
	flagMinIssued, flagMaxAnsweredPercent                          int64
	flagMinAnswered, flagMaxSolvedPercent, flagMaxAutomatedPercent int64
	suspendMinIssued, suspendMaxAnsweredPercent                    int64
}

func newScrapingRules(scrapingConf *conf.Scraping) scrapingRules {
	return scrapingRules{
		flagMinIssued:             int64(scrapingConf.GetFlagMinIssued()),
		flagMaxAnsweredPercent:    int64(scrapingConf.GetFlagMaxAnsweredPercent()),
		flagMinAnswered:           int64(scrapingConf.GetFlagMinAnswered()),
		flagMaxSolvedPercent:      int64(scrapingConf.GetFlagMaxSolvedPercent()),
		flagMaxAutomatedPercent:   int64(scrapingConf.GetFlagMaxAutomatedPercent()),
		suspendMinIssued:          int64(scrapingConf.GetSuspendMinIssued()),
		suspendMaxAnsweredPercent: int64(scrapingConf.GetSuspendMaxAnsweredPercent()),
	}
}

type scrapingVerdict int

const (
	scrapingNone scrapingVerdict = iota
	// Throttled for the rest of the day: a single-page app that fetches ahead, or a scraper
	// padding its ratio with junk answers, gets slowed down, not cut off.
	scrapingFlag
	// Only a key that collects thousands of challenges a day and answers almost none is cut off.
	scrapingSuspend
)

// judge grades the counters of a key. Most scrapers harvest the images and never answer; some pad
// their ratio with answers that are wrong or automated.
func (r scrapingRules) judge(c DailyCounts) scrapingVerdict {
	switch {
	case r.suspendMinIssued > 0 && c.Issued >= r.suspendMinIssued && c.Answered*100 < c.Issued*r.suspendMaxAnsweredPercent:
		return scrapingSuspend
	case r.flagMinIssued > 0 && c.Issued >= r.flagMinIssued && c.Answered*100 < c.Issued*r.flagMaxAnsweredPercent:
		return scrapingFlag
	case r.flagMinAnswered > 0 && c.Answered >= r.flagMinAnswered &&
		(c.Solved*100 < c.Answered*r.flagMaxSolvedPercent || c.Automated*100 > c.Answered*r.flagMaxAutomatedPercent):
		return scrapingFlag
	}
	return scrapingNone
}

// admitChallenge enforces the daily quota of the owner, then counts the challenge for the scraping
// check of the key and acts on its verdict. Requests turned away earlier are never counted: a site
// that ran out of quota must not be taken for a scraper.
func (uc *ChallengeUsecase) admitChallenge(ctx context.Context, site *Site) error {
	now := uc.now()
	day := quota.Day(now, uc.location)
	verificationsToday, err := uc.counters.QuotaUsed(ctx, site.UserID, day)
	if err != nil {
		return err
	}
	if verificationsToday >= site.DailyQuota {
		return ErrDailyQuotaExceeded
	}

	counts, err := uc.counters.CountChallenge(ctx, site.APIKeyID, day)
	if err != nil {
		return err
	}
	switch uc.scraping.judge(counts) {
	case scrapingSuspend:
		suspended, err := uc.sites.SuspendForScraping(ctx, site.APIKeyID)
		if err != nil {
			return err
		}
		if suspended {
			uc.recordScrapingEvent(ctx, site.APIKeyID, "suspended", day, counts)
		}
		return ErrApiKeySuspended
	case scrapingFlag:
		if uc.flaggedToday(site) {
			return nil
		}
		// A failure only delays the throttling to the next challenge.
		flagged, err := uc.sites.FlagForScraping(ctx, site.APIKeyID, dayStart(now, uc.location))
		if err != nil {
			log.ErrorContext(ctx, "failed flagging key for scraping", "err", err, "api_key_id", site.APIKeyID)
		} else if flagged {
			uc.recordScrapingEvent(ctx, site.APIKeyID, "flagged", day, counts)
		}
	}
	return nil
}

// recordScrapingEvent writes what the scraping check did, with the counters it went by, into the
// activity log of the key. A failure is logged: the key has been dealt with already.
func (uc *ChallengeUsecase) recordScrapingEvent(ctx context.Context, apiKeyID, action, day string, counts DailyCounts) {
	detail := map[string]any{
		"reason":    "SCRAPING",
		"day":       day,
		"issued":    counts.Issued,
		"answered":  counts.Answered,
		"solved":    counts.Solved,
		"automated": counts.Automated,
	}
	if err := uc.sites.RecordEvent(ctx, apiKeyID, action, detail); err != nil {
		log.ErrorContext(ctx, "failed recording key event", "err", err, "api_key_id", apiKeyID, "action", action)
	}
}

// dayStart is the start of the local day t falls on.
func dayStart(t time.Time, location *time.Location) time.Time {
	local := t.In(location)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
}
