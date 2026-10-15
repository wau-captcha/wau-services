package biz

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Penalties of the signals, on the score of checkBehavior (see behaviorcheck.go). None but no_pow
// makes a session suspicious alone; the others do once they add up past half the score.
const (
	// No proof of work from a client that sends behaviour: an SDK before 1.3.0, or a script that
	// skips the work. On its own it already makes the session suspicious.
	penaltyNoPow = 0.6
	// People rarely change address or browser within the two minutes of a challenge.
	penaltyIPChanged = 0.2
	penaltyUAChanged = 0.3
	// Every current browser sends Sec-Fetch-Site and Accept-Language; Safari before 16.4 lacks the
	// first, hence a small penalty.
	penaltyMissingBrowserHeaders = 0.2
	penaltyBadHistory            = 0.3
	// Hints the client reports about its browser. A script fakes them at will and some privacy
	// tools trip them, so they are weak: the three together make a session suspicious.
	penaltyHeadlessUA                = 0.25
	penaltyNoLanguages               = 0.15
	penaltyChromeWithoutWindowChrome = 0.15
)

// Signals are what the service observes itself about the requests behind an answer, as opposed
// to what the client reports in Behavior. AutomationUA and PowInvalid break a rule; the others
// lower the score.
type Signals struct {
	// The User-Agent of the answer is missing or names an automation client.
	AutomationUA bool
	// The proof of work the client sent does not solve the challenge.
	PowInvalid bool
	// The challenge asked for a proof of work and the client sent none: an SDK before 1.3.0.
	NoPow bool
	// The answer comes from another address, or another User-Agent, than the challenge was asked by.
	IPChanged bool
	UAChanged bool
	// A browser User-Agent without the headers every browser sends.
	MissingBrowserHeaders bool
	// The address had more answers judged automated in the last hour than people produce.
	BadHistory bool
}

// brokenRule names the rule the signals break, or "".
func (s Signals) brokenRule() string {
	switch {
	case s.AutomationUA:
		return "automation_ua"
	case s.PowInvalid:
		return "pow_invalid"
	}
	return ""
}

// automationAgents are User-Agent fragments of HTTP libraries and automation tools; a browser a
// person uses names none of them.
var automationAgents = []string{"headlesschrome", "python-requests", "curl/", "go-http-client", "node-fetch", "axios", "playwright", "phantomjs"}

// automationUA reports whether a User-Agent is missing or names an automation client.
func automationUA(userAgent string) bool {
	userAgent = strings.ToLower(strings.TrimSpace(userAgent))
	if userAgent == "" {
		return true
	}
	for _, agent := range automationAgents {
		if strings.Contains(userAgent, agent) {
			return true
		}
	}
	return false
}

// browserUA reports whether a User-Agent claims to be a browser: every one starts with Mozilla/.
func browserUA(userAgent string) bool {
	return strings.HasPrefix(strings.TrimSpace(userAgent), "Mozilla/")
}

// missingBrowserHeaders reports a browser User-Agent without Sec-Fetch-Site, which browsers send on
// every fetch, or without Accept-Language.
func missingBrowserHeaders(userAgent, secFetchSite, acceptLanguage string) bool {
	return browserUA(userAgent) && (strings.TrimSpace(secFetchSite) == "" || strings.TrimSpace(acceptLanguage) == "")
}

// maxUserAgentBytes is how much of a User-Agent verify passes on with an answer (maxUserAgent
// there); no browser sends a longer one.
const maxUserAgentBytes = 1024

// hashUserAgent is what a challenge keeps of the User-Agent that asked for it: enough to tell
// whether the answer comes from the same one, and nothing to read back.
func hashUserAgent(userAgent string) string {
	sum := userAgentDigest(userAgent)
	return hex.EncodeToString(sum[:])
}

// userAgentDigest is the SHA-256 of a User-Agent, of hashUserAgent and of clearances. Only what
// verify passes on counts: the first maxUserAgentBytes, without what is not UTF-8.
func userAgentDigest(userAgent string) [sha256.Size]byte {
	return sha256.Sum256([]byte(strings.ToValidUTF8(userAgent[:min(len(userAgent), maxUserAgentBytes)], "")))
}
