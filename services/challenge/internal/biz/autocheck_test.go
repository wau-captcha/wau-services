package biz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"challenge/internal/conf"

	"github.com/go-kratos/kratos/v3/log"
	"google.golang.org/protobuf/types/known/durationpb"
)

// autoIssued is an auto_check challenge as New stores it for a visitor of shop.example.com on Chrome.
func autoIssued(now time.Time) *Challenge {
	challenge := issued(now)
	challenge.Type, challenge.Answer, challenge.PowDifficulty = TypeAutoCheck, "", 18
	return challenge
}

// autoNonce solves the 18-bit proof of work of autoIssued once for all the tests.
var autoNonce = sync.OnceValue(func() string { return solvePow(powSalt, testChallengeID, 18) })

// autoBehavior is a visitor who moved the mouse over the page while the check ran: two strokes that
// speed up and brake, 2.5 s in all.
func autoBehavior() *Behavior {
	var pointer []PointerSample
	t := 200
	for stroke := range 2 {
		for i := range 20 {
			u := float64(i) / 19
			eased := u * u * (3 - 2*u)
			pointer = append(pointer, PointerSample{T: t, X: 40 + 260*eased + float64(stroke)*30, Y: 120 - 60*eased})
			t += 16 + i%3*4
		}
		t += 400
	}
	return &Behavior{Version: 1, ElapsedMs: 2500, PointerType: "mouse", Pointer: pointer}
}

func autoAnswerJSON(userAgent, language string, findings ...string) string {
	if findings == nil {
		findings = []string{}
	}
	data, _ := json.Marshal(map[string]any{"v": 1, "ua": userAgent, "lang": language, "f": findings})
	return string(data)
}

// autoAnswered is a clean answer to autoIssued from the same visitor, as the SDK sends it.
func autoAnswered() Answer {
	answer := answered()
	answer.Text = autoAnswerJSON(chromeUA, "vi-VN")
	answer.Behavior = autoBehavior()
	answer.PowNonce = autoNonce()
	return answer
}

