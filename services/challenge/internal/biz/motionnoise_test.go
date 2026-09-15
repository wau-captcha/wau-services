package biz

import (
	"bytes"
	"image/gif"
	"testing"
)

func TestRenderMotionNoise(t *testing.T) {
	media, err := renderMotionNoise(newCode())
	if err != nil {
		t.Fatalf("renderMotionNoise() error = %v", err)
	}
	if media.MIMEType != "image/gif" {
		t.Fatalf("MIMEType = %q, want image/gif", media.MIMEType)
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
	if duration != 500 {
		t.Fatalf("duration = %d cs, want 500 cs", duration)
	}
	if b := video.Image[0].Bounds(); b.Dx() != videoWidth || b.Dy() != videoHeight {
		t.Fatalf("frame size = %v, want %dx%d", b.Size(), videoWidth, videoHeight)
	}
}
