package biz

import (
	"context"

	"common/quota"

	"github.com/go-kratos/kratos/v3/log"
)

const (
	// A key is judged for scraping once it was issued this many challenges in a day,
	scrapingMinChallenges = 200
	// and taken for a scraper when fewer than this share of them got an answer checked.
	scrapingMinAnsweredPercent = 30
)

// CounterRepo keeps the daily counters shared with the verify service (see common/quota).
type CounterRepo interface {
	// QuotaUsed returns the successful verifications of a user on a day, 0 when none.
	QuotaUsed(ctx context.Context, userID, day string) (int64, error)
	// CountChallenge records a challenge issued for an API key on a day and returns the
	// challenges issued so far, this one included, and the answers checked for them.
	CountChallenge(ctx context.Context, apiKeyID, day string) (issued, verified int64, err error)
}

// admit enforces the daily quota of the owner, then counts the challenge for the scraping
// check of the key. Requests turned away earlier are never counted: a site that ran out
// of quota must not be taken for a scraper.
func (uc *ChallengeUsecase) admit(ctx context.Context, site *Site) error {
	day := quota.Day(uc.now(), uc.location)
	used, err := uc.counters.QuotaUsed(ctx, site.UserID, day)
	if err != nil {
		return err
	}
	if used >= site.DailyQuota {
		return ErrDailyQuotaExceeded
	}

	issued, verified, err := uc.counters.CountChallenge(ctx, site.APIKeyID, day)
	if err != nil {
		return err
	}
	if !isScraping(issued, verified) {
		return nil
	}
	if err := uc.sites.SuspendForScraping(ctx, site.APIKeyID); err != nil {
		return err
	}
	log.WarnContext(ctx, "api key suspended for scraping",
		"api_key_id", site.APIKeyID, "challenges", issued, "verifications", verified)
	return ErrApiKeySuspended
}

// isScraping tells a key that fetches challenges without answering them: most scrapers
// harvest the images and never submit an answer.
func isScraping(issued, verified int64) bool {
	return issued >= scrapingMinChallenges && verified*100 < issued*scrapingMinAnsweredPercent
}
