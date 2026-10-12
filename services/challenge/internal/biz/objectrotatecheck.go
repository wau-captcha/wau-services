package biz

import (
	"encoding/json"
	"math"
	"time"
)

// Every threshold of the object_rotate check. Each round's answer lives only in the pixels of its
// strip, so the check holds the presses to what a person turning the object could do.
const (
	rotateMaxAnswerBytes = 2 << 10 // two rounds of 24 presses fit in about 600 bytes
	rotateMaxPresses     = 24      // three full turns; the SDKs ignore presses past it
	// A first press this soon after the strip was shown came before anyone could look at the object.
	rotateMinFirstPressMs = 250
	// Faster than a finger taps a button or a key twice; the SDKs drop presses closer than this.
	rotateMinPressGapMs = 60
	// A round left this long was not being solved.
	rotateMaxDoneMs = 60000
)

// rotateVerdict is the outcome of checkRotate; reason names the first rule broken, for tests.
type rotateVerdict struct {
	pass   bool
	reason string
}

type rotateRoundAnswer struct {
	F    *float64    `json:"f"`
	S    [][]float64 `json:"s"`
	Done *float64    `json:"done"`
}

// checkObjectRotate checks an object_rotate answer against the targets stored with the challenge.
func checkObjectRotate(answer, expected string, sinceIssue time.Duration) bool {
	var stored rotateAnswer
	if json.Unmarshal([]byte(expected), &stored) != nil || len(stored.R) != rotateRounds {
		return false
	}
	return checkRotate(answer, stored.R, sinceIssue).pass
}

// checkRotate decides from the answer, {"r": [{"f", "s": [[ms, d], ...], "done"}, ...]} with one
// entry per round in order, whether each round ended on the frame facing the arrow, reached by
// presses a person could make.
func checkRotate(answer string, targets []rotateTarget, sinceIssue time.Duration) rotateVerdict {
	fail := func(reason string) rotateVerdict { return rotateVerdict{reason: reason} }
	if len(answer) > rotateMaxAnswerBytes {
		return fail("answer too long")
	}
	var parsed struct {
		R []rotateRoundAnswer `json:"r"`
	}
	if json.Unmarshal([]byte(answer), &parsed) != nil {
		return fail("not the answer JSON")
	}
	if len(parsed.R) != rotateRounds || len(targets) != rotateRounds {
		return fail("round count")
	}
	total := 0.0
	for i, round := range parsed.R {
		if round.F == nil || round.Done == nil {
			return fail("missing number")
		}
		frame, done := *round.F, *round.Done
		if !whole(frame) || !whole(done) {
			return fail("not an integer")
		}
		if len(round.S) < 1 || len(round.S) > rotateMaxPresses {
			return fail("press count")
		}
		sum, last := 0.0, 0.0
		for j, press := range round.S {
			if len(press) != 2 {
				return fail("press is not [ms, d]")
			}
			ms, direction := press[0], press[1]
			switch {
			case !whole(ms) || !whole(direction):
				return fail("not an integer")
			case direction != 1 && direction != -1:
				return fail("direction")
			case j == 0 && ms < rotateMinFirstPressMs:
				return fail("first press too soon")
			case j > 0 && ms <= last:
				return fail("presses out of order")
			case j > 0 && ms-last < rotateMinPressGapMs:
				return fail("presses too close")
			}
			sum, last = sum+direction, ms
		}
		switch {
		case done <= last:
			return fail("done before the last press")
		case done > rotateMaxDoneMs:
			return fail("round too long")
		case frame < 0 || frame >= rotateFrames:
			return fail("frame outside the strip")
		}
		// The presses must land on the frame sent: k steps right and back, modulo a full turn.
		if reached := math.Mod(math.Mod(sum, rotateFrames)+rotateFrames, rotateFrames); reached != frame {
			return fail("presses do not reach the frame")
		}
		if int(frame) != targets[i].Target {
			return fail("wrong frame")
		}
		total += done
	}
	// Both rounds were on screen at least this long. Written this way round so that the unknown
	// issue time (math.MaxInt64) cannot overflow.
	if sinceIssue < time.Duration(total)*time.Millisecond {
		return fail("solved too soon after issue")
	}
	return rotateVerdict{pass: true}
}

// whole reports whether value is an integer; JSON has no infinities, so none comes through.
func whole(value float64) bool { return value == math.Trunc(value) }
