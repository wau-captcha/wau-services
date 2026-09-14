package biz

import (
	"bytes"
	"image/png"
	"strings"
	"testing"
)

func TestNewImageDistort(t *testing.T) {
	code, img, err := NewImageDistort()
	if err != nil {
		t.Fatalf("NewImageDistort() error = %v", err)
	}
	if len(code) != codeLength || strings.Trim(code, codeChars) != "" {
		t.Fatalf("code = %q, want %d chars from %q", code, codeLength, codeChars)
	}
	decoded, err := png.Decode(bytes.NewReader(img))
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
	if b := decoded.Bounds(); b.Dx() != imageWidth || b.Dy() != imageHeight {
		t.Fatalf("image size = %v, want %dx%d", b.Size(), imageWidth, imageHeight)
	}
}
