package biz

import (
	"bytes"
	"encoding/json"
	"image"
	"image/draw"
	"image/gif"
	"math"
	"testing"
)

// checkRound verifies the rules of a planned round; it returns a reason, or "" when the round holds.
func checkRound(round jumpRound) string {
	seen := map[int]bool{}
	highJumps := 0
	for i, head := range round.heads {
		if seen[head.species] {
			return "species repeated"
		}
		seen[head.species] = true
		if diameter := 2 * head.radius; diameter < jumpMinDiameter || diameter > jumpMaxDiameter {
			return "diameter"
		}
		if slot := float64(i*jumpSlot + jumpSlot/2); math.Abs(head.centreX-slot) > jumpJitter {
			return "jitter"
		}
		if bottom := head.centreY + speciesExtent[head.species][3]*head.radius + headOutlinePx; math.Abs(bottom-jumpGround) > 1e-9 {
			return "not on the ground"
		}
		if len(head.jumps) < 1 || len(head.jumps) > 2 {
			return "jump count"
		}
		for j, hop := range head.jumps {
			switch {
			case hop.start < jumpMinStart || hop.end() > jumpMaxEnd:
				return "jump outside 150-1900 ms"
			case hop.duration < jumpMinDuration || hop.duration > jumpMaxDuration:
				return "duration"
			case hop.height >= jumpHighMin && hop.height <= jumpHighMax:
				highJumps++
				if i != round.target || hop != round.high {
					return "high jump by another head"
				}
			case hop.height < jumpLowMin || hop.height > jumpLowMax:
				return "height"
			}
			for _, other := range head.jumps[j+1:] {
				first, second := hop, other
				if second.start < first.start {
					first, second = second, first
				}
				if second.start < first.end()+jumpMinGap {
					return "jumps overlap"
				}
			}
		}
		if head.top-head.highest < 0 {
			return "leaves the image"
		}
	}
	if highJumps != 1 {
		return "not exactly one high jump"
	}
	if apex := round.high.apex(); apex < jumpApexMin || apex > jumpApexMax {
		return "apex outside 500-1600 ms"
	}
	// Every other jump stays 20 px under the high one.
	for i, head := range round.heads {
		for _, hop := range head.jumps {
			if (i != round.target || hop != round.high) && round.high.height-hop.height < jumpHighMin-jumpLowMax {
				return "height gap"
			}
		}
	}
	for i := range len(round.heads) - 1 {
		_, _, right, _ := round.heads[i].hitBox()
		left, _, _, _ := round.heads[i+1].hitBox()
		if right >= left {
			return "hit boxes overlap"
		}
	}
	return ""
}

func TestPlanRoundRules(t *testing.T) {
	random := newRandomSource()
	worst := 0.0
	for range 1000 {
		var apexes float64
		for range jumpRounds {
			round := planRound(random)
			if reason := checkRound(round); reason != "" {
				t.Fatalf("%s: %+v", reason, round)
			}
			apexes += round.high.apex()
		}
		// Each round: the high jump peaks, a person reacts within a second and clicks; the ring
		// shows 250 ms and the next round loads. Both rounds must fit in 10 s with room to spare.
		total := apexes + jumpRounds*(1000+250+300)
		worst = max(worst, total)
	}
	if worst > 7000 {
		t.Fatalf("worst two-round time %.0f ms, want at most 7 s of the 10 s budget", worst)
	}
	t.Logf("worst two-round time with a 1 s reaction: %.0f ms", worst)
}

func TestSpeciesFitTheirWidth(t *testing.T) {
	if len(allSpecies) < 8 {
		t.Fatalf("%d species, want at least 8", len(allSpecies))
	}
	for i, kind := range allSpecies {
		extent := speciesExtent[i]
		if extent[0] < -1 || extent[2] > 1 {
			t.Errorf("%s is wider than its face: %v", kind.name, extent)
		}
		// The tallest head at the largest size, at the top of the highest jump, stays in the image.
		top := jumpGround - (extent[3]-extent[1])*jumpMaxDiameter/2 - 2*headOutlinePx - jumpHighMax
		if top < 0 {
			t.Errorf("%s leaves the image at the top of a high jump: %.1f", kind.name, top)
		}
	}
}

// frames decodes a GIF and composites its frames as a browser does (disposal none).
func frames(t *testing.T, data []byte) (*gif.GIF, []*image.RGBA) {
	t.Helper()
	animation, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	canvas := image.NewRGBA(image.Rect(0, 0, animation.Config.Width, animation.Config.Height))
	var composed []*image.RGBA
	for _, frame := range animation.Image {
		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
		composed = append(composed, image.NewRGBA(canvas.Bounds()))
		copy(composed[len(composed)-1].Pix, canvas.Pix)
	}
	return animation, composed
}

func TestAnimalJumpGIF(t *testing.T) {
	random := newRandomSource()
	for range 6 {
		round := planRound(random)
		data, err := renderRound(random, round)
		if err != nil {
			t.Fatal(err)
		}
		animation, composed := frames(t, data)
		switch {
		case len(animation.Image) != jumpFrames:
			t.Fatalf("%d frames, want 50", len(animation.Image))
		case animation.LoopCount != -1:
			t.Fatalf("LoopCount %d, want -1: played once", animation.LoopCount)
		case animation.Config.Width != jumpWidth*jumpScale || animation.Config.Height != jumpHeight*jumpScale:
			t.Fatalf("size %dx%d", animation.Config.Width, animation.Config.Height)
		case len(animation.Image[0].Palette) > jumpPaletteSize:
			t.Fatalf("%d colours", len(animation.Image[0].Palette))
		}
		for _, delay := range animation.Delay {
			if delay != jumpFrameDelay {
				t.Fatalf("delay %d, want 4 cs", delay)
			}
		}
		// The last frame, which stays on screen, shows every head where it stood in the first.
		first, last := composed[0], composed[len(composed)-1]
		for _, head := range round.heads {
			inner := image.Rect(int(head.centreX-head.radius/2), int(head.centreY-head.radius/2), int(head.centreX+head.radius/2), int(head.centreY+head.radius/2))
			for y := inner.Min.Y * jumpScale; y < inner.Max.Y*jumpScale; y++ {
				for x := inner.Min.X * jumpScale; x < inner.Max.X*jumpScale; x++ {
					if first.RGBAAt(x, y) != last.RGBAAt(x, y) {
						t.Fatalf("%s not at rest in the last frame at (%d, %d)", allSpecies[head.species].name, x, y)
					}
				}
			}
		}
	}
}

func TestAnimalJumpSize(t *testing.T) {
	const challenges = 12
	var total, worst int
	for range challenges {
		answer, media, err := generateAnimalJump()
		if err != nil {
			t.Fatal(err)
		}
		size := len(media.Data) + len(media.SecondRound)
		total, worst = total+size, max(worst, size)
		var stored jumpAnswer
		if err := json.Unmarshal([]byte(answer), &stored); err != nil || len(stored.R) != jumpRounds {
			t.Fatalf("stored answer %q: %v", answer, err)
		}
		if media.Width != jumpWidth || media.Height != jumpHeight || media.MIMEType != "image/gif" || media.Piece != nil {
			t.Fatalf("media %+v", media)
		}
	}
	t.Logf("both rounds: %d KB on average, %d KB at most", total/challenges/1024, worst/1024)
	if worst >= 400<<10 {
		t.Fatalf("both rounds take %d KB, want under 400 KB", worst/1024)
	}
}
