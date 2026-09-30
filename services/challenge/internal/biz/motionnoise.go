package biz

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	mrand "math/rand/v2"
	"sync"
)

const (
	// Sizes below are in display (CSS) pixels, multiplied by pixelScale into video pixels.
	displayWidth  = 300
	displayHeight = 150
	pixelScale    = 1 // video pixels per display pixel; 1 matches the dot size of the reference video
	videoWidth    = displayWidth * pixelScale
	videoHeight   = displayHeight * pixelScale
	videoFPS      = 50  // must divide 100 (GIF delays are whole centiseconds); browsers play 50 at most
	videoFrames   = 100 // 2 s, looping without a seam: text and background end where they started
	frameDelay    = 100 / videoFPS
	textCapHeight = 72 * pixelScale            // height of the capitals; codes too wide for textArea get smaller ones
	textMargin    = 8 * pixelScale             // kept clear around the text's whole path
	textGap       = 16 * pixelScale            // between letters, wider than a stroke so the eye can split letters seen only through motion
	textStep      = 1                          // video pixels per frame: the slowest speed that still moves the text every frame
	textTravel    = textStep * videoFrames / 2 // down or up, then back, once per loop
	noiseStep     = 3                          // video pixels per frame; longer jumps of 1 px dots look like flicker, not motion
	noisePeriod   = noiseStep * videoFrames    // background width: one loop moves it by exactly one period
	dotChroma     = 0.5                        // share of color kept around each dot's luminance, like the reference video's compression
)

// The background must not repeat within a frame.
const _ uint = noisePeriod - videoWidth

// textArea is where the text may go: the frame inside textMargin.
var textArea = image.Pt(videoWidth-2*textMargin, videoHeight-2*textMargin)

// dotPalette spreads 27 colors evenly over the RGB cube, which reads as random RGB noise. Each
// color keeps its luminance, which carries the motion, but only dotChroma of its color.
var dotPalette = func() color.Palette {
	levels := []float64{42, 127, 212}
	var palette color.Palette
	for _, red := range levels {
		for _, green := range levels {
			for _, blue := range levels {
				luminance := 0.299*red + 0.587*green + 0.114*blue
				desaturate := func(channel float64) uint8 { return uint8(luminance + dotChroma*(channel-luminance)) }
				palette = append(palette, color.RGBA{desaturate(red), desaturate(green), desaturate(blue), 255})
			}
		}
	}
	return palette
}()

// renderMotionNoise hides the code in moving dot noise. A single frame is pure noise; the text
// only shows because its dots move with it, up and down, across a background flowing sideways.
func renderMotionNoise(code string) (Media, error) {
	random := newRandomSource()
	textBlock, err := newText(random, code)
	if err != nil {
		return Media{}, err
	}
	noise := newDots(random, noisePeriod, videoHeight)
	noiseDirection := image.Pt(1-2*random.IntN(2), 0)

	// Frames only read the shared inputs, so they are drawn concurrently.
	bounds := image.Rect(0, 0, videoWidth, videoHeight)
	video := &gif.GIF{
		Image:  make([]*image.Paletted, videoFrames),
		Delay:  make([]int, videoFrames),
		Config: image.Config{ColorModel: dotPalette, Width: videoWidth, Height: videoHeight},
	}
	var frames sync.WaitGroup
	for i := range videoFrames {
		frames.Go(func() {
			frame := image.NewPaletted(bounds, dotPalette)
			shift := noiseDirection.Mul(noiseStep * i)
			for y := range videoHeight {
				for x := range videoWidth {
					frame.SetColorIndex(x, y, noise.indexAt(image.Pt(x, y).Sub(shift)))
				}
			}
			textCorner := textBlock.cornerAt(i)
			for y := range textBlock.mask.Rect.Dy() {
				for x := range textBlock.mask.Rect.Dx() {
					if textBlock.mask.AlphaAt(x, y).A >= 0x80 {
						frame.SetColorIndex(textCorner.X+x, textCorner.Y+y, textBlock.dots.indexAt(image.Pt(x, y)))
					}
				}
			}
			video.Image[i], video.Delay[i] = frame, frameDelay
		})
	}
	frames.Wait()

	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, video); err != nil {
		return Media{}, err
	}
	return Media{Data: buf.Bytes(), MIMEType: "image/gif", Width: displayWidth, Height: displayHeight}, nil
}

// dots is a grid of one-pixel random palette indices that wraps at its edges.
// Dots are single pixels because larger dots leave visible seams along the glyph edges.
type dots struct {
	width, height int
	indices       []uint8
}

func newDots(random *mrand.Rand, width, height int) dots {
	grid := dots{width: width, height: height, indices: make([]uint8, width*height)}
	for i := range grid.indices {
		grid.indices[i] = uint8(random.IntN(len(dotPalette)))
	}
	return grid
}

func (d dots) indexAt(point image.Point) uint8 {
	x, y := (point.X%d.width+d.width)%d.width, (point.Y%d.height+d.height)%d.height
	return d.indices[y*d.width+x]
}