// The answers here are the SDK 1.6.0 kind, without "c" or "m", from a residential address in
// balanced mode: they are judged as before, and a pass adds first_visit.
func TestAutoCheckPass(t *testing.T) {
	noPointer := func(a *Answer) { a.Behavior.Pointer, a.Behavior.PointerType = nil, "" }
	tests := []struct {
		name    string
		answer  func(*Answer)
		history func(*fakeHistory)
		pass    bool
		verdict string
		score   float64
		reasons []string
	}{
		{"clean desktop answer", nil, nil, true, VerdictHuman, 1, []string{"first_visit"}},
		{"no pointer at all still passes", noPointer, nil, true, VerdictHuman, 0.85, []string{"no_pointer", "first_visit"}},
		{"one weak finding still passes", func(a *Answer) { a.Text = autoAnswerJSON(chromeUA, "vi-VN", "no_plugins") }, nil,
			true, VerdictHuman, 0.9, []string{"no_plugins", "first_visit"}},
		{"another language in Accept-Language", func(a *Answer) { a.AcceptLanguage = "en-US,en;q=0.9" }, nil,
			true, VerdictHuman, 0.85, []string{"lang_mismatch", "first_visit"}},
		{"a region of the same language is no mismatch", func(a *Answer) { a.Text = autoAnswerJSON(chromeUA, "vi") }, nil,
			true, VerdictHuman, 1, []string{"first_visit"}},
		{"unknown names ignored, each name once", func(a *Answer) {
			a.Text = autoAnswerJSON(chromeUA, "vi-VN", "no_plugins", "no_plugins", "made_up")
		}, nil, true, VerdictHuman, 0.9, []string{"no_plugins", "first_visit"}},
		{"a probe cannot report what only the service finds", func(a *Answer) {
			a.Text = autoAnswerJSON(chromeUA, "vi-VN", "ua_mismatch", "lang_mismatch")
		}, nil, true, VerdictHuman, 1, []string{"first_visit"}},

		{"webdriver", func(a *Answer) { a.Behavior.Webdriver = true }, nil, false, VerdictSuspicious, 0.4, []string{"webdriver"}},
		{"missing proof of work", func(a *Answer) { a.PowNonce = "" }, nil, false, VerdictSuspicious, 0.4, []string{"no_pow"}},
		{"invalid proof of work", func(a *Answer) { a.PowNonce = failingPow(powSalt, testChallengeID, 18) }, nil,
			false, VerdictAutomated, 0, []string{"pow_invalid"}},
		{"ua_mismatch plus no_pointer", func(a *Answer) {
			noPointer(a)
			a.Text = autoAnswerJSON("Mozilla/5.0 (X11; Linux x86_64) Chrome/141.0.0.0", "vi-VN")
		}, nil, false, VerdictSuspicious, 0.45, []string{"no_pointer", "ua_mismatch"}},
		{"patched_natives plus no_pointer", func(a *Answer) {
			noPointer(a)
			a.Text = autoAnswerJSON(chromeUA, "vi-VN", "patched_natives")
		}, nil, false, VerdictSuspicious, 0.45, []string{"no_pointer", "patched_natives"}},
		{"outer_zero plus software_gl", func(a *Answer) { a.Text = autoAnswerJSON(chromeUA, "vi-VN", "software_gl", "outer_zero") }, nil,
			false, VerdictSuspicious, 0.45, []string{"outer_zero", "software_gl"}},
		{"bad address history", nil, func(h *fakeHistory) { h.automated[visitor.String()] = 6 }, false, VerdictHuman, 0.7,
			[]string{"ip_bad_history"}},
		{"IP cap reached", nil, func(h *fakeHistory) { h.passes[visitor.String()] = 30 }, false, VerdictHuman, 1, []string{"auto_pass_cap"}},
		{"origin change", func(a *Answer) { a.Origin = "https://evil.test" }, nil, false, VerdictHuman, 1, []string{"origin_changed"}},
		{"another site key", func(a *Answer) { a.SiteKey = "pk_other" }, nil, false, VerdictHuman, 1, []string{"site_key_mismatch"}},
		{"malformed answer", func(a *Answer) { a.Text = `{"v":1,"ua":"x","lang":"vi","f":[1]}` }, nil, false, VerdictHuman, 1,
			[]string{"malformed_answer"}},
		{"answer over 2 KiB", func(a *Answer) { a.Text += strings.Repeat(" ", 2<<10) }, nil, false, VerdictHuman, 1, []string{"malformed_answer"}},
		{"answer without the client's address", func(a *Answer) { a.Client, a.Origin, a.UserAgent = netip.Addr{}, "", "" }, nil,
			false, VerdictHuman, 1, []string{"no_client_address"}},
		{"elapsed under 1000 ms", func(a *Answer) { a.Behavior.ElapsedMs = 900; a.Behavior.Pointer = nil }, nil,
			false, VerdictAutomated, 0, []string{"too_fast"}},
		{"no behaviour", func(a *Answer) { a.Behavior = nil }, nil, false, VerdictNotMeasured, 0, nil},
	}
	for _, test := range tests {
		f := newFixture(nil)
		f.challenges.stored[testChallengeID] = autoIssued(f.now)
		if test.history != nil {
			test.history(f.history)
		}
		answer := autoAnswered()
		if test.answer != nil {
			test.answer(&answer)
		}
		outcome, err := f.usecase.Check(context.Background(), answer)
		if err != nil {
			t.Errorf("%s: %v", test.name, err)
			continue
		}
		got := outcome.Behavior
		if outcome.Right != test.pass || got.Verdict != test.verdict || got.Score != test.score || !slices.Equal(got.Reasons, test.reasons) {
			t.Errorf("%s: got pass %v, %s %.2f %q; want %v, %s %.2f %q", test.name, outcome.Right, got.Verdict, got.Score, got.Reasons,
				test.pass, test.verdict, test.score, test.reasons)
		}
		// Only a pass counts against the address's caps.
		if passes := f.history.passes[visitor.String()]; test.pass != (passes == 1) && test.name != "IP cap reached" {
			t.Errorf("%s: %d passes counted", test.name, passes)
		}
		if visits := f.history.firstVisits[visitor.String()]; test.pass != (visits == 1) {
			t.Errorf("%s: %d first visits counted", test.name, visits)
		}
	}
}

