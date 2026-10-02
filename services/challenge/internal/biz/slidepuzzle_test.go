package biz

import (
	"bytes"
	"image/jpeg"
	"image/png"
	"strconv"
	"testing"
)

func TestGenerateSlidePuzzle(t *testing.T) {
	for i := range 100 {
		answer, media, err := generateSlidePuzzle()
		if err != nil {
			t.Fatal(err)
		}
		if media.MIMEType != "image/jpeg" || media.Width != slideWidth || media.Height != slideHeight {
			t.Fatalf("media %s %d × %d, want image/jpeg 300 × 150", media.MIMEType, media.Width, media.Height)
		}
		background, err := jpeg.Decode(bytes.NewReader(media.Data))
		if err != nil {
			t.Fatalf("background is not a JPEG: %v", err)
		}
		if size := background.Bounds().Size(); size.X != slideWidth || size.Y != slideHeight {
			t.Fatalf("background is %v, want 300 × 150", size)
		}

		piece := media.Piece
		if piece == nil {
			t.Fatal("no piece")
		}
		decoded, err := png.Decode(bytes.NewReader(piece.Data))
		if err != nil {
			t.Fatalf("piece is not a PNG: %v", err)
		}
		if size := decoded.Bounds().Size(); size.X != piece.Width || size.Y != piece.Height {
			t.Fatalf("piece PNG is %v, reported %d × %d", size, piece.Width, piece.Height)
		}
		transparent, opaque := 0, 0
		for y := range piece.Height {
			for x := range piece.Width {
				switch _, _, _, alpha := decoded.At(x, y).RGBA(); alpha {
				case 0:
					transparent++
				case 0xffff:
					opaque++
				}
			}
		}
		if transparent == 0 || opaque == 0 {
			t.Fatalf("piece has %d transparent and %d opaque pixels, want both", transparent, opaque)
		}
		if piece.Y < 0 || piece.Y+piece.Height > slideHeight {
			t.Fatalf("piece y %d with height %d leaves the image", piece.Y, piece.Height)
		}

		x, err := strconv.Atoi(answer)
		if err != nil {
			t.Fatalf("answer %q is not the target x", answer)
		}
		if x < piece.Width+holeStartGap || x > slideWidth-piece.Width-holeRightGap {
			t.Fatalf("render %d: target x %d outside [%d, %d]", i, x, piece.Width+holeStartGap, slideWidth-piece.Width-holeRightGap)
		}
	}
}

func TestPlaceHoles(t *testing.T) {
	random := newRandomSource()
	for range 2000 {
		shape := newPieceShape(random)
		decoy := newPieceShape(random)
		for decoy.sameTabsAs(shape) {
			decoy = newPieceShape(random)
		}
		x, y, decoyX, decoyY := placeHoles(random, shape, decoy)
		if gap := max(shape.width, decoy.width); decoyX > x-gap && decoyX < x+gap {
			t.Fatalf("decoy at %d is within %d px of the hole at %d", decoyX, gap, x)
		}
		if decoyX < 0 || decoyX+decoy.width > slideWidth || decoyY < 0 || decoyY+decoy.height > slideHeight {
			t.Fatalf("decoy at (%d, %d) leaves the image", decoyX, decoyY)
		}
		if y < 0 || y+shape.height > slideHeight {
			t.Fatalf("hole at y %d leaves the image", y)
		}
	}
}
