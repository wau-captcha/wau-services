package biz

import (
	"bytes"
	"image/png"
	"testing"
)

func TestRenderImageDistort(t *testing.T) {
	media, err := renderImageDistort(newCode(alphanumericChars))
	if err != nil {
		t.Fatalf("renderImageDistort() error = %v", err)
	}
	if media.MIMEType != "image/png" {
		t.Fatalf("MIMEType = %q, want image/png", media.MIMEType)
	}
	img, err := png.Decode(bytes.NewReader(media.Data))
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
	if b := img.Bounds(); b.Dx() != imageWidth || b.Dy() != imageHeight {
		t.Fatalf("image size = %v, want %dx%d", b.Size(), imageWidth, imageHeight)
	}
}
