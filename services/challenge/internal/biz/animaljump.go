package biz

import (
	"encoding/json"
	"fmt"
	"math"
	mrand "math/rand/v2"
	"slices"
)

// One animal_jump round: five animal heads in a row jump at different speeds and moments; exactly
// one makes a jump clearly higher than every other. Sizes are CSS pixels of the 300 × 150 image,
// times milliseconds from the round's first frame.
const (
	jumpWidth, jumpHeight = 300, 150
	jumpRounds            = 2 // a random click is right 1 time in 5; two in a row, 1 in 25
	jumpHeads             = 5
	jumpSlot              = 60 // width of each head's slot
	jumpJitter            = 3  // most a head moves sideways from its slot's centre
	jumpGround            = 138
	jumpMinDiameter       = 40
	jumpMaxDiameter       = 48
	// Jumps: time in the air is drawn apart from height, so speed says nothing about which is high.
	jumpMinStart    = 150
	jumpMaxEnd      = 1900
	jumpMinDuration = 380
	jumpMaxDuration = 720
	jumpMinGap      = 100 // between two jumps of one head
	jumpLowMin      = 12
	jumpLowMax      = 26
	// At least 20 px above any other jump: plain to the eye.
	jumpHighMin = 46
	jumpHighMax = 58
	// The high jump peaks in this window, so a person has seen it 1.6 s into the round at the latest.
	jumpApexMin = 500
	jumpApexMax = 1600
	// A click anywhere on the target, at rest or in the air, counts: its box widened by this much.
	jumpHitMargin = 4
)

// jump is one hop: it leaves the ground at start, lands duration ms later and peaks height px up.
type jump struct{ start, duration, height float64 }

func (j jump) apex() float64 { return j.start + j.duration/2 }
func (j jump) end() float64  { return j.start + j.duration }

// heightAt follows the parabola h(t) = H · 4u(1 − u), u the share of the jump done; 0 on the ground.
func (j jump) heightAt(t float64) float64 {
	if t <= j.start || t >= j.end() {
		return 0
	}
	u := (t - j.start) / j.duration
	return j.height * 4 * u * (1 - u)
}

type jumpHead struct {
	species int
	tint    [3]float64
	radius  float64
	// The face's centre at rest, and the head's box at rest: its bottom is on the ground.
	centreX, centreY float64
	left, top, right float64
	jumps            []jump
	highest          float64
}

// hitBox is where a click counts as one on this head: the head at rest, widened by jumpHitMargin,
// and stretched upward by its highest jump plus the margin, so a click in mid-air counts too.
func (h jumpHead) hitBox() (x0, y0, x1, y1 float64) {
	return h.left - jumpHitMargin, max(h.top-h.highest-jumpHitMargin, 0),
		h.right + jumpHitMargin, min(jumpGround+jumpHitMargin, jumpHeight)
}

type jumpRound struct {
	heads  [jumpHeads]jumpHead
	target int
	high   jump
}

// jumpTarget is what the service stores of one round: the target's hit box and when its high jump
// peaks. It is never sent to the client.
type jumpTarget struct {
	X0   float64 `json:"x0"`
	X1   float64 `json:"x1"`
	Y0   float64 `json:"y0"`
	Y1   float64 `json:"y1"`
	Apex float64 `json:"apex"`
}

func (r jumpRound) answer() jumpTarget {
	x0, y0, x1, y1 := r.heads[r.target].hitBox()
	return jumpTarget{X0: round1(x0), X1: round1(x1), Y0: round1(y0), Y1: round1(y1), Apex: math.Round(r.high.apex())}
}

func round1(value float64) float64 { return math.Round(value*10) / 10 }

// planRound draws a round's species, places and jumps; renderRound turns it into a GIF.
func planRound(random *mrand.Rand) jumpRound {
	var round jumpRound
	species := random.Perm(len(allSpecies))
	for {
		for i := range round.heads {
			diameter := jumpMinDiameter + random.Float64()*(jumpMaxDiameter-jumpMinDiameter)
			jitter := (random.Float64()*2 - 1) * jumpJitter
			round.heads[i] = placeHead(species[i], diameter/2, float64(i*jumpSlot+jumpSlot/2)+jitter)
		}
		// Two large heads pushed towards each other can bring their boxes within reach: draw again.
		if apart(round.heads[:]) {
			break
		}
	}
	round.target = random.IntN(jumpHeads)
	for i := range round.heads {
		head := &round.heads[i]
		for c := range head.tint {
			head.tint[c] = 1 + (random.Float64()*2-1)*0.06
		}
		if i == round.target {
			round.high = highJump(random)
			head.jumps = append(head.jumps, round.high)
			if random.IntN(2) == 0 {
				head.jumps = append(head.jumps, lowJumpAround(random, round.high)...)
			}
		} else {
			head.jumps = lowJumps(random, 1+random.IntN(2))
		}
		for _, j := range head.jumps {
			head.highest = max(head.highest, j.height)
		}
	}
	return round
}

