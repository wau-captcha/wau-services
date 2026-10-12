package biz

import (
	"encoding/json"
	"math"
	mrand "math/rand/v2"
	"strings"
	"testing"
	"time"
)

// rotateRoundAnswerJSON is one round of an object_rotate answer as the SDKs send it.
type rotateRoundAnswerJSON struct {
	F    float64     `json:"f"`
	S    [][]float64 `json:"s"`
	Done float64     `json:"done"`
}

// turnBy answers a round with presses in the given directions, the first 600 ms in and the rest
// 300 ms apart, Done 400 ms after the last, ending on the frame they reach.
func turnBy(directions ...float64) rotateRoundAnswerJSON {
	round := rotateRoundAnswerJSON{S: [][]float64{}}
	ms, sum := 300.0, 0.0
	for _, direction := range directions {
		ms += 300
		sum += direction
		round.S = append(round.S, []float64{ms, direction})
	}
	round.F = math.Mod(math.Mod(sum, rotateFrames)+rotateFrames, rotateFrames)
	round.Done = ms + 400
	return round
}

// presses is n presses: right ones of them to the right, the rest to the left.
func presses(n, right int) []float64 {
	directions := make([]float64, n)
	for i := range directions {
		directions[i] = -1
		if i < right {
			directions[i] = 1
		}
	}
	return directions
}

func rotateAnswerJSON(rounds ...rotateRoundAnswerJSON) string {
	data, _ := json.Marshal(map[string]any{"r": rounds})
	return string(data)
}

func TestCheckRotate(t *testing.T) {
	targets := []rotateTarget{{Target: 3, Object: "dog"}, {Target: 6, Object: "teapot"}}
	late := 10 * time.Second
	right := func() []rotateRoundAnswerJSON { return []rotateRoundAnswerJSON{turnBy(1, 1, 1), turnBy(-1, -1)} }
	// with changes the right answer's round with change.
	with := func(round int, change func(*rotateRoundAnswerJSON)) string {
		rounds := right()
		change(&rounds[round])
		return rotateAnswerJSON(rounds...)
	}
	press := func(i int, ms, direction float64) func(*rotateRoundAnswerJSON) {
		return func(round *rotateRoundAnswerJSON) { round.S[i] = []float64{ms, direction} }
	}
	// Round 1 is Done at 1600 ms, round 2 at 1300: 2.9 s on screen in all.
	soon := 2899 * time.Millisecond
	tests := []struct {
		name       string
		answer     string
		sinceIssue time.Duration
		reason     string
	}{
		{"right", rotateAnswerJSON(right()...), late, ""},
		{"right, issue time unknown", rotateAnswerJSON(right()...), math.MaxInt64, ""},
		{"past the arrow and back", rotateAnswerJSON(turnBy(1, 1, 1, 1, -1), turnBy(-1, -1)), late, ""},
		{"the long way round", rotateAnswerJSON(turnBy(presses(5, 0)...), turnBy(presses(6, 6)...)), late, ""},
		{"more than a full turn", rotateAnswerJSON(turnBy(presses(11, 11)...), turnBy(-1, -1)), late, ""},
		{"24 presses", rotateAnswerJSON(turnBy(1, 1, 1), turnBy(presses(24, 15)...)), late, ""},
		{"25 presses", rotateAnswerJSON(turnBy(presses(25, 14)...), turnBy(-1, -1)), late, "press count"},
		{"no press", with(0, func(r *rotateRoundAnswerJSON) { r.S, r.F = [][]float64{}, 0 }), late, "press count"},
		{"one step short", rotateAnswerJSON(turnBy(1, 1), turnBy(-1, -1)), late, "wrong frame"},
		{"one step past", rotateAnswerJSON(turnBy(1, 1, 1), turnBy(-1)), late, "wrong frame"},
		{"back where it started", rotateAnswerJSON(turnBy(1, -1), turnBy(-1, -1)), late, "wrong frame"},
		{"frame not reached by the presses", with(1, func(r *rotateRoundAnswerJSON) { r.F = 5 }), late, "presses do not reach the frame"},
		{"frame 8", with(0, func(r *rotateRoundAnswerJSON) { r.F = 8 }), late, "frame outside the strip"},
		{"frame -1", with(1, func(r *rotateRoundAnswerJSON) { r.F = -1 }), late, "frame outside the strip"},
		{"direction 2", with(0, press(1, 900, 2)), late, "direction"},
		{"direction 0", with(1, press(0, 600, 0)), late, "direction"},
		{"first press at 249 ms", with(0, press(0, 249, 1)), late, "first press too soon"},
		{"first press at 250 ms", with(0, press(0, 250, 1)), late, ""},
		{"presses 59 ms apart", with(0, press(1, 659, 1)), late, "presses too close"},
		{"presses 60 ms apart", with(0, press(1, 660, 1)), late, ""},
		{"presses out of order", with(1, press(1, 500, -1)), late, "presses out of order"},
		{"two presses at once", with(1, press(1, 600, -1)), late, "presses out of order"},
		{"Done with the last press", with(0, func(r *rotateRoundAnswerJSON) { r.Done = 1200 }), late, "done before the last press"},
		{"Done after 60 s", with(1, func(r *rotateRoundAnswerJSON) { r.Done = 60001 }), late, "round too long"},
		{"Done at 60 s", with(1, func(r *rotateRoundAnswerJSON) { r.Done = 60000 }), 70 * time.Second, ""},
		{"half a millisecond", with(0, press(2, 1200.5, 1)), late, "not an integer"},
		{"half a frame", with(1, func(r *rotateRoundAnswerJSON) { r.F = 5.5 }), late, "not an integer"},
		{"Done half way through a millisecond", with(0, func(r *rotateRoundAnswerJSON) { r.Done = 1600.25 }), late, "not an integer"},
		{"a press of one number", with(0, func(r *rotateRoundAnswerJSON) { r.S[0] = []float64{600} }), late, "press is not [ms, d]"},
		{"a press of three numbers", with(1, func(r *rotateRoundAnswerJSON) { r.S[1] = []float64{900, -1, 0} }), late, "press is not [ms, d]"},
		{"one round", rotateAnswerJSON(right()[0]), late, "round count"},
		{"three rounds", rotateAnswerJSON(append(right(), right()[1])...), late, "round count"},
		{"f missing", `{"r":[{"s":[[600,1],[900,1],[1200,1]],"done":1600},{"f":6,"s":[[600,-1],[900,-1]],"done":1300}]}`, late, "missing number"},
		{"done missing", `{"r":[{"f":3,"s":[[600,1],[900,1],[1200,1]],"done":1600},{"f":6,"s":[[600,-1],[900,-1]]}]}`, late, "missing number"},
		{"answer over 2 KiB", rotateAnswerJSON(right()...) + strings.Repeat(" ", 2<<10), late, "answer too long"},
		{"not JSON", `{"r": [`, late, "not the answer JSON"},
		{"text answer", "abcde", late, "not the answer JSON"},
		{"presses not a list", `{"r":[{"f":3,"s":"rrr","done":1600},{"f":6,"s":[[600,-1],[900,-1]],"done":1300}]}`, late, "not the answer JSON"},
		{"too soon after issue", rotateAnswerJSON(right()...), soon, "solved too soon after issue"},
		{"just late enough after issue", rotateAnswerJSON(right()...), soon + time.Millisecond, ""},
	}
	for _, test := range tests {
		verdict := checkRotate(test.answer, targets, test.sinceIssue)
		if verdict.pass != (test.reason == "") || verdict.reason != test.reason {
			t.Errorf("%s: got pass %v %q, want %q", test.name, verdict.pass, verdict.reason, test.reason)
		}
	}
	stored, _ := json.Marshal(rotateAnswer{R: targets})
	if !checkObjectRotate(rotateAnswerJSON(right()...), string(stored), late) {
		t.Error("checkObjectRotate refused a right answer")
	}
	for _, broken := range []string{"", "12", `{"r":[]}`, `{"r":[{"t":3}]}`, string(stored[:len(stored)-2])} {
		if checkObjectRotate(rotateAnswerJSON(right()...), broken, late) {
			t.Errorf("stored answer %q accepted", broken)
		}
	}
}

