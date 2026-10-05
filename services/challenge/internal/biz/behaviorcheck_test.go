package biz

import (
	"context"
	"math"
	mrand "math/rand/v2"
	"slices"
	"testing"
	"time"

	"challenge/internal/conf"

	"google.golang.org/protobuf/types/known/durationpb"
)

// handPath is a mouse moved by a hand from (fromX, fromY) to (toX, toY): a minimum-jerk curve
// that speeds up and brakes, bending a little, sampled every 16-24 ms from start ms on.
func handPath(random *mrand.Rand, start int, fromX, fromY, toX, toY float64, duration int) []PointerSample {
	var samples []PointerSample
	for t := 0; t <= duration; t += 16 + random.IntN(9) {
		s := float64(t) / float64(duration)
		progress := 10*s*s*s - 15*s*s*s*s + 6*s*s*s*s*s
		bend := 18 * math.Sin(math.Pi*progress)
		samples = append(samples, PointerSample{T: start + t, X: round1(fromX + (toX-fromX)*progress), Y: round1(fromY + (toY-fromY)*progress + bend)})
	}
	return samples
}

// scriptPath moves in a straight line at one speed, every 16 ms.
func scriptPath(start int, fromX, fromY, toX, toY float64, steps int) []PointerSample {
	samples := make([]PointerSample, steps+1)
	for i := range samples {
		share := float64(i) / float64(steps)
		samples[i] = PointerSample{T: start + 16*i, X: round1(fromX + (toX-fromX)*share), Y: round1(fromY + (toY-fromY)*share)}
	}
	return samples
}

// typing is the intervals between the key presses of a person typing n keys.
func typing(random *mrand.Rand, n int) []int {
	intervals := make([]int, n)
	for i := range intervals {
		intervals[i] = 90 + random.IntN(220)
	}
	return intervals
}

// typedAnswer is a person who moves the mouse to the answer box, types five characters and the
// Enter key, and moves on: about four seconds on screen.
func typedAnswer(seed uint64) *Behavior {
	random := mrand.New(mrand.NewPCG(seed, 3))
	pointer := append(handPath(random, 100, -320, 260, 140, 120, 650), handPath(random, 3300, 140, 120, 60, 180, 420)...)
	return &Behavior{Version: 1, ElapsedMs: 4200, Pointer: pointer, PointerType: "mouse", KeyIntervalsMs: typing(random, 5), InputEvents: 5}
}

func with(behavior *Behavior, change func(*Behavior)) *Behavior {
	copied := *behavior
	copied.Pointer = slices.Clone(behavior.Pointer)
	copied.KeyIntervalsMs = slices.Clone(behavior.KeyIntervalsMs)
	change(&copied)
	return &copied
}

