package biz

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	mrand "math/rand/v2"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	// Sizes below are in display (CSS) pixels, multiplied by pixelScale into video pixels.
	displayWidth  = 300
	displayHeight = 150
	pixelScale    = 1 // video pixels per display pixel; 1 matches the dot size of the reference video
	videoWidth    = displayWidth * pixelScale
	videoHeight   = displayHeight * pixelScale
	videoFPS      = 20 // must divide 100: GIF delays are whole centiseconds
	videoSeconds  = 5
	videoFrames   = videoFPS * videoSeconds
	frameDelay    = 100 / videoFPS   // centiseconds per frame
	textMaxSize   = 80 * pixelScale  // font size, shrunk for words wider than textMaxWidth
	textMaxWidth  = 280 * pixelScale // widest word; a narrow margin is enough since the text bounces
	textGap       = 12 * pixelScale  // between letters, wider than a stroke so the eye can split letters seen only through motion
	textStep      = 1                // video pixels per frame: the slowest speed that still moves the text every frame
	noiseStep     = 3                // video pixels per frame; longer jumps of 1 px dots look like flicker, not motion
	dotChroma     = 0.5              // share of color kept around each dot's luminance, like the reference video's compression
)

var directions = []image.Point{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} // up, down, left, right

// dotPalette spreads 27 colors evenly over the RGB cube, which reads as random RGB noise. Each
// color keeps its luminance, which carries the motion, but only dotChroma of its color.
var dotPalette = func() color.Palette {
	levels := []float64{42, 127, 212}
	var p color.Palette
	for _, r := range levels {
		for _, g := range levels {
			for _, b := range levels {
				l := 0.299*r + 0.587*g + 0.114*b
				c := func(v float64) uint8 { return uint8(l + dotChroma*(v-l)) }
				p = append(p, color.RGBA{c(r), c(g), c(b), 255})
			}
		}
	}
	return p
}()

// textFont is Go Medium, a sans-serif close to the letters of the reference video.
var textFont = func() *opentype.Font {
	f, err := opentype.Parse(gomedium.TTF)
	if err != nil {
		panic(err)
	}
	return f
}()

// renderMotionNoise hides the code in moving dot noise. A single frame is pure noise; the text
// only shows because its dots move with it, against a background moving another way.
func renderMotionNoise(code string) (Media, error) {
	rng := newRand()
	txt, err := newText(rng, code)
	if err != nil {
		return Media{}, err
	}

	// Background dots, longer along the motion axis so pixels entering the frame are new.
	noiseDir := directions[rng.IntN(len(directions))]
	noiseW, noiseH := videoWidth, videoHeight
	if noiseDir.X != 0 {
		noiseW += noiseStep * videoFrames
	} else {
		noiseH += noiseStep * videoFrames
	}
	noise := newDots(rng, noiseW, noiseH)

	// Frames only read the shared inputs, so they are drawn concurrently.
	bounds := image.Rect(0, 0, videoWidth, videoHeight)
	video := &gif.GIF{
		Image:  make([]*image.Paletted, videoFrames),
		Delay:  make([]int, videoFrames),
		Config: image.Config{ColorModel: dotPalette, Width: videoWidth, Height: videoHeight},
	}
	var wg sync.WaitGroup
	for i := range videoFrames {
		wg.Go(func() {
			frame := image.NewPaletted(bounds, dotPalette)
			shift := noiseDir.Mul(noiseStep * i)
			for y := range videoHeight {
				for x := range videoWidth {
					frame.SetColorIndex(x, y, noise.at(image.Pt(x, y).Sub(shift)))
				}
			}
			pos := txt.at(i)
			for y := range txt.mask.Rect.Dy() {
				for x := range txt.mask.Rect.Dx() {
					if txt.mask.AlphaAt(x, y).A >= 0x80 {
						frame.SetColorIndex(pos.X+x, pos.Y+y, txt.dots.at(image.Pt(x, y)))
					}
				}
			}
			video.Image[i], video.Delay[i] = frame, frameDelay
		})
	}
	wg.Wait()

	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, video); err != nil {
		return Media{}, err
	}
	return Media{Data: buf.Bytes(), MIMEType: "image/gif", Width: displayWidth, Height: displayHeight}, nil
}

// text is the whole code as one block of dots, drifting and bouncing off the frame edges.
type text struct {
	mask  *image.Alpha
	dots  dots        // travel with the text
	hi    image.Point // largest top-left corner that keeps the text inside the frame
	start image.Point
	dir   image.Point
}

// newText renders the code and gives it a random start and direction.
func newText(rng *mrand.Rand, code string) (text, error) {
	mask, err := textMask(code)
	if err != nil {
		return text{}, err
	}
	size := mask.Rect.Size()
	hi := image.Pt(videoWidth-size.X, videoHeight-size.Y)
	return text{
		mask:  mask,
		dots:  newDots(rng, size.X, size.Y),
		hi:    hi,
		start: image.Pt(rng.IntN(hi.X+1), rng.IntN(hi.Y+1)),
		dir:   directions[rng.IntN(len(directions))],
	}, nil
}

// at returns the text's top-left corner by frame.
func (t text) at(frame int) image.Point {
	moved := t.dir.Mul(textStep * frame)
	return image.Pt(bounce(t.start.X+moved.X, t.hi.X), bounce(t.start.Y+moved.Y, t.hi.Y))
}

// bounce folds an unbounded coordinate back into [0, hi], as if it reflected off both ends.
func bounce(p, hi int) int {
	if hi == 0 {
		return 0
	}
	p %= 2 * hi
	if p < 0 {
		p += 2 * hi
	}
	if p > hi {
		p = 2*hi - p
	}
	return p
}

// textMask renders code as one alpha mask, shrinking the font from textMaxSize until the word
// is at most textMaxWidth wide.
func textMask(code string) (*image.Alpha, error) {
	for size := float64(textMaxSize); ; size-- {
		face, err := opentype.NewFace(textFont, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
		if err != nil {
			return nil, err
		}
		mask := layoutWord(face, []rune(code))
		face.Close()
		if mask.Rect.Dx() <= textMaxWidth {
			return mask, nil
		}
	}
}

// layoutWord draws each letter textGap pixels after the ink of the previous one, into a mask
// cropped to the word's ink.
func layoutWord(face font.Face, letters []rune) *image.Alpha {
	xs := make([]fixed.Int26_6, len(letters))
	var right, top, bottom fixed.Int26_6
	for i, c := range letters {
		ink, _, _ := face.GlyphBounds(c)
		xs[i] = -ink.Min.X
		if i > 0 {
			xs[i] += right + fixed.I(textGap)
		}
		right = xs[i] + ink.Max.X
		top, bottom = min(top, ink.Min.Y), max(bottom, ink.Max.Y)
	}
	mask := image.NewAlpha(image.Rect(0, 0, right.Ceil(), (bottom - top).Ceil()))
	d := font.Drawer{Dst: mask, Src: image.Opaque, Face: face}
	for i, c := range letters {
		d.Dot = fixed.Point26_6{X: xs[i], Y: -top}
		d.DrawString(string(c))
	}
	return mask
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
		d.pix[i] = uint8(rng.IntN(len(dotPalette)))
	}
	return d
}

func (d dots) at(p image.Point) uint8 {
	x, y := (p.X%d.width+d.width)%d.width, (p.Y%d.height+d.height)%d.height
	return d.pix[y*d.width+x]
}