// placeHead stands a head of the species on the ground with its face's centre at x.
func placeHead(kind int, radius, x float64) jumpHead {
	extent := speciesExtent[kind]
	// The outline goes round the outer parts, so the bottom of its outer edge touches the ground.
	centreY := jumpGround - extent[3]*radius - headOutlinePx
	return jumpHead{
		species: kind, radius: radius, centreX: x, centreY: centreY,
		top: centreY + extent[1]*radius - headOutlinePx, left: x + extent[0]*radius, right: x + extent[2]*radius,
	}
}

// apart reports whether neighbouring heads' boxes keep clear of each other. They are as tall as the
// jumps allow, which can only overlap above other heads, so the horizontal extents decide.
func apart(heads []jumpHead) bool {
	for i := range len(heads) - 1 {
		if heads[i].right+jumpHitMargin >= heads[i+1].left-jumpHitMargin {
			return false
		}
	}
	return true
}

func between(random *mrand.Rand, low, high float64) float64 { return low + random.Float64()*(high-low) }

// highJump is the target's high jump, peaking inside the apex window.
func highJump(random *mrand.Rand) jump {
	duration := between(random, jumpMinDuration, jumpMaxDuration)
	apex := between(random, max(jumpApexMin, jumpMinStart+duration/2), min(jumpApexMax, jumpMaxEnd-duration/2))
	return jump{start: apex - duration/2, duration: duration, height: between(random, jumpHighMin, jumpHighMax)}
}

// lowJumps spreads count normal jumps over the round, jumpMinGap apart.
func lowJumps(random *mrand.Rand, count int) []jump {
	durations := make([]float64, count)
	slack := float64(jumpMaxEnd - jumpMinStart - jumpMinGap*(count-1))
	for i := range durations {
		durations[i] = between(random, jumpMinDuration, jumpMaxDuration)
		slack -= durations[i]
	}
	// Cut the spare time at count random points: before, between and after the jumps.
	cuts := make([]float64, count)
	for i := range cuts {
		cuts[i] = random.Float64() * slack
	}
	slices.Sort(cuts)
	jumps := make([]jump, count)
	start, used := float64(jumpMinStart), 0.0
	for i := range jumps {
		start += cuts[i] - used
		used = cuts[i]
		jumps[i] = jump{start: start, duration: durations[i], height: between(random, jumpLowMin, jumpLowMax)}
		start += durations[i] + jumpMinGap
	}
	return jumps
}

// lowJumpAround fits one normal jump before or after the high one, or none when neither side has room.
func lowJumpAround(random *mrand.Rand, high jump) []jump {
	duration := between(random, jumpMinDuration, jumpMaxDuration)
	var starts [][2]float64
	if last := high.start - jumpMinGap - duration; last >= jumpMinStart {
		starts = append(starts, [2]float64{jumpMinStart, last})
	}
	if first := high.end() + jumpMinGap; first+duration <= jumpMaxEnd {
		starts = append(starts, [2]float64{first, jumpMaxEnd - duration})
	}
	if len(starts) == 0 {
		return nil
	}
	window := starts[random.IntN(len(starts))]
	return []jump{{start: between(random, window[0], window[1]), duration: duration, height: between(random, jumpLowMin, jumpLowMax)}}
}

type jumpAnswer struct {
	R []jumpTarget `json:"r"`
}

// generateAnimalJump draws both rounds; the stored answer holds each round's target box and apex.
func generateAnimalJump() (string, Media, error) {
	random := newRandomSource()
	var answer jumpAnswer
	var gifs [jumpRounds][]byte
	for i := range gifs {
		round := planRound(random)
		data, err := renderRound(random, round)
		if err != nil {
			return "", Media{}, fmt.Errorf("animal_jump round %d: %w", i+1, err)
		}
		gifs[i] = data
		answer.R = append(answer.R, round.answer())
	}
	stored, err := json.Marshal(answer)
	if err != nil {
		return "", Media{}, err
	}
	return string(stored), Media{Data: gifs[0], SecondRound: gifs[1], MIMEType: "image/gif", Width: jumpWidth, Height: jumpHeight}, nil
}