func TestCheckBehavior(t *testing.T) {
	typed := typedAnswer(1)
	random := mrand.New(mrand.NewPCG(9, 9))
	dragged := &Behavior{Version: 1, ElapsedMs: 2600, PointerType: "mouse",
		Pointer: append(handPath(random, 200, -200, 300, 20, 190, 700), handPath(random, 1300, 20, 190, 180, 196, 900)...)}
	scripted := with(typed, func(b *Behavior) {
		b.Pointer = append(scriptPath(100, -320, 260, 140, 120, 30), scriptPath(3300, 140, 120, 60, 180, 12)...)
		b.KeyIntervalsMs = []int{50, 50, 50, 50, 50}
	})
	tests := []struct {
		name          string
		behavior      *Behavior
		challengeType string
		sinceIssue    time.Duration
		verdict       string
		reasons       []string
	}{
		{"no behaviour: only the answer decides", nil, TypeImageDistort, time.Minute, VerdictNotMeasured, nil},
		{"a version this check does not know", with(typed, func(b *Behavior) { b.Version = 2 }), TypeImageDistort, time.Minute, VerdictNotMeasured, nil},
		{"typed with the mouse nearby", typed, TypeImageDistort, time.Minute, VerdictHuman, []string{}},
		{"motion_noise typed", with(typed, func(b *Behavior) { b.KeyIntervalsMs = b.KeyIntervalsMs[:4] }), TypeMotionNoise, time.Minute, VerdictHuman, []string{}},
		{"keyboard only", with(typed, func(b *Behavior) { b.Pointer, b.PointerType = nil, "" }), TypeImageDistort, time.Minute, VerdictHuman, []string{"no_pointer"}},
		{"dictation: input events, no key presses", with(typed, func(b *Behavior) { b.KeyIntervalsMs, b.InputEvents = nil, 1 }), TypeImageDistort, time.Minute, VerdictHuman, []string{}},
		{"touch: taps, an on-screen keyboard", with(typed, func(b *Behavior) {
			b.PointerType, b.Pointer = "touch", []PointerSample{{T: 900, X: 120, Y: 118}, {T: 3900, X: 60, Y: 182}}
		}), TypeImageDistort, time.Minute, VerdictHuman, []string{}},
		{"pasted answer", with(typed, func(b *Behavior) { b.KeyIntervalsMs, b.InputEvents, b.PasteEvents = []int{}, 0, 1 }), TypeImageDistort, time.Minute, VerdictHuman, []string{"pasted"}},
		{"slide_puzzle dragged by hand", dragged, TypeSlidePuzzle, time.Minute, VerdictHuman, []string{}},
		{"webdriver alone: suspicious, not refused", with(typed, func(b *Behavior) { b.Webdriver = true }), TypeImageDistort, time.Minute, VerdictSuspicious, []string{"webdriver"}},
		{"straight mouse and even fast typing", scripted, TypeImageDistort, time.Minute, VerdictSuspicious, []string{"even_pointer", "even_typing"}},
		{"all of it under webdriver", with(scripted, func(b *Behavior) { b.Webdriver = true }), TypeImageDistort, time.Minute, VerdictSuspicious, []string{"webdriver", "even_pointer", "even_typing"}},
		{"faster than anyone types", with(typed, func(b *Behavior) { b.KeyIntervalsMs = []int{22, 31, 18, 27, 25} }), TypeImageDistort, time.Minute, VerdictHuman, []string{"fast_typing"}},
		{"events dispatched by a script", with(typed, func(b *Behavior) { b.UntrustedEvents = 1 }), TypeImageDistort, time.Minute, VerdictAutomated, []string{"untrusted_events"}},
		{"value set by a script", with(typed, func(b *Behavior) { b.KeyIntervalsMs, b.InputEvents = nil, 0 }), TypeImageDistort, time.Minute, VerdictAutomated, []string{"scripted_input"}},
		{"slide_puzzle has no answer box", with(dragged, func(b *Behavior) { b.InputEvents = 0 }), TypeSlidePuzzle, time.Minute, VerdictHuman, []string{}},
		{"image_distort in 900 ms", with(typed, func(b *Behavior) { b.ElapsedMs, b.Pointer = 900, nil }), TypeImageDistort, time.Minute, VerdictAutomated, []string{"too_fast"}},
		{"motion_noise in 1 400 ms", with(typed, func(b *Behavior) { b.ElapsedMs, b.Pointer = 1400, nil }), TypeMotionNoise, time.Minute, VerdictAutomated, []string{"too_fast"}},
		{"slide_puzzle in 300 ms", with(dragged, func(b *Behavior) { b.ElapsedMs, b.Pointer = 300, nil }), TypeSlidePuzzle, time.Minute, VerdictAutomated, []string{"too_fast"}},
		{"on screen longer than it existed", typed, TypeImageDistort, 3 * time.Second, VerdictAutomated, []string{"elapsed_beyond_issue"}},
		{"stored without an issue time", typed, TypeImageDistort, math.MaxInt64, VerdictHuman, []string{}},
		{"more than 200 samples", with(typed, func(b *Behavior) { b.Pointer = scriptPath(0, 0, 0, 400, 0, 200) }), TypeImageDistort, time.Minute, VerdictAutomated, []string{"malformed_behavior"}},
		{"more than 64 key intervals", with(typed, func(b *Behavior) { b.KeyIntervalsMs = typing(random, 65) }), TypeImageDistort, time.Minute, VerdictAutomated, []string{"malformed_behavior"}},
		{"NaN coordinate", with(typed, func(b *Behavior) { b.Pointer[3].X = math.NaN() }), TypeImageDistort, time.Minute, VerdictAutomated, []string{"malformed_behavior"}},
		{"samples out of order", with(typed, func(b *Behavior) { b.Pointer[4].T = b.Pointer[3].T - 5 }), TypeImageDistort, time.Minute, VerdictAutomated, []string{"malformed_behavior"}},
		{"sample after the answer", with(typed, func(b *Behavior) { b.Pointer[len(b.Pointer)-1].T = 9000 }), TypeImageDistort, time.Minute, VerdictAutomated, []string{"malformed_behavior"}},
		{"unknown pointer type", with(typed, func(b *Behavior) { b.PointerType = "trackball" }), TypeImageDistort, time.Minute, VerdictAutomated, []string{"malformed_behavior"}},
		{"hidden longer than shown", with(typed, func(b *Behavior) { b.HiddenMs = 5000 }), TypeImageDistort, time.Minute, VerdictAutomated, []string{"malformed_behavior"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := checkBehavior(test.behavior, test.challengeType, test.sinceIssue)
			if result.Verdict != test.verdict || (test.reasons != nil && !slices.Equal(result.Reasons, test.reasons)) {
				t.Fatalf("got %s %v (score %.2f), want %s %v", result.Verdict, result.Reasons, result.Score, test.verdict, test.reasons)
			}
			if result.Measured() && (result.Score < 0 || result.Score > 1) {
				t.Fatalf("score %.2f outside [0, 1]", result.Score)
			}
		})
	}
}