func TestAutoCheckCapCountsPasses(t *testing.T) {
	// Without the cap on first visits, which is tested with the owner's bot.
	f := newFixture(nil, func(c *conf.AutoCheck) { c.FirstVisitPassesPerIpHour = 0 })
	for i := range 31 {
		f.challenges.stored[testChallengeID] = autoIssued(f.now)
		outcome, err := f.usecase.Check(context.Background(), autoAnswered())
		if err != nil {
			t.Fatal(err)
		}
		if outcome.Right != (i < 30) {
			t.Fatalf("answer %d: pass %v; want the first 30 to pass", i+1, outcome.Right)
		}
	}
	if got := f.history.passes[visitor.String()]; got != 30 {
		t.Fatalf("%d passes counted, want 30", got)
	}
}

func TestAutoCheckFallbackIsNotAWrongAnswer(t *testing.T) {
	f := newFixture(nil)
	addr := visitor.String()
	// Falls back: webdriver leaves it suspicious, not automated.
	f.challenges.stored[testChallengeID] = autoIssued(f.now)
	fallback := autoAnswered()
	fallback.Behavior.Webdriver = true
	if outcome, err := f.usecase.Check(context.Background(), fallback); err != nil || outcome.Right {
		t.Fatalf("got %v, %+v", err, outcome)
	}
	if f.history.wrong[addr] != 0 || f.history.automated[addr] != 0 {
		t.Fatalf("a fallback counted: %d wrong, %d automated; want none", f.history.wrong[addr], f.history.automated[addr])
	}
	// Judged automated: counted as automated, and as wrong.
	f.challenges.stored[testChallengeID] = autoIssued(f.now)
	scripted := autoAnswered()
	scripted.UserAgent = "python-requests/2.32.3"
	if outcome, err := f.usecase.Check(context.Background(), scripted); err != nil || outcome.Behavior.Verdict != VerdictAutomated {
		t.Fatalf("got %v, %+v", err, outcome)
	}
	if f.history.wrong[addr] != 1 || f.history.automated[addr] != 1 {
		t.Fatalf("an automated answer: %d wrong, %d automated; want 1 and 1", f.history.wrong[addr], f.history.automated[addr])
	}
	// A wrong answer to another type still counts as wrong.
	f.challenges.stored[testChallengeID] = issued(f.now)
	wrong := answered()
	wrong.Text = "wrong"
	if _, err := f.usecase.Check(context.Background(), wrong); err != nil {
		t.Fatal(err)
	}
	if f.history.wrong[addr] != 2 {
		t.Fatalf("a wrong image_distort answer: %d wrong, want 2", f.history.wrong[addr])
	}
}

func TestFallbackType(t *testing.T) {
	all := []string{TypeImageDistort, TypeMotionNoise, TypeSlidePuzzle, TypeAnimalJump, TypeObjectRotate, TypeAutoCheck}
	tests := []struct {
		name      string
		allowed   []string
		requested string
		want      string
	}{
		{"the requested type when allowed", all, TypeAnimalJump, TypeAnimalJump},
		{"no request: the first of the order", all, "", TypeSlidePuzzle},
		{"a request the key does not allow", []string{TypeImageDistort, TypeAutoCheck}, TypeSlidePuzzle, TypeImageDistort},
		{"an unknown request", all, "behavioral_advanced", TypeSlidePuzzle},
		{"never auto_check, even when asked", all, TypeAutoCheck, TypeSlidePuzzle},
		{"the order goes on", []string{TypeAutoCheck, TypeMotionNoise, TypeAnimalJump}, "", TypeAnimalJump},
		{"then any other allowed type", []string{TypeAutoCheck, TypeObjectRotate}, "", TypeObjectRotate},
		{"an allowed type no kind serves", []string{TypeAutoCheck, "behavioral_advanced"}, "", ""},
		{"nothing interactive allowed", []string{TypeAutoCheck}, TypeSlidePuzzle, ""},
	}
	for _, test := range tests {
		if got := fallbackType(test.allowed, test.requested); got != test.want {
			t.Errorf("%s: got %q, want %q", test.name, got, test.want)
		}
	}
}

