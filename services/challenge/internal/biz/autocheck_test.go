package biz

import (
	"context"
	"encoding/json"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"challenge/internal/conf"
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
		{"clean desktop answer", nil, nil, true, VerdictHuman, 1, []string{}},
		{"no pointer at all still passes", noPointer, nil, true, VerdictHuman, 0.85, []string{"no_pointer"}},
		{"one weak finding still passes", func(a *Answer) { a.Text = autoAnswerJSON(chromeUA, "vi-VN", "no_plugins") }, nil,
			true, VerdictHuman, 0.9, []string{"no_plugins"}},
		{"another language in Accept-Language", func(a *Answer) { a.AcceptLanguage = "en-US,en;q=0.9" }, nil,
			true, VerdictHuman, 0.85, []string{"lang_mismatch"}},
		{"a region of the same language is no mismatch", func(a *Answer) { a.Text = autoAnswerJSON(chromeUA, "vi") }, nil,
			true, VerdictHuman, 1, []string{}},
		{"unknown names ignored, each name once", func(a *Answer) {
			a.Text = autoAnswerJSON(chromeUA, "vi-VN", "no_plugins", "no_plugins", "made_up")
		}, nil, true, VerdictHuman, 0.9, []string{"no_plugins"}},
		{"a probe cannot report what only the service finds", func(a *Answer) {
			a.Text = autoAnswerJSON(chromeUA, "vi-VN", "ua_mismatch", "lang_mismatch")
		}, nil, true, VerdictHuman, 1, []string{}},

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
		// Only a pass counts against the address's cap.
		if passes := f.history.passes[visitor.String()]; test.pass != (passes == 1) && test.name != "IP cap reached" {
			t.Errorf("%s: %d passes counted", test.name, passes)
		}
	}
}

func TestAutoCheckCapCountsPasses(t *testing.T) {
	f := newFixture(nil)
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
	}
	for _, test := range tests {
		if _, ok := parseAutoProbe(test.answer); ok != test.ok {
			t.Errorf("%s: got %v, want %v", test.name, ok, test.ok)
		}
	}
}

func TestAutoCheckRulesAreValidated(t *testing.T) {
	for _, bad := range []*conf.AutoCheck{
		{PowDifficulty: -1, MinScorePercent: 80},
		{PowDifficulty: 33, MinScorePercent: 80},
		{PowDifficulty: 18, MinScorePercent: 0},
		{PowDifficulty: 18, MinScorePercent: 101},
		{PowDifficulty: 18, MinScorePercent: 80, PassesPerIpHour: -1},
	} {
		if _, err := newAutoCheckRules(bad); err == nil {
			t.Errorf("%+v accepted", bad)
		}
	}
	rules, err := newAutoCheckRules(&conf.AutoCheck{PowDifficulty: 0, MinScorePercent: 100, PassesPerIpHour: 0})
	if err != nil || rules.minScore != 1 || rules.passesPerHour != 0 {
		t.Fatalf("got %+v, %v", rules, err)
	}
}
