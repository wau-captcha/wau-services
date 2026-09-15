package biz

import (
	"bytes"
	"image/gif"
	"strings"
	"testing"
)

// Any code, including the widest, must fit the frame, keep clear gaps between its letters, stay
// inside the frame while bouncing and move in every frame.
func TestMotionNoiseText(t *testing.T) {
	rng := newRand()
	codes := []string{newCode(uppercaseChars)}
	for _, c := range uppercaseChars {
		codes = append(codes, strings.Repeat(string(c), codeLength))
	}
	for _, code := range codes {
		txt, err := newText(rng, code)
		if err != nil {
			t.Fatalf("newText(%q) error = %v", code, err)
		}
		size := txt.mask.Rect.Size()
		if size.X > textMaxWidth || size.Y > videoHeight {
			t.Fatalf("%s: text size = %v, want width <= %d and height <= %d", code, size, textMaxWidth, videoHeight)
		}

		// Count runs of empty columns at least textGap wide, allowing 2 px of rasterization.
		gaps, run := 0, 0
		for x := range size.X {
			empty := true
			for y := range size.Y {
				if txt.mask.AlphaAt(x, y).A >= 0x80 {
					empty = false
					break
				}
			}
			if empty {
				run++
				continue
			}
			if run >= textGap-2 {
				gaps++
			}
			run = 0
		}
		if gaps < codeLength-1 {
			t.Fatalf("%s: %d gaps of %d px between letters, want %d", code, gaps, textGap, codeLength-1)
		}

		for frame := range videoFrames {
			p := txt.at(frame)
			if p.X < 0 || p.Y < 0 || p.X+size.X > videoWidth || p.Y+size.Y > videoHeight {
				t.Fatalf("%s: text at %v (size %v) leaves the frame in frame %d", code, p, size, frame)
			}
			if frame > 0 && p == txt.at(frame-1) {
				t.Fatalf("%s: text stays at %v in frame %d", code, p, frame)
			}
		}
	}
}

func TestRenderMotionNoise(t *testing.T) {
	media, err := renderMotionNoise(newCode(uppercaseChars))
	if err != nil {
		t.Fatalf("renderMotionNoise() error = %v", err)
	}
	if media.MIMEType != "image/gif" {
		t.Fatalf("MIMEType = %q, want image/gif", media.MIMEType)
	}
	if media.Width != displayWidth || media.Height != displayHeight {
		t.Fatalf("display size = %dx%d, want %dx%d", media.Width, media.Height, displayWidth, displayHeight)
	}
	video, err := gif.DecodeAll(bytes.NewReader(media.Data))
	if err != nil {
		t.Fatalf("gif.DecodeAll() error = %v", err)
	}
	if len(video.Image) != videoFrames {
		t.Fatalf("frames = %d, want %d", len(video.Image), videoFrames)
	}
	duration := 0
	for _, delay := range video.Delay {
		duration += delay
	}
	if duration != videoSeconds*100 {
		t.Fatalf("duration = %d cs, want %d cs", duration, videoSeconds*100)
	}
	if b := video.Image[0].Bounds(); b.Dx() != videoWidth || b.Dy() != videoHeight {
		t.Fatalf("frame size = %v, want %dx%d", b.Size(), videoWidth, videoHeight)
	}
}
