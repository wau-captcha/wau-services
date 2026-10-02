package biz

import (
	"encoding/json"
	"fmt"
	"math"
	mrand "math/rand/v2"
	"strings"
	"testing"
	"time"
)

const testTarget = 150

// humanTrail is a drag the way a hand makes one: it speeds up and brakes (a minimum-jerk curve),
// overshoots the hole by 3 px and comes back over the last fifth, drifts 1-3 px up and down, and
// samples every 8-20 ms. It lasts about duration and ends on endX.
func humanTrail(seed uint64, endX float64, duration float64) [][3]float64 {
	random := mrand.New(mrand.NewPCG(seed, 7))
	const overshoot = 3.0
	at := func(t float64) float64 {
		share := min(t/duration, 1)
		if share < 0.8 {
			s := share / 0.8
			return (endX + overshoot) * (10*s*s*s - 15*s*s*s*s + 6*s*s*s*s*s)
		}
		s := (share - 0.8) / 0.2
		return endX + overshoot*(1-(3*s*s-2*s*s*s))
	}
	trail := [][3]float64{{0, 0, 0}}
	for t := 8 + random.Float64()*12; t < duration; t += 8 + random.Float64()*12 {
		wobble := 2*math.Sin(2*math.Pi*t/700) + (random.Float64()*2-1)*0.6
		trail = append(trail, [3]float64{round1(t), round1(at(t)), round1(wobble)})
	}
	return append(trail, [3]float64{round1(duration + 10), endX, trail[len(trail)-1][2]})
}

// scriptTrail moves at constant speed in even 16 ms steps with y always 0.
func scriptTrail(endX float64, steps int) [][3]float64 {
	trail := make([][3]float64, steps+1)
	for i := range trail {
		trail[i] = [3]float64{float64(16 * i), round1(endX * float64(i) / float64(steps)), 0}
	}
	return trail
}

// browserScript is a constant-speed script seen through a real browser: events every 8-20 ms,
// each stamped up to 4 ms after its position was set, y always 0, and a hold of `hold` ms before
// the release. Counted in, the hold alone used to give it 3 signals.
func browserScript(seed uint64, endX, duration, hold float64) [][3]float64 {
	random := mrand.New(mrand.NewPCG(seed, 11))
	trail := [][3]float64{{0, 0, 0}}
	// Like the SDK, which keeps 8 ms between points, the last move comes 8 ms or more before duration.
	for t := 8 + random.Float64()*12; t < duration-8; t += 8 + random.Float64()*12 {
		trail = append(trail, [3]float64{round1(t), round1(endX * (t - random.Float64()*4) / duration), 0})
	}
	trail = append(trail, [3]float64{round1(duration), endX, 0})
	return append(trail, [3]float64{round1(duration + hold), endX, 0})
}

// hold adds a pause of `ms` before the release, the piece resting where it ended.
func hold(trail [][3]float64, ms float64) [][3]float64 {
	last := trail[len(trail)-1]
	return append(append([][3]float64(nil), trail...), [3]float64{last[0] + ms, last[1], last[2]})
}

func round1(value float64) float64 { return math.Round(value*10) / 10 }

