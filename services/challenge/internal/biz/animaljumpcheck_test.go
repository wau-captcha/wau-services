package biz

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

// clickOn answers a round by clicking the head at index, at its face's centre, ms into the playback.
func clickOn(round jumpRound, index int, ms float64) map[string]float64 {
	head := round.heads[index]
	return map[string]float64{"x": head.centreX, "y": head.centreY, "ms": ms, "p": 0}
}

func jumpAnswerJSON(clicks ...map[string]float64) string {
	data, _ := json.Marshal(map[string]any{"c": clicks})
	return string(data)
}

// otherHead is any head of the round but the target.
func otherHead(round jumpRound) int { return (round.target + 1) % jumpHeads }

func TestCheckJump(t *testing.T) {
	random := newRandomSource()
	first, second := planRound(random), planRound(random)
	targets := []jumpTarget{first.answer(), second.answer()}
	late := 10 * time.Second
	right := func() []map[string]float64 {
		return []map[string]float64{
			clickOn(first, first.target, targets[0].Apex+400),
			clickOn(second, second.target, targets[1].Apex+400),
		}
	}
	with := func(round int, key string, value float64) string {
		clicks := right()
		clicks[round][key] = value
		return jumpAnswerJSON(clicks...)
	}
	soon := time.Duration(targets[0].Apex+targets[1].Apex)*time.Millisecond + 599*time.Millisecond
	tests := []struct {
		name       string
		answer     string
		sinceIssue time.Duration
		reason     string
	}{
		{"right", jumpAnswerJSON(right()...), late, ""},
		{"right, issue time unknown", jumpAnswerJSON(right()...), math.MaxInt64, ""},
		{"right, with replays", with(1, "p", 3), late, ""},
		{"right, on the target's box edge", with(0, "x", targets[0].X1), late, ""},
		{"answer over 1 KiB", jumpAnswerJSON(right()...) + strings.Repeat(" ", 1<<10), late, "answer too long"},
		{"not JSON", "{\"c\": [", late, "not the answer JSON"},
		{"text answer", "abcde", late, "not the answer JSON"},
		{"one click", jumpAnswerJSON(right()[0]), late, "click count"},
		{"three clicks", jumpAnswerJSON(append(right(), right()[0])...), late, "click count"},
		{"a field missing", `{"c":[{"x":1,"y":1,"ms":900},{"x":1,"y":1,"ms":900,"p":0}]}`, late, "missing number"},
		{"x below 0", with(0, "x", -1), late, "x outside the image"},
		{"x past 300", with(1, "x", 300.5), late, "x outside the image"},
		{"y below 0", with(1, "y", -0.5), late, "y outside the image"},
		{"y past 150", with(0, "y", 151), late, "y outside the image"},
		{"four replays", with(0, "p", 4), late, "replays"},
		{"half a replay", with(0, "p", 1.5), late, "replays"},
		{"negative replays", with(1, "p", -1), late, "replays"},
		{"negative time", with(0, "ms", -1), late, "click time"},
		{"time past 30 s", with(1, "ms", 30001), late, "click time"},
		{"another head in round 1", jumpAnswerJSON(clickOn(first, otherHead(first), targets[0].Apex+400), right()[1]), late, "missed the target"},
		{"another head in round 2", jumpAnswerJSON(right()[0], clickOn(second, otherHead(second), targets[1].Apex+400)), late, "missed the target"},
		{"above the target's reach", with(0, "y", targets[0].Y0-1), late, "missed the target"},
		{"before apex + 150 ms", with(0, "ms", targets[0].Apex+149), late, "clicked before the high jump was seen"},
		{"at apex + 150 ms", with(1, "ms", targets[1].Apex+150), late, ""},
		{"too soon after issue", jumpAnswerJSON(right()...), soon, "solved too soon after issue"},
		{"just late enough after issue", jumpAnswerJSON(right()...), soon + time.Millisecond, ""},
	}
	for _, test := range tests {
		verdict := checkJump(test.answer, targets, test.sinceIssue)
		if verdict.pass != (test.reason == "") || verdict.reason != test.reason {
			t.Errorf("%s: got pass %v %q, want %q", test.name, verdict.pass, verdict.reason, test.reason)
		}
	}
	stored, _ := json.Marshal(jumpAnswer{R: targets})
	if !checkAnimalJump(jumpAnswerJSON(right()...), string(stored), late) {
		t.Error("checkAnimalJump refused a right answer")
	}
	for _, broken := range []string{"", "12", `{"r":[]}`, string(stored[:len(stored)-2])} {
		if checkAnimalJump(jumpAnswerJSON(right()...), broken, late) {
			t.Errorf("stored answer %q accepted", broken)
		}
	}
}

// TestRandomHeadPassRate clicks the centre of a random head in each round, after the high jump has
// peaked: a guess like that passes 1 time in 25.
func TestRandomHeadPassRate(t *testing.T) {
	const answers = 100_000
	random := newRandomSource()
	passed := 0
	for range answers {
		first, second := planRound(random), planRound(random)
		targets := []jumpTarget{first.answer(), second.answer()}
		answer := jumpAnswerJSON(
			clickOn(first, random.IntN(jumpHeads), targets[0].Apex+jumpMinReactionMs+random.Float64()*500),
			clickOn(second, random.IntN(jumpHeads), targets[1].Apex+jumpMinReactionMs+random.Float64()*500),
		)
		if checkJump(answer, targets, 10*time.Second).pass {
			passed++
		}
	}
	rate := float64(passed) / answers
	t.Logf("random heads pass %d of %d: 1 in %.1f", passed, answers, 1/rate)
	if rate < 1.0/30 || rate > 1.0/20 {
		t.Fatalf("pass rate 1 in %.1f, want between 1 in 30 and 1 in 20", 1/rate)
	}
}

func TestJumpHitBoxCoversTheTarget(t *testing.T) {
	random := newRandomSource()
	for range 1000 {
		round := planRound(random)
		target, head := round.answer(), round.heads[round.target]
		// The head at rest, and its top at the height of the high jump.
		for _, point := range [][2]float64{{head.left, jumpGround}, {head.right, head.top}, {head.centreX, head.top - round.high.height}} {
			if point[0] < target.X0 || point[0] > target.X1 || point[1] < target.Y0 || point[1] > target.Y1 {
				t.Fatalf("%v outside the target's box %+v", point, target)
			}
		}
		// No other head's centre is in it.
		for i, other := range round.heads {
			if i != round.target && other.centreX >= target.X0 && other.centreX <= target.X1 {
				t.Fatalf("head %d inside the target's box", i)
			}
		}
	}
}