// TestRotateRandomPassRate presses the buttons at random, 1 to 24 times a round with a person's
// pace, and presses Done: the frame it ends on is right 1 time in 8, both rounds 1 time in 64.
func TestRotateRandomPassRate(t *testing.T) {
	const answers = 100_000
	random := mrand.New(mrand.NewPCG(5, 6))
	passed := 0
	for range answers {
		targets := []rotateTarget{{Target: 1 + random.IntN(rotateFrames-1)}, {Target: 1 + random.IntN(rotateFrames-1)}}
		var rounds []rotateRoundAnswerJSON
		for range targets {
			round := rotateRoundAnswerJSON{}
			ms, sum := float64(rotateMinFirstPressMs+random.IntN(1500)), 0.0
			for j := range 1 + random.IntN(rotateMaxPresses) {
				if j > 0 {
					ms += float64(rotateMinPressGapMs + random.IntN(700))
				}
				direction := float64(1 - 2*random.IntN(2))
				sum += direction
				round.S = append(round.S, []float64{ms, direction})
			}
			round.F = math.Mod(math.Mod(sum, rotateFrames)+rotateFrames, rotateFrames)
			round.Done = ms + float64(100+random.IntN(1000))
			rounds = append(rounds, round)
		}
		if checkRotate(rotateAnswerJSON(rounds...), targets, math.MaxInt64).pass {
			passed++
		}
	}
	rate := float64(passed) / answers
	t.Logf("random presses pass %d of %d: 1 in %.1f", passed, answers, 1/rate)
	if rate < 1.0/80 || rate > 1.0/50 {
		t.Fatalf("pass rate 1 in %.1f, want between 1 in 80 and 1 in 50", 1/rate)
	}
}