func TestNewAutoCheck(t *testing.T) {
	site := scrapingSite()
	site.ChallengeTypes = []string{TypeImageDistort, TypeSlidePuzzle, TypeAutoCheck}
	f := newFixture(site)
	challenge, _, err := f.usecase.New(context.Background(), NewRequest{
		SiteKey: "pk_shop", ChallengeType: TypeAutoCheck, FallbackType: TypeImageDistort, Client: visitor, UserAgent: chromeUA,
	})
	if err != nil {
		t.Fatal(err)
	}
	stored := f.challenges.stored[challenge.ID]
	switch {
	case challenge.FallbackType != TypeImageDistort:
		t.Fatalf("fallback %q, want image_distort", challenge.FallbackType)
	case stored.PowDifficulty != 18 || len(stored.PowSalt) != 16:
		t.Fatalf("pow %d bits, %d-byte salt; want 18 and 16", stored.PowDifficulty, len(stored.PowSalt))
	case stored.Answer != "" || len(challenge.Media.Data) != 0 || challenge.Media.MIMEType != "" || challenge.Media.Width != 0:
		t.Fatalf("auto_check has an answer or media: %q %+v", stored.Answer, challenge.Media)
	}
	// The other types keep their difficulty and get no fallback.
	other, _, err := f.usecase.New(context.Background(), NewRequest{
		SiteKey: "pk_shop", ChallengeType: TypeSlidePuzzle, FallbackType: TypeImageDistort, Client: visitor, UserAgent: chromeUA,
	})
	if err != nil || other.FallbackType != "" || other.PowDifficulty != 8 {
		t.Fatalf("slide_puzzle: %v, fallback %q, pow %d bits", err, other.FallbackType, other.PowDifficulty)
	}
	// Not allowed for the key: refused like any other type.
	site.ChallengeTypes = []string{TypeImageDistort}
	f = newFixture(site)
	if _, _, err := f.usecase.New(context.Background(), NewRequest{SiteKey: "pk_shop", ChallengeType: TypeAutoCheck}); err != ErrChallengeTypeNotAllowed {
		t.Fatalf("auto_check not allowed: %v", err)
	}
	if _, _, err := Sample(TypeAutoCheck); err == nil {
		t.Fatal("Sample(auto_check) has media to sample")
	}
}

