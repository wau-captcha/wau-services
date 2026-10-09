package biz

import (
	"context"
	"net/netip"
	"strings"
)

// ChallengeClient checks answers with the challenge service.
type ChallengeClient interface {
	CheckAnswer(ctx context.Context, answer Answer) (*Solution, error)
}

// Answer is a /challenge/verify call: the answer, and what verify passes on to the challenge
// service about the request it came in, which that service compares with the /challenge/new
// request of the challenge.
type Answer struct {
	ChallengeID string
	// The text read from the image, or the JSON of a slide_puzzle drag.
	Text string
	// Nil when the client sent none.
	Behavior *Behavior
	// Empty from SDKs before 1.3.0.
	SiteKey  string
	PowNonce string
	// The website of the page (requestinfo.Origin), the client's address read through the trusted
	// proxies, and headers every browser sends.
	Origin         string
	Client         netip.Addr
	UserAgent      string
	SecFetchSite   string
	AcceptLanguage string
}

// Solution is what the challenge service concluded about an answer.
type Solution struct {
	// The answer solves the challenge.
	Success       bool
	APIKeyID      string
	ChallengeType string
	// Owner of the API key; empty for challenges issued before owners were recorded.
	UserID   string
	Behavior BehaviorResult
	// The host name of the website the challenge was asked for from; empty when unknown.
	Hostname string
	// The key requires behaviour data and the answer came without any.
	BehaviorRequired bool
}

// Limits of what verify passes on. A site key or a nonce past them cannot be valid, so they are
// cut one character past the limit and still judged; the challenge service only compares the
// first maxUserAgent bytes of a User-Agent (maxUserAgentBytes there), and only reads whether the
// other headers are there at all.
const (
	maxSiteKey   = 64
	maxPowNonce  = 20
	maxUserAgent = 1024
	maxHeader    = 64
)

// boundAnswer keeps what verify forwards small; the headers come in up to the 1 MB the HTTP
// server reads.
func boundAnswer(answer Answer) Answer {
	answer.Behavior = boundBehavior(answer.Behavior)
	answer.SiteKey = cut(answer.SiteKey, maxSiteKey+1)
	answer.PowNonce = cut(answer.PowNonce, maxPowNonce+1)
	answer.UserAgent = cut(answer.UserAgent, maxUserAgent)
	answer.SecFetchSite = cut(answer.SecFetchSite, maxHeader)
	answer.AcceptLanguage = cut(answer.AcceptLanguage, maxHeader)
	return answer
}

// cut shortens value to at most limit bytes and drops what is not UTF-8, such as a character cut
// in two: a protobuf string cannot carry it.
func cut(value string, limit int) string {
	return strings.ToValidUTF8(value[:min(len(value), limit)], "")
}
