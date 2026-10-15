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
// the same browser asking again gets the same result. Every probe runs in a browser a bot may
// control, so the pass also rests on what a bot cannot fake from inside it: a clearance from a
// puzzle solved earlier (clearance.go), an address outside the hosting ranges (hosting.go) and
// caps per address and per clearance. Every threshold of it is here; the pass mark, the mode and
// the caps are in conf.AutoCheck.
const (
	// Limits of the answer the SDK writes, {"v":1,"ua":..,"lang":..,"f":[..],"c":..,"m":{..}}; one
	// past them did not come from it. "c" (at most maxClearanceLength) and "m" come from SDK 1.7.0
	// on and are optional.
	autoMaxAnswerBytes = 2 << 10
	autoAnswerVersion  = 1
	// navigator.userAgent is cut to as many characters; the User-Agent header is compared on as many.
	autoMaxUserAgent = 512
	// No BCP 47 tag a browser reports comes near it.
	autoMaxLanguage = 35
	// The SDK has seven probes; the room is for later ones.
	autoMaxFindings = 16
	autoMaxFinding  = 32
	// The pointer summary counts at most as many moves.
	autoMaxPointerMoves = 100000

	// Pointer evidence. The thresholds are first guesses: check them against the "auto check judged"
	// log lines of real traffic before turning enforce_pointer_signals on. Only a mouse that made
	// pointerMinMoves moves is judged.
	pointerMinMoves = 20
	// synthetic_pointer: no move came coalesced with others, which a real mouse polled faster than
	// the screen redraws often produces, or over half the moves had no movement although the
	// position changed: events a script dispatched.
	penaltySyntheticPointer = 0.3
	// step_pointer: over pointerStepPercent of the moves came within 1 ms of the most common gap
	// between moves: a fixed timer, not a hand.
	penaltyStepPointer = 0.2
	pointerStepPercent = 80

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

// The modes of auto_check.
const (
	// An automatic pass needs a clearance: a first visit always gets a puzzle.
	autoModeStrict = "strict"
	// A visitor without a clearance may pass on the signals alone, up to
	// first_visit_passes_per_ip_hour passes per address, with reason first_visit.
	autoModeBalanced = "balanced"
)

// Bounds of clearance_ttl.
const (
	minClearanceTTL = time.Hour
	maxClearanceTTL = 30 * 24 * time.Hour
)

// autoCheckRules are the settings of conf.AutoCheck.
type autoCheckRules struct {
	powBits       int
	minScore      float64
	passesPerHour int64
	mode          string
	// Passes without a clearance per address and hour, in balanced mode.
	firstVisitPerHour int64
	clearanceTTL      time.Duration
	clearancePerHour  int64
	clearancePerDay   int64
	clearanceKey      []byte
	blockHosting      bool
	hosting           hostingRanges
	enforcePointer    bool
}

func newAutoCheckRules(autoConf *conf.AutoCheck) (autoCheckRules, error) {
	bits, percent, passes := autoConf.GetPowDifficulty(), autoConf.GetMinScorePercent(), autoConf.GetPassesPerIpHour()
	mode, ttl := strings.ToLower(strings.TrimSpace(autoConf.GetMode())), autoConf.GetClearanceTtl().AsDuration()
	firstVisits, perHour, perDay := autoConf.GetFirstVisitPassesPerIpHour(), autoConf.GetClearancePassesPerHour(), autoConf.GetClearancePassesPerDay()
	switch {
	case bits < 0 || bits > maxPowDifficulty:
		return autoCheckRules{}, fmt.Errorf("auto_check pow_difficulty must be between 0 and %d", maxPowDifficulty)
	case percent < 1 || percent > 100:
		return autoCheckRules{}, fmt.Errorf("auto_check min_score_percent must be between 1 and 100")
	case passes < 0:
		return autoCheckRules{}, fmt.Errorf("auto_check passes_per_ip_hour must be 0 or more")
	case mode != autoModeStrict && mode != autoModeBalanced:
		return autoCheckRules{}, fmt.Errorf("auto_check mode must be strict or balanced")
	case firstVisits < 0:
		return autoCheckRules{}, fmt.Errorf("auto_check first_visit_passes_per_ip_hour must be 0 or more")
	case perHour < 0 || perDay < 0:
		return autoCheckRules{}, fmt.Errorf("auto_check clearance_passes_per_hour and clearance_passes_per_day must be 0 or more")
	case ttl < minClearanceTTL || ttl > maxClearanceTTL:
		return autoCheckRules{}, fmt.Errorf("auto_check clearance_ttl must be between 3600s (1 hour) and 2592000s (30 days)")
	case strings.TrimSpace(autoConf.GetClearanceSecret()) == "":
		return autoCheckRules{}, fmt.Errorf("auto_check clearance_secret is required")
	}
	hosting, err := newHostingRanges(autoConf.GetExtraHostingCidrs())
	if err != nil {
		return autoCheckRules{}, err
	}
	return autoCheckRules{
		powBits:           int(bits),
		minScore:          float64(percent) / 100,
		passesPerHour:     int64(passes),
		mode:              mode,
		firstVisitPerHour: int64(firstVisits),
		clearanceTTL:      ttl,
		clearancePerHour:  int64(perHour),
		clearancePerDay:   int64(perDay),
		clearanceKey:      clearanceKey(autoConf.GetClearanceSecret()),
		blockHosting:      autoConf.GetBlockHostingAddresses(),
		hosting:           hosting,
		enforcePointer:    autoConf.GetEnforcePointerSignals(),
	}, nil
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
// navigator.language, the names of the browser probes that tripped and, from SDK 1.7.0 on, the
// clearance the browser keeps for the site and a count of its pointer moves.
type autoProbe struct {
	Version   *float64 `json:"v"`
	UserAgent *string  `json:"ua"`
	Language  *string  `json:"lang"`
	Findings  []string `json:"f"`
	// Nil from older SDKs, from a browser that solved no puzzle of the key yet, and from one that
	// moved no mouse.
	Clearance *string      `json:"c"`
	Pointer   *autoPointer `json:"m"`
}

// autoPointer counts the trusted mouse moves over the page while the check ran, as the SDK sees
// them before it thins them out for the behaviour data.
type autoPointer struct {
	// The moves.
	N *int `json:"n"`
	// Moves that came coalesced with others; -1 when the browser has no getCoalescedEvents.
	K *int `json:"k"`
	// Moves with movementX and movementY both 0 although the position changed.
	Z *int `json:"z"`
	// Moves whose gap from the one before was within 1 ms of the most common gap.
	S *int `json:"s"`
}

// valid reports whether every count is there and within its bounds.
func (p *autoPointer) valid() bool {
	if p.N == nil || p.K == nil || p.Z == nil || p.S == nil {
		return false
	}
	n := *p.N
	return n >= 0 && n <= autoMaxPointerMoves && *p.K >= -1 && *p.K <= n && *p.Z >= 0 && *p.Z <= n && *p.S >= 0 && *p.S <= n
}

// parseAutoProbe reads an auto_check answer; false when it is not one the SDK writes.
func parseAutoProbe(answer string) (autoProbe, bool) {
	var probe autoProbe
	if len(answer) > autoMaxAnswerBytes || json.Unmarshal([]byte(answer), &probe) != nil {
		return autoProbe{}, false
	}
	if probe.Version == nil || *probe.Version != autoAnswerVersion || probe.UserAgent == nil || probe.Language == nil ||
		probe.Findings == nil || utf8.RuneCountInString(*probe.UserAgent) > autoMaxUserAgent ||
		len(*probe.Language) > autoMaxLanguage || len(probe.Findings) > autoMaxFindings ||
		(probe.Clearance != nil && utf8.RuneCountInString(*probe.Clearance) > maxClearanceLength) ||
		(probe.Pointer != nil && !probe.Pointer.valid()) {
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

// autoJudgement is what judgeAuto weighed about an answer, for the log line of judgeAutoCheck.
type autoJudgement struct {
	pass   bool
	result BehaviorResult
	// The pointer summary, nil when the answer had none, and the penalties it earns, which only
	// count while enforce_pointer_signals is on.
	pointer                       *autoPointer
	syntheticPointer, stepPointer bool
	clearanceSent, clearanceValid bool
	// The client's address is in a hosting range, of that prefix length.
	hosting     bool
	hostingBits int
}

// judgeAutoCheck decides whether an auto_check answer lets its visitor through (see judgeAuto),
// and logs one line of what it weighed, so the pointer evidence can be checked against real
// traffic before it counts. The line never holds the address, the User-Agent or the clearance.
func (uc *ChallengeUsecase) judgeAutoCheck(ctx context.Context, challenge *Challenge, answer Answer, signals Signals,
	result BehaviorResult, matched bool) (bool, BehaviorResult) {
	judged := uc.judgeAuto(ctx, challenge, answer, signals, result, matched)
	var n, k, z, s int
	if pointer := judged.pointer; pointer != nil {
		n, k, z, s = *pointer.N, *pointer.K, *pointer.Z, *pointer.S
	}
	log.InfoContext(ctx, "auto check judged", "pass", judged.pass, "mode", uc.auto.mode, "score", judged.result.Score,
		"reasons", judged.result.Reasons, "pointer_type", loggedPointerType(answer.Behavior), "n", n, "k", k, "z", z, "s", s,
		"synthetic_pointer", judged.syntheticPointer, "step_pointer", judged.stepPointer,
		"clearance_sent", judged.clearanceSent, "clearance_valid", judged.clearanceValid,
		"hosting", judged.hosting, "hosting_prefix", judged.hostingBits, "challenge_id", challenge.ID)
	return judged.pass, judged.result
}

// judgeAuto is the decision of judgeAutoCheck. The behaviour as checkBehavior judged it (result),
// less a penalty per finding, must reach the pass mark; the proof of work must solve the challenge;
// the answer must come with the client's address, from the key and website of the challenge
// (matched), and the address must be under its cap of passes. Then, in this order: a hosting
// address never passes; a valid clearance under its caps passes, with reason cleared; without one,
// strict mode refuses, and balanced mode needs the address under its cap of first visits and adds
// reason first_visit, so that a site's backend can refuse such passes for risky actions. The
// result holds the final score and every reason, so the usage log and, through siteverify, the
// site see why a visitor passed or fell back.
func (uc *ChallengeUsecase) judgeAuto(ctx context.Context, challenge *Challenge, answer Answer, signals Signals,
	result BehaviorResult, matched bool) autoJudgement {
	judged := autoJudgement{result: result}
	judged.hostingBits, judged.hosting = uc.auto.hosting.lookup(answer.Client)
	probe, formed := parseAutoProbe(answer.Text)
	if !formed {
		judged.result.Reasons = append(result.Reasons, "malformed_answer")
		return judged
	}
	judged.pointer = probe.Pointer
	judged.syntheticPointer, judged.stepPointer = pointerEvidence(answer.Behavior, probe.Pointer)
	var cleared clearance
	if probe.Clearance != nil && *probe.Clearance != "" {
		judged.clearanceSent = true
		cleared, judged.clearanceValid = parseClearance(uc.auto.clearanceKey, *probe.Clearance, challenge.APIKeyID,
			answer.UserAgent, uc.now())
	}
	if !result.Measured() || result.Verdict == VerdictAutomated {
		return judged
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
	if uc.auto.enforcePointer {
		if judged.syntheticPointer {
			score -= penaltySyntheticPointer
			reasons = append(reasons, "synthetic_pointer")
		}
		if judged.stepPointer {
			score -= penaltyStepPointer
			reasons = append(reasons, "step_pointer")
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
	// Whatever the clearance and the score: the fallback still comes, and a person can solve it.
	if judged.hosting && uc.auto.blockHosting {
		pass = false
		reasons = append(reasons, "hosting_address")
	}
	// A clearance that fails a check counts as none, and so does one over a cap. The caps are only
	// read for an answer that may still pass.
	usable := judged.clearanceValid
	if judged.clearanceSent && !usable {
		reasons = append(reasons, "clearance_invalid")
	}
	if usable && pass && uc.clearanceCapped(ctx, cleared.id) {
		usable = false
		reasons = append(reasons, "clearance_cap")
	}
	switch {
	case usable:
		if pass {
			reasons = append(reasons, "cleared")
		}
	case uc.auto.mode == autoModeStrict:
		pass = false
		reasons = append(reasons, "clearance_required")
	default:
		if pass && uc.firstVisitCapped(ctx, answer.Client) {
			pass = false
			reasons = append(reasons, "first_visit_cap")
		}
		if pass {
			reasons = append(reasons, "first_visit")
		}
	}
	verdict := result.Verdict
	if verdict == VerdictHuman && score < behaviorMinHumanScore {
		verdict = VerdictSuspicious
	}
	// Each cap counts a pass only once it is decided.
	if pass {
		now := uc.now()
		if err := uc.history.CountAutoPass(ctx, answer.Client.String(), now); err != nil {
			log.WarnContext(ctx, "auto pass not counted for its address", "err", err)
		}
		if usable {
			uc.countClearancePass(ctx, cleared.id)
		} else if err := uc.history.CountFirstVisitPass(ctx, answer.Client.String(), now); err != nil {
			log.WarnContext(ctx, "first visit not counted for its address", "err", err)
		}
	}
	judged.pass, judged.result = pass, BehaviorResult{Verdict: verdict, Score: score, Reasons: reasons}
	return judged
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

// firstVisitCapped reports whether addr had its first_visit_passes_per_ip_hour passes without a
// clearance in the last hour. A failure of Redis counts as over the cap, as it does for a
// clearance: this cap is all that limits a visitor who never solved a puzzle.
func (uc *ChallengeUsecase) firstVisitCapped(ctx context.Context, addr netip.Addr) bool {
	if uc.auto.firstVisitPerHour <= 0 {
		return false
	}
	passes, err := uc.history.FirstVisitPassesLastHour(ctx, addr.String(), uc.now())
	if err != nil {
		log.WarnContext(ctx, "first visits of the address not read, counted as over the cap", "err", err)
		return true
	}
	return passes >= uc.auto.firstVisitPerHour
}

// pointerEvidence tells which pointer penalties a summary earns. Only a mouse that made
// pointerMinMoves moves is judged: the SDK counts a mouse's moves only, and a few moves say little.
func pointerEvidence(behavior *Behavior, pointer *autoPointer) (synthetic, step bool) {
	if behavior == nil || behavior.PointerType != "mouse" || pointer == nil || *pointer.N < pointerMinMoves {
		return false, false
	}
	n := *pointer.N
	synthetic = *pointer.K == 0 || *pointer.Z*2 > n
	step = *pointer.S*100 > pointerStepPercent*n
	return synthetic, step
}

// loggedPointerType is the pointer type of behavior for the log line: the client writes it, so
// anything but the types the SDK sends is logged as "other".
func loggedPointerType(behavior *Behavior) string {
	if behavior == nil {
		return ""
	}
	switch behavior.PointerType {
	case "", "mouse", "touch", "pen":
		return behavior.PointerType
	}
	return "other"
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