func TestParseAutoProbe(t *testing.T) {
	long := strings.Repeat("a", 513)
	tests := []struct {
		name   string
		answer string
		ok     bool
	}{
		{"the SDK's answer", autoAnswerJSON(chromeUA, "vi-VN", "no_plugins"), true},
		{"no findings", autoAnswerJSON(chromeUA, "vi-VN"), true},
		{"not JSON", "abcde", false},
		{"a list", `[1, 2]`, false},
		{"null", `null`, false},
		{"another version", `{"v":2,"ua":"x","lang":"vi","f":[]}`, false},
		{"v not a number", `{"v":"1","ua":"x","lang":"vi","f":[]}`, false},
		{"no ua", `{"v":1,"lang":"vi","f":[]}`, false},
		{"no lang", `{"v":1,"ua":"x","f":[]}`, false},
		{"no f", `{"v":1,"ua":"x","lang":"vi"}`, false},
		{"f not a list of strings", `{"v":1,"ua":"x","lang":"vi","f":["a",2]}`, false},
		{"ua over 512 characters", autoAnswerJSON(long, "vi"), false},
		{"lang too long", autoAnswerJSON("x", strings.Repeat("a", 36)), false},
		{"over 16 findings", autoAnswerJSON("x", "vi", strings.Split(strings.Repeat("a,", 16)+"a", ",")...), false},
		{"a finding too long", autoAnswerJSON("x", "vi", strings.Repeat("a", 33)), false},
		{"over 2 KiB", autoAnswerJSON("x", "vi") + strings.Repeat(" ", 2<<10), false},

		{"1.7.0: a clearance and pointer counts", autoAnswerWith("abc.def", pointerCounts(90, 30, 2, 40)), true},
		{"no getCoalescedEvents", autoAnswerWith("", pointerCounts(90, -1, 2, 40)), true},
		{"c and m null", `{"v":1,"ua":"x","lang":"vi","f":[],"c":null,"m":null}`, true},
		{"c empty", `{"v":1,"ua":"x","lang":"vi","f":[],"c":""}`, true},
		{"c of 256 characters", autoAnswerWith(strings.Repeat("a", 256), nil), true},
		{"c over 256 characters", autoAnswerWith(strings.Repeat("a", 257), nil), false},
		{"c not a string", `{"v":1,"ua":"x","lang":"vi","f":[],"c":5}`, false},
		{"m of 100000 moves", autoAnswerWith("", pointerCounts(100000, 0, 0, 0)), true},
		{"m over 100000 moves", autoAnswerWith("", pointerCounts(100001, 0, 0, 0)), false},
		{"m without s", `{"v":1,"ua":"x","lang":"vi","f":[],"m":{"n":5,"k":0,"z":0}}`, false},
		{"m empty", `{"v":1,"ua":"x","lang":"vi","f":[],"m":{}}`, false},
		{"m with a fraction", `{"v":1,"ua":"x","lang":"vi","f":[],"m":{"n":5.5,"k":0,"z":0,"s":0}}`, false},
		{"m with a string", `{"v":1,"ua":"x","lang":"vi","f":[],"m":{"n":"5","k":0,"z":0,"s":0}}`, false},
		{"m not an object", `{"v":1,"ua":"x","lang":"vi","f":[],"m":[5,0,0,0]}`, false},
		{"n negative", autoAnswerWith("", pointerCounts(-1, -1, 0, 0)), false},
		{"k under -1", autoAnswerWith("", pointerCounts(5, -2, 0, 0)), false},
		{"k over n", autoAnswerWith("", pointerCounts(5, 6, 0, 0)), false},
		{"z over n", autoAnswerWith("", pointerCounts(5, 0, 6, 0)), false},
		{"z negative", autoAnswerWith("", pointerCounts(5, 0, -1, 0)), false},
		{"s over n", autoAnswerWith("", pointerCounts(5, 0, 0, 6)), false},
		{"s negative", autoAnswerWith("", pointerCounts(5, 0, 0, -1)), false},
	}
	for _, test := range tests {
		if _, ok := parseAutoProbe(test.answer); ok != test.ok {
			t.Errorf("%s: got %v, want %v", test.name, ok, test.ok)
		}
	}
}

