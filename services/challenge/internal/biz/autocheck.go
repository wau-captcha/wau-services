package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/netip"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"challenge/internal/conf"

	"github.com/go-kratos/kratos/v3/log"
)

// auto_check lets a visitor through without a puzzle when the browser and the behaviour in it look
// like a person's, and sends everyone else to an interactive type, the fallback. It has no media
// and stores no answer: the SDK answers with what its browser probes found, and the decision rests
// on that, on the behaviour check and on what the service observes itself. It has no randomness:
// the same browser asking again gets the same result. Every threshold of it is here; the pass mark
// and the cap per address are in conf.AutoCheck.
const (
	// Limits of the answer the SDK writes, {"v":1,"ua":..,"lang":..,"f":[..]}; one past them did not
	// come from it.
	autoMaxAnswerBytes = 2 << 10
	autoAnswerVersion  = 1
	// navigator.userAgent is cut to as many characters; the User-Agent header is compared on as many.
	autoMaxUserAgent = 512
	// No BCP 47 tag a browser reports comes near it.
	autoMaxLanguage = 35
	// The SDK has seven probes; the room is for later ones.
	autoMaxFindings = 16
	autoMaxFinding  = 32

	// Penalties on the score of checkBehavior. Each finding is a hint a careful script avoids and a
	// few people's setups trip, so none refuses on its own: two strong ones, or one with no pointer
	// at all, keep a visitor under the pass mark, and that visitor gets the fallback, never an error.
	// Stealth plugins patch navigator.webdriver and friends with functions that are not native.
	penaltyPatchedNatives = 0.4
	// navigator.userAgent differs from the User-Agent header: one of the two was rewritten.
	penaltyUAMismatch = 0.4
	// A headless window has no outer size.
	penaltyOuterZero = 0.3
	// WebGL rendered on the CPU (SwiftShader, llvmpipe): headless Chrome, servers and some VMs.
	penaltySoftwareGL = 0.25
	// Notifications "denied" while the Permissions API says "prompt": headless Chrome.
	penaltyNotificationMismatch = 0.2
	// A phone's User-Agent on a device without touch points.
	penaltyTouchMismatch = 0.2
	// navigator.language and Accept-Language name different languages; a few people set them so.
	penaltyLangMismatch = 0.15
	// No CPU count, or a desktop Chrome without the PDF viewer plugins: rare on people's machines.
	penaltyNoHardware = 0.1
	penaltyNoPlugins  = 0.1
)

// autoFindings are what lowers an auto_check score beyond checkBehavior, in the order their codes
// join Reasons. ua_mismatch and lang_mismatch the service finds itself; the others the probes report.
var autoFindings = []struct {
	code    string
	penalty float64
	server  bool
}{
	{"patched_natives", penaltyPatchedNatives, false},
	{"ua_mismatch", penaltyUAMismatch, true},
	{"outer_zero", penaltyOuterZero, false},
	{"software_gl", penaltySoftwareGL, false},
	{"notification_mismatch", penaltyNotificationMismatch, false},
	{"touch_mismatch", penaltyTouchMismatch, false},
	{"lang_mismatch", penaltyLangMismatch, true},
	{"no_hardware", penaltyNoHardware, false},
	{"no_plugins", penaltyNoPlugins, false},
}

// fallbackOrder is the order auto_check offers the interactive types in when the site names none
// the key allows: the quickest to solve first.
var fallbackOrder = []string{TypeSlidePuzzle, TypeImageDistort, TypeAnimalJump, TypeMotionNoise}

// autoCheckRules are the settings of conf.AutoCheck.
type autoCheckRules struct {
	powBits       int
	minScore      float64
	passesPerHour int64
}

func newAutoCheckRules(autoConf *conf.AutoCheck) (autoCheckRules, error) {
	bits, percent, passes := autoConf.GetPowDifficulty(), autoConf.GetMinScorePercent(), autoConf.GetPassesPerIpHour()
	switch {
	case bits < 0 || bits > maxPowDifficulty:
		return autoCheckRules{}, fmt.Errorf("auto_check pow_difficulty must be between 0 and %d", maxPowDifficulty)
	case percent < 1 || percent > 100:
		return autoCheckRules{}, fmt.Errorf("auto_check min_score_percent must be between 1 and 100")
	case passes < 0:
		return autoCheckRules{}, fmt.Errorf("auto_check passes_per_ip_hour must be 0 or more")
	}
	return autoCheckRules{powBits: int(bits), minScore: float64(percent) / 100, passesPerHour: int64(passes)}, nil
}

// fallbackType is the type an auto_check challenge loads when it does not pass: the one the site
// asked for when the key allows it, else the first of fallbackOrder the key allows, else any other
// type it allows, in name order so the choice never changes; empty when it allows none.
func fallbackType(allowed []string, requested string) string {
	interactive := func(challengeType string) bool {
		_, known := challengeKinds[challengeType]
		return known && challengeType != TypeAutoCheck && slices.Contains(allowed, challengeType)
	}
	if interactive(requested) {
		return requested
	}
	for _, challengeType := range append(slices.Clone(fallbackOrder), slices.Sorted(slices.Values(allowed))...) {
		if interactive(challengeType) {
			return challengeType
		}
	}
	return ""
}

// autoProbe is the answer the SDK writes for an auto_check challenge: navigator.userAgent and
// navigator.language, and the names of the browser probes that tripped.
type autoProbe struct {
	Version   *float64 `json:"v"`
	UserAgent *string  `json:"ua"`
	Language  *string  `json:"lang"`
	Findings  []string `json:"f"`
}

