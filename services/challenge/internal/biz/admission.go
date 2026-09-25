package biz

import (
	"common/quota"
	"context"
)

const (
	// A key is judged for spam once it was issued this many challenges (/challenge/new) in a day,
	scrapingMinChallenges = 200
	// and taken for a spammer when fewer than this share of them reached /challenge/verify.
	scrapingMinVerifiedPercent = 20
)

// CounterRepo keeps the daily counters shared with the verify service (see common/quota).
type CounterRepo interface {
	// QuotaUsed returns the successful verifications of a user on a day, 0 when none.
	QuotaUsed(ctx context.Context, userID, day string) (int64, error)
	// CountChallenge records a challenge issued for an API key on a day and returns the
	// challenges issued so far, this one included, and the answers checked for them.
	CountChallenge(ctx context.Context, apiKeyID, day string) (issued, answered int64, err error)
}

// admitChallenge enforces the daily quota of the owner, then counts the challenge for the scraping
// check of the key. Requests turned away earlier are never counted: a site that ran out
// of quota must not be taken for a scraper.
func (uc *ChallengeUsecase) admitChallenge(ctx context.Context, site *Site) error {
	day := quota.Day(uc.now(), uc.location)
	verificationsToday, err := uc.counters.QuotaUsed(ctx, site.UserID, day)
	if err != nil {
		return err
	}
	if verificationsToday >= site.DailyQuota {
		return ErrDailyQuotaExceeded
	}

	issuedToday, answeredToday, err := uc.counters.CountChallenge(ctx, site.APIKeyID, day)
	if err != nil {
		return err
	}
	if !isScraping(issuedToday, answeredToday) {
		return nil
	}
	if err := uc.sites.SuspendForScraping(ctx, site.APIKeyID); err != nil {
		return err
	}

	return ErrApiKeySuspended
}

// isScraping tells a key that fetches challenges without answering them: once it has
// scrapingMinChallenges /challenge/new calls in the day, (/challenge/verify)/(/challenge/new)
// below scrapingMinVerifiedPercent is spam. Most scrapers harvest the images and never submit
// an answer.
func isScraping(issued, answered int64) bool {
	return issued >= scrapingMinChallenges && answered*100 < issued*scrapingMinVerifiedPercent
}