func TestAutoCheckRulesAreValidated(t *testing.T) {
	for name, change := range map[string]func(*conf.AutoCheck){
		"pow_difficulty -1":                 func(c *conf.AutoCheck) { c.PowDifficulty = -1 },
		"pow_difficulty 33":                 func(c *conf.AutoCheck) { c.PowDifficulty = 33 },
		"min_score_percent 0":               func(c *conf.AutoCheck) { c.MinScorePercent = 0 },
		"min_score_percent 101":             func(c *conf.AutoCheck) { c.MinScorePercent = 101 },
		"passes_per_ip_hour -1":             func(c *conf.AutoCheck) { c.PassesPerIpHour = -1 },
		"mode missing":                      func(c *conf.AutoCheck) { c.Mode = "" },
		"mode lenient":                      func(c *conf.AutoCheck) { c.Mode = "lenient" },
		"first_visit_passes_per_ip_hour -1": func(c *conf.AutoCheck) { c.FirstVisitPassesPerIpHour = -1 },
		"clearance_passes_per_hour -1":      func(c *conf.AutoCheck) { c.ClearancePassesPerHour = -1 },
		"clearance_passes_per_day -1":       func(c *conf.AutoCheck) { c.ClearancePassesPerDay = -1 },
		"clearance_ttl missing":             func(c *conf.AutoCheck) { c.ClearanceTtl = nil },
		"clearance_ttl 59m":                 func(c *conf.AutoCheck) { c.ClearanceTtl = durationpb.New(59 * time.Minute) },
		"clearance_ttl 31 days":             func(c *conf.AutoCheck) { c.ClearanceTtl = durationpb.New(31 * 24 * time.Hour) },
		"extra_hosting_cidrs not a block":   func(c *conf.AutoCheck) { c.ExtraHostingCidrs = "198.51.100.0/24, 198.51.100.7" },
		"extra_hosting_cidrs bad length":    func(c *conf.AutoCheck) { c.ExtraHostingCidrs = "198.51.100.0/33" },
		"clearance_secret missing":          func(c *conf.AutoCheck) { c.ClearanceSecret = "" },
		"clearance_secret blank":            func(c *conf.AutoCheck) { c.ClearanceSecret = "  " },
	} {
		bad := testAutoConf()
		change(bad)
		if _, err := newAutoCheckRules(bad); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	// The bounds themselves, and 0 for every cap.
	good := testAutoConf()
	good.PowDifficulty, good.MinScorePercent, good.PassesPerIpHour = 0, 100, 0
	good.Mode, good.FirstVisitPassesPerIpHour, good.ClearancePassesPerHour, good.ClearancePassesPerDay = " Strict ", 0, 0, 0
	good.ClearanceTtl, good.ExtraHostingCidrs = durationpb.New(time.Hour), "198.51.100.0/24, 2001:db8::/32"
	rules, err := newAutoCheckRules(good)
	if err != nil || rules.minScore != 1 || rules.passesPerHour != 0 || rules.mode != autoModeStrict || rules.clearanceTTL != time.Hour {
		t.Fatalf("got %+v, %v", rules, err)
	}
	good.ClearanceTtl = durationpb.New(30 * 24 * time.Hour)
	if _, err := newAutoCheckRules(good); err != nil {
		t.Fatalf("30 days refused: %v", err)
	}
	// The settings of configs/config.yaml.
	rules, err = newAutoCheckRules(testAutoConf())
	if err != nil || rules.mode != autoModeBalanced || rules.firstVisitPerHour != 5 || rules.clearanceTTL != 168*time.Hour ||
		rules.clearancePerHour != 6 || rules.clearancePerDay != 24 || !rules.blockHosting || rules.enforcePointer {
		t.Fatalf("got %+v, %v", rules, err)
	}
}

// autoAnswerWith is an answer of SDK 1.7.0 on from the visitor of autoAnswered: with "c" when
// clearance is not empty and "m" when pointer is not nil.
func autoAnswerWith(clearance string, pointer map[string]int) string {
	answer := map[string]any{"v": 1, "ua": chromeUA, "lang": "vi-VN", "f": []string{}}
	if clearance != "" {
		answer["c"] = clearance
	}
	if pointer != nil {
		answer["m"] = pointer
	}
	data, _ := json.Marshal(answer)
	return string(data)
}

func pointerCounts(n, k, z, s int) map[string]int {
	return map[string]int{"n": n, "k": k, "z": z, "s": s}
}

// hostingVisitor is an address of AWS EC2 in us-east-2: 3.16.0.0/14 in ip-ranges.json, merged
// into 3.16.0.0/12 in hostingranges.txt.
var hostingVisitor = netip.MustParseAddr("3.16.0.10")

// checkAuto stores autoIssued as issued to client and checks answer from client.
func checkAuto(t *testing.T, f *fixture, client netip.Addr, answer Answer) *Outcome {
	t.Helper()
	challenge := autoIssued(f.now)
	challenge.ClientIP, answer.Client = client.String(), client
	f.challenges.stored[testChallengeID] = challenge
	outcome, err := f.usecase.Check(context.Background(), answer)
	if err != nil {
		t.Fatal(err)
	}
	return outcome
}

// botAnswer is the owner's bot: a browser automated at start-up whose every probe is faked clean,
// with the User-Agent and the language of its headers, a valid proof of work and one smooth,
// accelerated mouse path over 1600 ms, sent as the SDK thins it, a sample per 16 ms. No clearance.
func botAnswer() Answer {
	var pointer []PointerSample
	for i := range 101 {
		u := float64(i) / 100
		eased := u * u * u * (u*(6*u-15) + 10)
		pointer = append(pointer, PointerSample{T: 150 + i*16, X: 40 + 560*eased, Y: 380 - 220*eased})
	}
	answer := autoAnswered()
	answer.Behavior = &Behavior{Version: 1, ElapsedMs: 1900, PointerType: "mouse", Pointer: pointer}
	return answer
}

func TestOwnersBot(t *testing.T) {
	expect := func(name string, got *Outcome, pass bool, reasons ...string) {
		t.Helper()
		if got.Right != pass || !slices.Equal(got.Behavior.Reasons, reasons) {
			t.Errorf("%s: pass %v, %s %.2f %q; want pass %v, %q", name, got.Right, got.Behavior.Verdict, got.Behavior.Score,
				got.Behavior.Reasons, pass, reasons)
		}
	}
	// It still looks like a person to every probe: human, score 1. That no longer lets it through.
	strict := newFixture(nil, func(c *conf.AutoCheck) { c.Mode = "strict" })
	got := checkAuto(t, strict, visitor, botAnswer())
	expect("strict", got, false, "clearance_required")
	if got.Behavior.Verdict != VerdictHuman || got.Behavior.Score != 1 {
		t.Errorf("strict: %s %.2f; want human 1", got.Behavior.Verdict, got.Behavior.Score)
	}

	balanced := newFixture(nil)
	expect("balanced, from a hosting address", checkAuto(t, balanced, hostingVisitor, botAnswer()), false, "hosting_address")
	for i := range 5 {
		expect(fmt.Sprintf("balanced, residential, answer %d", i+1), checkAuto(t, balanced, visitor, botAnswer()), true, "first_visit")
	}
	expect("balanced, residential, answer 6 within the hour", checkAuto(t, balanced, visitor, botAnswer()), false, "first_visit_cap")
	if passes, visits := balanced.history.passes[visitor.String()], balanced.history.firstVisits[visitor.String()]; passes != 5 || visits != 5 {
		t.Errorf("%d passes and %d first visits counted, want 5 and 5", passes, visits)
	}

	// No move ever came coalesced: logged while the pointer signals are not enforced, refused once they are.
	neverCoalesced := botAnswer()
	neverCoalesced.Text = autoAnswerWith("", pointerCounts(90, 0, 0, 0))
	expect("pointer signals logged only", checkAuto(t, newFixture(nil), visitor, neverCoalesced), true, "first_visit")
	enforced := newFixture(nil, func(c *conf.AutoCheck) { c.EnforcePointerSignals = true })
	expect("pointer signals enforced", checkAuto(t, enforced, visitor, neverCoalesced), false, "synthetic_pointer")
}

func TestPointerEvidence(t *testing.T) {
	mouse, touch := &Behavior{PointerType: "mouse"}, &Behavior{PointerType: "touch"}
	summary := func(n, k, z, s int) *autoPointer { return &autoPointer{N: &n, K: &k, Z: &z, S: &s} }
	tests := []struct {
		name            string
		behavior        *Behavior
		pointer         *autoPointer
		synthetic, step bool
	}{
		{"a hand: some moves coalesced, uneven gaps", mouse, summary(90, 30, 2, 40), false, false},
		{"no move coalesced", mouse, summary(90, 0, 0, 40), true, false},
		{"a browser without getCoalescedEvents", mouse, summary(90, -1, 0, 40), false, false},
		{"over half the moves without movement", mouse, summary(90, 30, 46, 40), true, false},
		{"half is not over half", mouse, summary(90, 30, 45, 40), false, false},
		{"over 80 percent on the most common gap", mouse, summary(90, 30, 0, 73), false, true},
		{"80 percent is not over it", mouse, summary(90, 30, 0, 72), false, false},
		{"both", mouse, summary(90, 0, 90, 90), true, true},
		{"19 moves say too little", mouse, summary(19, 0, 19, 19), false, false},
		{"20 moves are judged", mouse, summary(20, 0, 0, 0), true, false},
		{"a pen is not judged", &Behavior{PointerType: "pen"}, summary(90, 0, 90, 90), false, false},
		{"touch is not judged", touch, summary(90, 0, 90, 90), false, false},
		{"no summary", mouse, nil, false, false},
		{"no behaviour", nil, summary(90, 0, 90, 90), false, false},
	}
	for _, test := range tests {
		if synthetic, step := pointerEvidence(test.behavior, test.pointer); synthetic != test.synthetic || step != test.step {
			t.Errorf("%s: got %v %v, want %v %v", test.name, synthetic, step, test.synthetic, test.step)
		}
	}
}

func TestPointerSignalsCountOnlyWhenEnforced(t *testing.T) {
	tests := []struct {
		name    string
		enforce bool
		pointer map[string]int
		pass    bool
		score   float64
		reasons []string
	}{
		{"a hand, enforced", true, pointerCounts(90, 30, 2, 40), true, 1, []string{"first_visit"}},
		{"synthetic, logged only", false, pointerCounts(90, 0, 0, 40), true, 1, []string{"first_visit"}},
		{"synthetic, enforced", true, pointerCounts(90, 0, 0, 40), false, 0.7, []string{"synthetic_pointer"}},
		{"a timer, logged only", false, pointerCounts(90, 30, 0, 80), true, 1, []string{"first_visit"}},
		{"a timer, enforced, alone stays at the pass mark", true, pointerCounts(90, 30, 0, 80), true, 0.8, []string{"step_pointer", "first_visit"}},
		{"both, enforced", true, pointerCounts(90, 0, 90, 90), false, 0.5, []string{"synthetic_pointer", "step_pointer"}},
	}
	for _, test := range tests {
		f := newFixture(nil, func(c *conf.AutoCheck) { c.EnforcePointerSignals = test.enforce })
		answer := autoAnswered()
		answer.Text = autoAnswerWith("", test.pointer)
		got := checkAuto(t, f, visitor, answer)
		if got.Right != test.pass || got.Behavior.Score != test.score || !slices.Equal(got.Behavior.Reasons, test.reasons) {
			t.Errorf("%s: got pass %v, %.2f %q; want %v, %.2f %q", test.name, got.Right, got.Behavior.Score, got.Behavior.Reasons,
				test.pass, test.score, test.reasons)
		}
	}
}

func TestAutoCheckLogLine(t *testing.T) {
	var output bytes.Buffer
	previous := log.Default()
	log.SetDefault(slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { log.SetDefault(previous) })

	f := newFixture(nil)
	clearance, err := issueClearance(clearanceKey(testTokenSecret), "key-1", chromeUA, f.now.Add(-time.Hour), 168*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cleared := autoAnswered()
	cleared.Text = autoAnswerWith(clearance, pointerCounts(90, 30, 2, 40))
	// Answers that carry the address and the User-Agent where nothing checks them yet.
	malformed := autoAnswered()
	malformed.Text = `{"v":1,"ua":"` + chromeUA + `","lang":"vi","f":["` + visitor.String() + `"],"c":"` + clearance + `","m":{}}`
	strangePointer := autoAnswered()
	strangePointer.Behavior.PointerType = visitor.String()
	checkAuto(t, f, visitor, cleared)
	checkAuto(t, f, hostingVisitor, cleared)
	checkAuto(t, f, visitor, malformed)
	checkAuto(t, f, visitor, strangePointer)

	text := output.String()
	if got := strings.Count(text, "auto check judged"); got != 4 {
		t.Fatalf("%d lines, want one per answer:\n%s", got, text)
	}
	encoded, signature, _ := strings.Cut(clearance, ".")
	for _, secret := range []string{visitor.String(), hostingVisitor.String(), chromeUA, "Mozilla", "Chrome/141", clearance, encoded, signature} {
		if strings.Contains(text, secret) {
			t.Errorf("the log holds %q:\n%s", secret, text)
		}
	}
	for _, field := range []string{"pass=true mode=balanced score=1 reasons=[cleared] pointer_type=mouse n=90 k=30 z=2 s=40",
		"synthetic_pointer=false step_pointer=false clearance_sent=true clearance_valid=true hosting=false hosting_prefix=0",
		"pass=false", "reasons=[hosting_address]", "hosting=true hosting_prefix=12", "reasons=[malformed_answer]",
		"pointer_type=other", "challenge_id=" + testChallengeID} {
		if !strings.Contains(text, field) {
			t.Errorf("the log lacks %q:\n%s", field, text)
		}
	}
}
