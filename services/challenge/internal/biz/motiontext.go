package biz

import (
	_ "embed"
	"image"
	mrand "math/rand/v2"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// firaSansBold is Fira Sans Bold (SIL Open Font License, fonts/OFL.txt): heavy strokes, open
// counters and narrow capitals, so four of them fill the frame.
//
//go:embed fonts/FiraSans-Bold.ttf
var firaSansBold []byte

var textFont = func() *opentype.Font {
	parsed, err := opentype.Parse(firaSansBold)
	if err != nil {
		panic(err)
	}
	return parsed
}()

// movingText is the whole code as one block of dots, going textTravel pixels up or down and back
// once per loop.
type movingText struct {
	mask      *image.Alpha
	dots      dots // travel with the text
	start     image.Point
	direction int // +1 down first, -1 up first
}

// newText renders the code at a random spot that leaves room for its travel.
func newText(random *mrand.Rand, code string) (movingText, error) {
	mask, err := textMask(code)
	if err != nil {
		return movingText{}, err
	}
	size := mask.Rect.Size()
	top := textMargin + random.IntN(textArea.Y-size.Y-textTravel+1) // highest point of the travel
	text := movingText{
		mask:      mask,
		dots:      newDots(random, size.X, size.Y),
		start:     image.Pt(textMargin+random.IntN(textArea.X-size.X+1), top),
		direction: 1,
	}
	if random.IntN(2) == 0 {
		text.start.Y, text.direction = top+textTravel, -1
	}
	return text, nil
}

// cornerAt returns the text's top-left corner by frame: textStep a frame away from the start
// until half the loop, then back.
func (t movingText) cornerAt(frame int) image.Point {
	return t.start.Add(image.Pt(0, t.direction*textStep*min(frame, videoFrames-frame)))
}

// textSize is the font size whose capitals are textCapHeight tall.
var textSize = capitalsSize(textFont, textCapHeight)

func capitalsSize(typeface *opentype.Font, capHeight int) float64 {
	const probe = 100
	face, err := opentype.NewFace(typeface, &opentype.FaceOptions{Size: probe, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		panic(err)
	}
	defer face.Close()
	ink, _, _ := face.GlyphBounds('H')
	return probe * float64(capHeight) / float64((ink.Max.Y - ink.Min.Y).Round())
}

// textMask renders code as one alpha mask, shrinking the font from textSize until the word is no
// wider than textArea and leaves room in it for its travel.
func textMask(code string) (*image.Alpha, error) {
	letters := []rune(code)
	for size := textSize; ; size-- {
		face, err := opentype.NewFace(textFont, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
		if err != nil {
			return nil, err
		}
		letterX, ink := layoutWord(face, letters)
		if ink.Dx() <= textArea.X && ink.Dy() <= textArea.Y-textTravel {
			mask := drawWord(face, letters, letterX, ink)
			face.Close()
			return mask, nil
		}
		face.Close()
	}
}

// layoutWord places each letter textGap pixels after the ink of the previous one. It returns the
// letters' x origins and the word's ink box, which starts at x 0 and is measured from the baseline.
func layoutWord(face font.Face, letters []rune) ([]fixed.Int26_6, image.Rectangle) {
	letterX := make([]fixed.Int26_6, len(letters))
	var right, top, bottom fixed.Int26_6
	for i, letter := range letters {
		ink, _, _ := face.GlyphBounds(letter)
		letterX[i] = -ink.Min.X
		if i > 0 {
			letterX[i] += right + fixed.I(textGap)
		}
		right = letterX[i] + ink.Max.X
		top, bottom = min(top, ink.Min.Y), max(bottom, ink.Max.Y)
	}
	return letterX, image.Rect(0, top.Floor(), right.Ceil(), bottom.Ceil())
}

// drawWord draws the letters into a mask cropped to the word's ink.
func drawWord(face font.Face, letters []rune, letterX []fixed.Int26_6, ink image.Rectangle) *image.Alpha {
	mask := image.NewAlpha(image.Rect(0, 0, ink.Dx(), ink.Dy()))
	drawer := font.Drawer{Dst: mask, Src: image.Opaque, Face: face}
	for i, letter := range letters {
		drawer.Dot = fixed.Point26_6{X: letterX[i], Y: fixed.I(-ink.Min.Y)}
		drawer.DrawString(string(letter))
	}
	return mask
}
