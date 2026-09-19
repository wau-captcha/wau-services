// Package quota names the Redis counters behind the daily verification quota and the
// scraping check, so every service reads and writes the same keys for the same day.
package quota

import "time"

// TTL is set when a counter is created. It outlives the day being counted, after which
// nobody reads the key any more, so the counter resets without a cleanup job.
const TTL = 24 * time.Hour

// Day is the local calendar day t falls on, as YYYY-MM-DD.
func Day(t time.Time, loc *time.Location) string {
	return t.In(loc).Format(time.DateOnly)
}

// NextReset is when the day after t starts in loc, which is when fresh counters begin.
func NextReset(t time.Time, loc *time.Location) time.Time {
	local := t.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, loc)
}

// UsedKey counts the successful verifications of a user on a day, all keys together.
func UsedKey(userID, day string) string {
	return "quota:daily:" + userID + ":" + day
}

// ChallengesKey counts the challenges issued for an API key on a day.
func ChallengesKey(apiKeyID, day string) string {
	return "abuse:new:" + apiKeyID + ":" + day
}

// VerificationsKey counts the answers checked for challenges of an API key on a day.
func VerificationsKey(apiKeyID, day string) string {
	return "abuse:verify:" + apiKeyID + ":" + day
}
