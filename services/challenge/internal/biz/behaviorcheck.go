package biz

import (
	"math"
	"slices"
	"time"
)

// Every threshold of the behaviour check. The values come from synthetic and browser-driven
// sessions; tune them once real traffic shows how people and bots score.
const (
	// The layout of Behavior this check reads; another one is not measured, like no behaviour.
	behaviorVersion = 1
	// Limits of what the SDK sends; a payload past them did not come from it.
	behaviorMaxPointer      = 200
	behaviorMaxKeyIntervals = 64
	behaviorMaxCoordinate   = 20000 // CSS px from the widget; no screen is that large
	behaviorMaxEvents       = 10000 // input, paste or untrusted events while one challenge is shown
	// The SDK starts timing once the challenge it fetched is on screen, so its elapsed time cannot
	// pass the time since issue; the slack covers the resolution of the two clocks.
	behaviorClockSlack = time.Second

	// The score starts at 1 and loses a penalty for each sign of a script; under
	// behaviorMinHumanScore the session is suspicious.
	behaviorMinHumanScore = 0.5
	// navigator.webdriver: Selenium, Puppeteer and Playwright set it unless they are told to hide
	// it. On its own it already makes the session suspicious.
	penaltyWebdriver = 0.6
	// No pointer at all: a script, or a person on the keyboard alone, who must still pass.
	penaltyNoPointer = 0.15
	// Every stroke of the mouse at one steady speed: interpolated, not pushed by a hand.
	penaltyEvenPointer = 0.3
	// Key presses at even intervals, or faster than people type, come from a loop. One of these
	// signs alone leaves the session human; two make it suspicious.
	penaltyEvenTyping = 0.3
	penaltyFastTyping = 0.3
	// People type what they read; a pasted answer was read somewhere else.
	penaltyPasted = 0.1

	// A pause longer than this between pointer samples, or a sample in place, ends a stroke.
	strokePauseMs = 100
	// Strokes with fewer speeds say nothing about how the pointer was moved.
	minStrokeSpeeds = 5
	// Coefficient of variation of the speed within a stroke: a hand speeds up and brakes (0.6 and
	// more); an interpolated path driven through a real browser stays near 0.2 from timing jitter.
	minStrokeSpeedVariation = 0.35
	// Fewer key intervals say nothing about the rhythm.
	minKeyIntervalsJudged = 4
	// Coefficient of variation of the key intervals. With the 4 to 6 presses of an answer a person
	// goes under 0.1 now and then by chance; a loop with a fixed delay stays near 0.03.
	minKeyIntervalVariation = 0.06
	// Median key interval: the fastest typists stay around 60 ms.
	minMedianKeyIntervalMs = 35
)

// minSolveTime is the least time each type takes a person on screen: reading and typing the
// text, finding the hole and dragging, or watching two rounds of jumps and clicking. Faster
// answers come from scripts.
var minSolveTime = map[string]time.Duration{
	TypeImageDistort: 1000 * time.Millisecond,
	TypeMotionNoise:  1500 * time.Millisecond,
	TypeSlidePuzzle:  400 * time.Millisecond,
	TypeAnimalJump:   1500 * time.Millisecond,
}

var pointerTypes = []string{"", "mouse", "touch", "pen"}