// parseAutoProbe reads an auto_check answer; false when it is not one the SDK writes.
func parseAutoProbe(answer string) (autoProbe, bool) {
	var probe autoProbe
	if len(answer) > autoMaxAnswerBytes || json.Unmarshal([]byte(answer), &probe) != nil {
		return autoProbe{}, false
	}
	if probe.Version == nil || *probe.Version != autoAnswerVersion || probe.UserAgent == nil || probe.Language == nil ||
		probe.Findings == nil || utf8.RuneCountInString(*probe.UserAgent) > autoMaxUserAgent ||
		len(*probe.Language) > autoMaxLanguage || len(probe.Findings) > autoMaxFindings {
		return autoProbe{}, false
	}
	for _, finding := range probe.Findings {
		if len(finding) > autoMaxFinding {
			return autoProbe{}, false
		}
	}
	return probe, true
}

// generateAutoCheck has nothing to draw and nothing to remember.
func generateAutoCheck() (string, Media, error) { return "", Media{}, nil }

// checkAutoCheck only tells whether the answer is one the SDK writes; Check decides on the pass.
func checkAutoCheck(answer, _ string, _ time.Duration) bool {
	_, ok := parseAutoProbe(answer)
	return ok
}

// judgeAutoCheck decides whether an auto_check answer lets its visitor through: the behaviour as
// checkBehavior judged it (result), less a penalty per finding, must reach the pass mark; the proof
// of work must solve the challenge; the answer must come with the client's address, from the key and
// website of the challenge (matched), and the address must be under its cap of passes. It returns the
// result with the final score and every reason, so the usage log shows why a visitor fell back.
func (uc *ChallengeUsecase) judgeAutoCheck(ctx context.Context, challenge *Challenge, answer Answer, signals Signals,
	result BehaviorResult, matched bool) (bool, BehaviorResult) {
	probe, formed := parseAutoProbe(answer.Text)
	if !formed {
		result.Reasons = append(result.Reasons, "malformed_answer")
		return false, result
	}
	if !result.Measured() || result.Verdict == VerdictAutomated {
		return false, result
	}
	// Unknown names are ignored, and each counts once.
	reported := map[string]bool{}
	for _, finding := range probe.Findings {
		reported[finding] = true
	}
	// The service's own findings compare the probe with the request, which a verify that passes no
	// address does not pass on either.
	observed := map[string]bool{}
	if answer.Client.IsValid() {
		observed["ua_mismatch"] = firstRunes(*probe.UserAgent, autoMaxUserAgent) != firstRunes(answer.UserAgent, autoMaxUserAgent)
		observed["lang_mismatch"] = answer.AcceptLanguage != "" && primarySubtag(*probe.Language) != primarySubtag(firstTag(answer.AcceptLanguage))
	}
	score, reasons := result.Score, slices.Clone(result.Reasons)
	for _, finding := range autoFindings {
		seen := reported[finding.code]
		if finding.server {
			seen = observed[finding.code]
		}
		if seen {
			score -= finding.penalty
			reasons = append(reasons, finding.code)
		}
	}
	score = math.Round(max(score, 0)*100) / 100
	powSolved := challenge.PowDifficulty == 0 || (!signals.NoPow && !signals.PowInvalid)
	pass := matched && powSolved && score >= uc.auto.minScore
	if !answer.Client.IsValid() {
		pass = false
		reasons = append(reasons, "no_client_address")
	}
	if pass && uc.autoPassCapped(ctx, answer.Client) {
		pass = false
		reasons = append(reasons, "auto_pass_cap")
	}
	verdict := result.Verdict
	if verdict == VerdictHuman && score < behaviorMinHumanScore {
		verdict = VerdictSuspicious
	}
	if pass {
		if err := uc.history.CountAutoPass(ctx, answer.Client.String(), uc.now()); err != nil {
			log.WarnContext(ctx, "auto pass not counted for its address", "err", err)
		}
	}
	return pass, BehaviorResult{Verdict: verdict, Score: score, Reasons: reasons}
}

// autoPassCapped reports whether addr had its passes_per_ip_hour auto passes in the last hour. A
// failure of Redis counts as under the cap: the other checks have passed already.
func (uc *ChallengeUsecase) autoPassCapped(ctx context.Context, addr netip.Addr) bool {
	if uc.auto.passesPerHour <= 0 {
		return false
	}
	passes, err := uc.history.AutoPassesLastHour(ctx, addr.String(), uc.now())
	if err != nil {
		log.WarnContext(ctx, "auto passes of the address not read", "err", err)
		return false
	}
	return passes >= uc.auto.passesPerHour
}

// firstRunes is text cut to at most n characters.
func firstRunes(text string, n int) string {
	if utf8.RuneCountInString(text) <= n {
		return text
	}
	return string([]rune(text)[:n])
}

// firstTag is the first language of an Accept-Language header, without its weight.
func firstTag(acceptLanguage string) string {
	tag, _, _ := strings.Cut(acceptLanguage, ",")
	tag, _, _ = strings.Cut(tag, ";")
	return tag
}

// primarySubtag is the language of a tag, "vi" of "vi-VN", in lower case.
func primarySubtag(tag string) string {
	language, _, _ := strings.Cut(strings.TrimSpace(tag), "-")
	language, _, _ = strings.Cut(language, "_")
	return strings.ToLower(language)
}
