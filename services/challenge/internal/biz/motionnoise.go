package biz

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"math"
	mrand "math/rand/v2"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	videoWidth   = 200
	videoHeight  = 100
	videoFrames  = 50 // 5 seconds at 10 fps
	frameDelay   = 10 // centiseconds per frame
	dotColors    = 8
	textFontSize = 40
	textStep     = 1 // pixels per frame on each axis
	noiseStep    = 2 // pixels per frame
)

var (
	textDirections  = []image.Point{{1, -1}, {-1, -1}, {1, 1}, {-1, 1}} // up-right, up-left, down-right, down-left
	noiseDirections = []image.Point{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}   // up, down, left, right
)

// renderMotionNoise hides the code in moving dot noise. A single frame is pure noise;
// the text only shows because it moves differently from the background.
func renderMotionNoise(code string) (Media, error) {
	rng := newRand()
	mask, err := textMask(code)
	if err != nil {
		return Media{}, err
	}
	textSize := mask.Rect.Size()
	textDir := textDirections[rng.IntN(len(textDirections))]
	noiseDir := noiseDirections[rng.IntN(len(noiseDirections))]
	textTravel := textStep * (videoFrames - 1)
	noiseTravel := noiseStep * videoFrames

	// Hues are spread around the color wheel so neighbouring dots always contrast.
	hue := rng.Float64()
	palette := make(color.Palette, dotColors)
	for i := range palette {
		palette[i] = hsvColor(hue+float64(i)/dotColors, 0.5+0.5*rng.Float64(), 0.4+0.6*rng.Float64())
	}

	// Step 1: background dots, longer along the motion axis so pixels entering the frame are new.
	noiseW, noiseH := videoWidth, videoHeight
	if noiseDir.X != 0 {
		noiseW += noiseTravel
	} else {
		noiseH += noiseTravel
	}
	noise := newDots(rng, noiseW, noiseH)

	// Step 2: text dots, drawn only inside the glyphs; the rest of the text box is transparent.
	text := newDots(rng, textSize.X, textSize.Y)

	// Step 3: random start that keeps the whole text path inside the frame.
	start := image.Pt(
		startOffset(rng, videoWidth-textSize.X, textTravel, textDir.X),
		startOffset(rng, videoHeight-textSize.Y, textTravel, textDir.Y),
	)

	// Step 4: text moves diagonally while the background moves along one axis.
	bounds := image.Rect(0, 0, videoWidth, videoHeight)
	video := &gif.GIF{Config: image.Config{ColorModel: palette, Width: videoWidth, Height: videoHeight}}
	for i := range videoFrames {
		textPos := start.Add(textDir.Mul(textStep * i))
		noiseShift := noiseDir.Mul(noiseStep * i)
		frame := image.NewPaletted(bounds, palette)
		for y := range videoHeight {
			for x := range videoWidth {
				p := image.Pt(x, y)
				if t := p.Sub(textPos); t.In(mask.Rect) && mask.AlphaAt(t.X, t.Y).A >= 0x80 {
					frame.SetColorIndex(x, y, text.at(t))
				} else {
					frame.SetColorIndex(x, y, noise.at(p.Sub(noiseShift)))
				}
			}
		}
		video.Image = append(video.Image, frame)
		video.Delay = append(video.Delay, frameDelay)
	}

	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, video); err != nil {
		return Media{}, err
	}
	return Media{Data: buf.Bytes(), MIMEType: "image/gif"}, nil
}

// textMask renders code as an alpha mask cropped to the text box.
func textMask(code string) (*image.Alpha, error) {
	face, err := opentype.NewFace(codeFont, &opentype.FaceOptions{Size: textFontSize, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, err
	}
	defer face.Close()
	m := face.Metrics()
	d := font.Drawer{Src: image.Opaque, Face: face}
	mask := image.NewAlpha(image.Rect(0, 0, d.MeasureString(code).Ceil(), (m.Ascent + m.Descent).Ceil()))
	d.Dst = mask
	d.Dot = fixed.Point26_6{Y: m.Ascent}
	d.DrawString(code)
	return mask, nil
}

// startOffset picks a start coordinate so that travelling in dir stays within [0, slack].
func startOffset(rng *mrand.Rand, slack, travel, dir int) int {
	offset := rng.IntN(slack - travel + 1)
	if dir < 0 {
		offset += travel
	}
	return offset
}

// hsvColor converts hue (in turns, any value), saturation and value in [0, 1] to RGB.
func hsvColor(h, s, v float64) color.RGBA {
	channel := func(n float64) uint8 {
		k := math.Mod(n+h*6, 6)
		return uint8(255 * (v - v*s*max(0, min(k, 4-k, 1))))
	}
	return color.RGBA{channel(5), channel(3), channel(1), 255}
}

// dots is a grid of one-pixel random palette indices that wraps at its edges.
// Dots are single pixels because larger dots leave visible seams along the glyph edges.
type dots struct {
	width, height int
	pix           []uint8
}

func newDots(rng *mrand.Rand, width, height int) dots {
	d := dots{width: width, height: height, pix: make([]uint8, width*height)}
	for i := range d.pix {
		d.pix[i] = uint8(rng.IntN(dotColors))
	}
	return d
}

func (d dots) at(p image.Point) uint8 {
	x, y := (p.X%d.width+d.width)%d.width, (p.Y%d.height+d.height)%d.height
	return d.pix[y*d.width+x]
}