// TestCheckBehaviorPeople shows that many sessions of a person, typed or dragged, all pass, so a
// change of threshold that would start refusing people is noticed.
func TestCheckBehaviorPeople(t *testing.T) {
	for seed := range uint64(300) {
		if result := checkBehavior(typedAnswer(seed), TypeImageDistort, time.Minute); result.Verdict != VerdictHuman || result.Score != 1 {
			t.Fatalf("person %d: %s %v %.2f", seed, result.Verdict, result.Reasons, result.Score)
		}
	}
}

// A right answer with automated behaviour does not solve the challenge; with suspicious behaviour
// it does, and the site decides.
func TestCheckRefusesAutomatedAnswers(t *testing.T) {
	now := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		behavior *Behavior
		right    bool
		verdict  string
	}{
		{typedAnswer(1), true, VerdictHuman},
		{with(typedAnswer(1), func(b *Behavior) { b.Webdriver = true }), true, VerdictSuspicious},
		{with(typedAnswer(1), func(b *Behavior) { b.UntrustedEvents = 2 }), false, VerdictAutomated},
	} {
		challenges := takeOnce{}
		usecase, err := NewChallengeUsecase(nil, challenges, nil, nil,
			&conf.Challenge{Ttl: durationpb.New(2 * time.Minute)}, &conf.Quota{TimeZone: "Asia/Ho_Chi_Minh"})
		if err != nil {
			t.Fatal(err)
		}
		usecase.now = func() time.Time { return now }
		_ = challenges.Save(context.Background(), &Challenge{ID: "c1", Type: TypeImageDistort, Answer: "Xy7Qz", IssuedAt: now.Add(-10 * time.Second)})
		_, right, result, err := usecase.Check(context.Background(), "c1", "xy7qz", test.behavior)
		if err != nil || right != test.right || result.Verdict != test.verdict {
			t.Fatalf("got right=%v %s err=%v, want right=%v %s", right, result.Verdict, err, test.right, test.verdict)
		}
	}
}