// checkBehavior judges how the person behaved while a challenge of challengeType was on screen,
// sinceIssue after it was issued, together with the signals the service observed itself. A rule
// broken by the signals makes the answer automated even without behaviour; otherwise no behaviour
// (nil, or a version this check does not know) is not measured. A broken hard rule of the
// behaviour makes it automated; otherwise the score decides.
func checkBehavior(behavior *Behavior, challengeType string, sinceIssue time.Duration, signals Signals) BehaviorResult {
	if reason := signals.brokenRule(); reason != "" {
		return BehaviorResult{Verdict: VerdictAutomated, Reasons: []string{reason}}
	}
	if behavior == nil || behavior.Version != behaviorVersion {
		return BehaviorResult{Verdict: VerdictNotMeasured}
	}
	if reason := brokenRule(behavior, challengeType, sinceIssue); reason != "" {
		return BehaviorResult{Verdict: VerdictAutomated, Reasons: []string{reason}}
	}
	score, reasons := 1.0, []string{}
	penalize := func(seen bool, penalty float64, reason string) {
		if seen {
			score -= penalty
			reasons = append(reasons, reason)
		}
	}
	penalize(behavior.Webdriver, penaltyWebdriver, "webdriver")
	penalize(len(behavior.Pointer) == 0 && behavior.PointerType == "", penaltyNoPointer, "no_pointer")
	penalize(evenPointer(behavior), penaltyEvenPointer, "even_pointer")
	variation, median, judged := keyRhythm(behavior.KeyIntervalsMs)
	penalize(judged && variation < minKeyIntervalVariation, penaltyEvenTyping, "even_typing")
	penalize(judged && median < minMedianKeyIntervalMs, penaltyFastTyping, "fast_typing")
	penalize(behavior.PasteEvents > 0, penaltyPasted, "pasted")
	penalize(signals.NoPow, penaltyNoPow, "no_pow")
	penalize(signals.IPChanged, penaltyIPChanged, "ip_changed")
	penalize(signals.UAChanged, penaltyUAChanged, "ua_changed")
	penalize(signals.MissingBrowserHeaders, penaltyMissingBrowserHeaders, "missing_browser_headers")
	penalize(signals.BadHistory, penaltyBadHistory, "ip_bad_history")
	penalize(behavior.HeadlessUA, penaltyHeadlessUA, "headless_ua")
	penalize(behavior.NoLanguages, penaltyNoLanguages, "no_languages")
	penalize(behavior.ChromeWithoutWindowChrome, penaltyChromeWithoutWindowChrome, "chrome_without_window_chrome")

	score = math.Round(max(score, 0)*100) / 100
	verdict := VerdictHuman
	if score < behaviorMinHumanScore {
		verdict = VerdictSuspicious
	}
	return BehaviorResult{Verdict: verdict, Score: score, Reasons: reasons}
}

// brokenRule returns the hard rule the behaviour breaks, or "".
func brokenRule(behavior *Behavior, challengeType string, sinceIssue time.Duration) string {
	elapsed := time.Duration(behavior.ElapsedMs) * time.Millisecond
	switch {
	case !wellFormed(behavior):
		return "malformed_behavior"
	case behavior.UntrustedEvents > 0:
		return "untrusted_events"
	// Written this way round so that the unknown issue time (math.MaxInt64) cannot overflow.
	case elapsed-behaviorClockSlack > sinceIssue:
		return "elapsed_beyond_issue"
	case elapsed < minSolveTime[challengeType]:
		return "too_fast"
	// The text was in the box without anyone typing, dictating or pasting it: a script set it.
	case textKinds[challengeType] && behavior.InputEvents == 0 && behavior.PasteEvents == 0:
		return "scripted_input"
	}
	return ""
}

var textKinds = map[string]bool{TypeImageDistort: true, TypeMotionNoise: true}

func wellFormed(b *Behavior) bool {
	if b.ElapsedMs < 0 || b.HiddenMs < 0 || b.HiddenMs > b.ElapsedMs || !counted(b.InputEvents) || !counted(b.PasteEvents) ||
		!counted(b.UntrustedEvents) || len(b.Pointer) > behaviorMaxPointer || len(b.KeyIntervalsMs) > behaviorMaxKeyIntervals ||
		!slices.Contains(pointerTypes, b.PointerType) {
		return false
	}
	for i, sample := range b.Pointer {
		if sample.T < 0 || sample.T > b.ElapsedMs || (i > 0 && sample.T < b.Pointer[i-1].T) ||
			!finite(sample.X) || !finite(sample.Y) || math.Abs(sample.X) > behaviorMaxCoordinate || math.Abs(sample.Y) > behaviorMaxCoordinate {
			return false
		}
	}
	for _, interval := range b.KeyIntervalsMs {
		if interval < 0 || interval > b.ElapsedMs {
			return false
		}
	}
	return true
}

// counted reports whether an event count is one a person could produce.
func counted(events int) bool { return events >= 0 && events <= behaviorMaxEvents }

// evenPointer reports whether every stroke of a mouse or pen long enough to judge kept one speed.
// Touch moves only while dragging, so it is not judged.
func evenPointer(b *Behavior) bool {
	if b.PointerType != "mouse" && b.PointerType != "pen" {
		return false
	}
	judged, even := 0, 0
	for _, speeds := range strokes(b.Pointer) {
		if len(speeds) < minStrokeSpeeds {
			continue
		}
		judged++
		if coefficientOfVariation(speeds) < minStrokeSpeedVariation {
			even++
		}
	}
	return judged > 0 && even == judged
}