// encode writes a drag the way the SDK does: {"x": <last x>, "t": [[ms, x, y], ...]}.
func encode(trail [][3]float64, x float64) string {
	encoded, err := json.Marshal(map[string]any{"x": x, "t": trail})
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func lastX(trail [][3]float64) float64 { return trail[len(trail)-1][1] }

func TestCheckSlide(t *testing.T) {
	human := humanTrail(1, testTarget, 900)
	backwards := humanTrail(2, testTarget, 900)
	backwards[5][0], backwards[6][0] = backwards[6][0], backwards[5][0]
	script := scriptTrail(testTarget, 40)
	browser := browserScript(1, testTarget, 400, 45)
	quick := humanTrail(3, testTarget, 90)
	tooMany := humanTrail(4, testTarget, 9000)
	for len(tooMany) <= slideMaxPoints {
		tooMany = append(tooMany, [3]float64{tooMany[len(tooMany)-1][0] + 10, testTarget, 0})
	}
	tests := []struct {
		name       string
		answer     string
		sinceIssue time.Duration
		pass       bool
		reason     string
	}{
		{"human-like trail", encode(human, lastX(human)), 2 * time.Second, true, ""},
		{"human-like trail, 4 px off the hole", encode(shiftEnd(human, 4), testTarget+4), 2 * time.Second, true, ""},
		{"wrong position by 6 px", encode(shiftEnd(human, 6), testTarget+6), 2 * time.Second, false, "wrong position"},
		{"linear constant-speed trail, y 0, even 16 ms steps", encode(script, lastX(script)), 2 * time.Second, false, ""},
		{"constant speed through a browser, held 45 ms before release", encode(browser, lastX(browser)), 2 * time.Second, false, ""},
		{"human-like trail held 600 ms before release", encode(hold(human, 600), lastX(human)), 2 * time.Second, true, ""},
		{"3 points", encode(human[:3:3], lastX(human[:3])), 2 * time.Second, false, "point count"},
		{"more than 400 points", encode(tooMany, lastX(tooMany)), 30 * time.Second, false, "point count"},
		{"drag of 100 ms", encode(quick, lastX(quick)), 2 * time.Second, false, "drag duration"},
		{"checked 200 ms after issue", encode(human, lastX(human)), 200 * time.Millisecond, false, "solved too soon after issue"},
		{"broken JSON", encode(human, lastX(human))[:40], 2 * time.Second, false, "not the answer JSON"},
		{"no x", `{"t":[[0,0,0]]}`, 2 * time.Second, false, "not the answer JSON"},
		{"last x differs from x", encode(human, testTarget+3), 2 * time.Second, false, "trail does not end at x"},
		{"times going backwards", encode(backwards, lastX(backwards)), 2 * time.Second, false, "times not increasing"},
		{"NaN", strings.Replace(encode(human, lastX(human)), `"x":150`, `"x":NaN`, 1), 2 * time.Second, false, "not the answer JSON"},
		{"point of two numbers", strings.Replace(encode(human, lastX(human)), "[0,0,0]", "[0,0]", 1), 2 * time.Second, false, "malformed point"},
		{"does not start at x = 0", encode(startAt(human, 5), lastX(human)), 2 * time.Second, false, "trail does not start at x = 0"},
		{"x past the image", encode(append(human[:len(human)-1:len(human)-1], [3]float64{2000, 301, 0}), 301), 2 * time.Second, false, "x outside the image"},
		{"answer over 16 KiB", strings.Repeat(" ", slideMaxAnswerBytes+1), 2 * time.Second, false, "answer too long"},
		{"typed text", "ABCDE", 2 * time.Second, false, "not the answer JSON"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verdict := checkSlide(test.answer, testTarget, test.sinceIssue)
			if verdict.pass != test.pass || verdict.reason != test.reason {
				t.Fatalf("got pass=%v reason=%q signals=%d, want pass=%v reason=%q",
					verdict.pass, verdict.reason, verdict.signals, test.pass, test.reason)
			}
		})
	}
}

// TestCheckSlideSignals shows which behaviour signals each trail gives, so a change of threshold
// that flips one of them is noticed.
func TestCheckSlideSignals(t *testing.T) {
	for seed := range uint64(200) {
		trail := humanTrail(seed, testTarget, 500+float64(seed%10)*150)
		if verdict := checkSlide(encode(trail, lastX(trail)), testTarget, 2*time.Second); verdict.signals != 5 {
			t.Fatalf("human trail %d: %d of 5 signals, want all", seed, verdict.signals)
		}
	}
	for _, steps := range []int{20, 40, 120} {
		trail := scriptTrail(testTarget, steps)
		if verdict := checkSlide(encode(trail, lastX(trail)), testTarget, 2*time.Second); verdict.signals != 0 {
			t.Fatalf("script trail of %d steps: %d signals, want none", steps, verdict.signals)
		}
	}
	// Through a browser the sampling is uneven, which is the one signal a constant-speed script gets.
	for seed := range uint64(200) {
		trail := browserScript(seed, testTarget, 300+float64(seed%10)*150, 20+float64(seed%5)*40)
		if verdict := checkSlide(encode(trail, lastX(trail)), testTarget, 2*time.Second); verdict.signals != 1 {
			t.Fatalf("browser script %d: %d signals, want only uneven sampling", seed, verdict.signals)
		}
	}
}

func TestCheckSlidePuzzleTarget(t *testing.T) {
	trail := humanTrail(5, testTarget, 900)
	answer := encode(trail, lastX(trail))
	for _, test := range []struct {
		expected string
		pass     bool
	}{{fmt.Sprint(testTarget), true}, {fmt.Sprint(testTarget + 10), false}, {"", false}, {"abc", false}} {
		if got := checkSlidePuzzle(answer, test.expected, 2*time.Second); got != test.pass {
			t.Errorf("stored answer %q: got %v, want %v", test.expected, got, test.pass)
		}
	}
}

func shiftEnd(trail [][3]float64, by float64) [][3]float64 {
	shifted := append([][3]float64(nil), trail...)
	shifted[len(shifted)-1][1] += by
	return shifted
}

func startAt(trail [][3]float64, x float64) [][3]float64 {
	moved := append([][3]float64(nil), trail...)
	moved[0][1] = x
	return moved
}
