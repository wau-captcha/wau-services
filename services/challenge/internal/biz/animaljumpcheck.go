package biz

import (
	"encoding/json"
	"math"
	"time"
)

// Every threshold of the animal_jump check. Each round's answer lives only in the pixels of its
// animation, so the check holds a click to where and when a person could have made it.
const (
	jumpMaxAnswerBytes = 1 << 10 // two clicks of four numbers fit in under 150 bytes
	jumpMaxReplays     = 3       // the SDKs disable the replay button after three
	jumpMaxClickMs     = 30000   // a playback lasts 2 s; much later and nobody was watching
	// A click this soon after the high jump's apex came before a person could see it peak and react:
	// a guess, or a script reading the frames.
	jumpMinReactionMs = 150
	// From issue to check both rounds must have played at least to their apex, with loading and the
	// pause between the rounds on top.
	jumpMinSolveSlack = 600 * time.Millisecond
)

// jumpVerdict is the outcome of checkJump; reason names the first rule broken, for tests.
type jumpVerdict struct {
	pass   bool
	reason string
}

type jumpClick struct {
	X  *float64 `json:"x"`
	Y  *float64 `json:"y"`
	Ms *float64 `json:"ms"`
	P  *float64 `json:"p"`
}

// checkAnimalJump checks an animal_jump answer against the targets stored with the challenge.
func checkAnimalJump(answer, expected string, sinceIssue time.Duration) bool {
	var stored jumpAnswer
	if json.Unmarshal([]byte(expected), &stored) != nil || len(stored.R) != jumpRounds {
		return false
	}
	return checkJump(answer, stored.R, sinceIssue).pass
}

// checkJump decides from the answer, {"c": [{"x", "y", "ms", "p"}, ...]} with one click per round in
// order, whether each click hit the round's target after its high jump could be seen.
func checkJump(answer string, targets []jumpTarget, sinceIssue time.Duration) jumpVerdict {
	fail := func(reason string) jumpVerdict { return jumpVerdict{reason: reason} }
	if len(answer) > jumpMaxAnswerBytes {
		return fail("answer too long")
	}
	var parsed struct {
		C []jumpClick `json:"c"`
	}
	if json.Unmarshal([]byte(answer), &parsed) != nil {
		return fail("not the answer JSON")
	}
	if len(parsed.C) != jumpRounds || len(targets) != jumpRounds {
		return fail("click count")
	}
	for _, click := range parsed.C {
		if click.X == nil || click.Y == nil || click.Ms == nil || click.P == nil {
			return fail("missing number")
		}
		x, y, ms, replays := *click.X, *click.Y, *click.Ms, *click.P
		switch {
		case !finite(x) || !finite(y) || !finite(ms) || !finite(replays):
			return fail("number not finite")
		case x < 0 || x > jumpWidth:
			return fail("x outside the image")
		case y < 0 || y > jumpHeight:
			return fail("y outside the image")
		case replays < 0 || replays > jumpMaxReplays || replays != math.Trunc(replays):
			return fail("replays")
		case ms < 0 || ms > jumpMaxClickMs:
			return fail("click time")
		}
	}
	for i, click := range parsed.C {
		target := targets[i]
		if *click.X < target.X0 || *click.X > target.X1 || *click.Y < target.Y0 || *click.Y > target.Y1 {
			return fail("missed the target")
		}
		if *click.Ms < target.Apex+jumpMinReactionMs {
			return fail("clicked before the high jump was seen")
		}
	}
	// Written this way round so that the unknown issue time (math.MaxInt64) cannot overflow.
	if sinceIssue-jumpMinSolveSlack < time.Duration(targets[0].Apex+targets[1].Apex)*time.Millisecond {
		return fail("solved too soon after issue")
	}
	return jumpVerdict{pass: true}
}
