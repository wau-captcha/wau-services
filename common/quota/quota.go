// Package quota names the Redis counters behind the daily verification quota and the
// scraping check, so every service reads and writes the same keys for the same day.
package quota

import "time"

// CounterTTL is set when a counter is created. It outlives the day being counted, after which
// nobody reads the key any more, so the counter resets without a cleanup job.
const CounterTTL = 24 * time.Hour

// Day is the local calendar day t falls on, as YYYY-MM-DD.
func Day(t time.Time, location *time.Location) string {
	return t.In(location).Format(time.DateOnly)
}

// NextReset is when the day after t starts in loc, which is when fresh counters begin.
func NextReset(t time.Time, location *time.Location) time.Time {
	local := t.In(location)
	return time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, location)
}

// DailyQuotaKey counts the successful verifications of a user on a day, all keys together.
func DailyQuotaKey(userID, day string) string {
	return "quota:daily:" + userID + ":" + day
}

// IssuedChallengesKey counts the challenges issued for an API key on a day.
func IssuedChallengesKey(apiKeyID, day string) string {
	return "abuse:new:" + apiKeyID + ":" + day
}

// CheckedAnswersKey counts the answers checked for challenges of an API key on a day.
func CheckedAnswersKey(apiKeyID, day string) string {
	return "abuse:verify:" + apiKeyID + ":" + day
}
