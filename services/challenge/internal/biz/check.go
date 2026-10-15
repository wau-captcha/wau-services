package biz

import (
	"context"
	"math"
	"net/netip"
	"time"
)

// Answer is an answer to a challenge as verify passes it on, with the /challenge/verify request it
// came in.
type Answer struct {
	ChallengeID string
	// The text read from the image, or the JSON of a slide_puzzle drag, of animal_jump's clicks, of
	// object_rotate's presses or of auto_check's browser probes.
	Text string
	// Nil when the client sent none.
	Behavior *Behavior
	// The site key the client answers for; empty from SDKs before 1.3.0.
	SiteKey string
	// The request: its website (requestinfo.Origin), the client's address and headers. A verify
	// that does not pass them on yet sends no address: then none of them is compared.
	Origin         string
	Client         netip.Addr
	UserAgent      string
	SecFetchSite   string
	AcceptLanguage string
	// The proof of work; empty from SDKs before 1.3.0.
	PowNonce string
}

// Outcome is what Check concluded about an answer.
type Outcome struct {
	Challenge *Challenge
	// The answer solves the challenge: right, for this key and this website, and not automated.
	Right    bool
	Behavior BehaviorResult
	// The key requires behaviour data and the answer came without any, so it was refused.
	BehaviorRequired bool
	// For a right answer to an interactive type: a clearance the browser keeps to pass auto_check
	// on this key without a puzzle for a while (see clearance.go). Empty otherwise.
	Clearance string
}

// Check takes the challenge, so each one is answered once whatever happens, and judges the answer:
// the text or the drag, the site key and website it comes from, and the behaviour behind it; for
// auto_check, whether all of that lets the visitor through without a puzzle. A right answer to an
// interactive type earns the browser a clearance for auto_check. An address over its limit of
// wrong answers is refused before anything is taken. A malformed answer is a wrong one, not an
// error.
func (uc *ChallengeUsecase) Check(ctx context.Context, answer Answer) (*Outcome, error) {
	if !validChallengeID(answer.ChallengeID) {
		return nil, ErrChallengeIDInvalid
	}
	if wait := uc.blockedFor(ctx, answer.Client); wait > 0 {
		return nil, rateLimited(wait)
	}
	challenge, err := uc.challenges.Take(ctx, answer.ChallengeID)
	if err != nil {
		return nil, err
	}
	sinceIssue := uc.sinceIssue(challenge)
	signals := uc.observe(ctx, challenge, answer)
	result := checkBehavior(answer.Behavior, challenge.Type, sinceIssue, signals)

	right := false
	if kind, ok := challengeKinds[challenge.Type]; ok {
		right = kind.check(answer.Text, challenge.Answer, sinceIssue)
	}
	// The challenge belongs to one key and the website that asked for it.
	matched := true
	if answer.SiteKey != "" && challenge.SiteKey != "" && answer.SiteKey != challenge.SiteKey {
		matched = false
		result.Reasons = append(result.Reasons, "site_key_mismatch")
	}
	if answer.Client.IsValid() && challenge.Origin != "" && answer.Origin != challenge.Origin {
		matched = false
		result.Reasons = append(result.Reasons, "origin_changed")
	}
	right = right && matched
	if challenge.Type == TypeAutoCheck {
		right, result = uc.judgeAutoCheck(ctx, challenge, answer, signals, result, matched)
	}
	if result.Verdict == VerdictAutomated {
		right = false
	}
	outcome := &Outcome{Challenge: challenge, Right: right, Behavior: result}
	if challenge.RequireBehavior && result.Verdict == VerdictNotMeasured {
		outcome.Right, outcome.BehaviorRequired = false, true
	}
	if outcome.Right {
		outcome.Clearance = uc.clearanceFor(ctx, challenge, answer)
	}
	// A right answer refused for want of behaviour is not a wrong one: the visitors of a site on
	// an old SDK must not get their addresses blocked. Nor is an auto_check that falls back, unless
	// it was judged automated: falling back is the normal path, and the visitors behind one carrier
	// NAT share an address.
	wrong := !right && (challenge.Type != TypeAutoCheck || result.Verdict == VerdictAutomated)
	uc.recordOutcome(ctx, answer.Client, wrong, result.Verdict)
	return outcome, nil
}

// observe compares the answer's request with the challenge's. Without the client's address the
// answer came through a verify that does not pass the request on, during a rollout: only the
// proof of work is judged then.
func (uc *ChallengeUsecase) observe(ctx context.Context, challenge *Challenge, answer Answer) Signals {
	var signals Signals
	if challenge.PowDifficulty > 0 {
		switch {
		case answer.PowNonce == "":
			signals.NoPow = true
		case !powSolved(challenge.PowSalt, challenge.ID, answer.PowNonce, challenge.PowDifficulty):
			signals.PowInvalid = true
		}
	}
	if !answer.Client.IsValid() {
		return signals
	}
	signals.AutomationUA = automationUA(answer.UserAgent)
	signals.MissingBrowserHeaders = missingBrowserHeaders(answer.UserAgent, answer.SecFetchSite, answer.AcceptLanguage)
	signals.IPChanged = challenge.ClientIP != "" && answer.Client.String() != challenge.ClientIP
	signals.UAChanged = challenge.UserAgentHash != "" && hashUserAgent(answer.UserAgent) != challenge.UserAgentHash
	signals.BadHistory = uc.badHistory(ctx, answer.Client)
	return signals
}

// sinceIssue is how long ago the challenge was issued. One stored without its issue time, by the
// version before during a rollout, counts as issued long ago.
func (uc *ChallengeUsecase) sinceIssue(challenge *Challenge) time.Duration {
	if challenge.IssuedAt.IsZero() {
		return math.MaxInt64
	}
	return uc.now().Sub(challenge.IssuedAt)
}
