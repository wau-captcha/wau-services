package biz

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	mrand "math/rand/v2"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/f64"
	"golang.org/x/image/math/fixed"
)

const (
	imageWidth  = 200
	imageHeight = 70
	glyphTile   = 64
	// The darkest and lightest channel of the text; decoy strokes take theirs from the same range.
	inkDarkest, inkLightest = 0, 120
)

func renderImageDistort(code string) (Media, error) {
	random := newRandomSource()
	face, err := opentype.NewFace(codeFont, &opentype.FaceOptions{Size: 42, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return Media{}, err
	}
	defer face.Close()

	img := image.NewRGBA(image.Rect(0, 0, imageWidth, imageHeight))
	draw.Draw(img, img.Bounds(), image.NewUniform(randomColor(random, 210, 255)), image.Point{}, draw.Src)
	step := float64(imageWidth-20) / float64(len(code))
	for i, char := range code {
		glyph := renderGlyph(face, string(char), randomColor(random, inkDarkest, inkLightest))
		centerX := 10 + step*(float64(i)+0.5) + random.Float64()*6 - 3
		centerY := imageHeight/2 + random.Float64()*10 - 5
		draw.BiLinear.Transform(img, glyphPlacement(random, centerX, centerY), glyph, glyph.Bounds(), draw.Over, nil)
	}
	drawDecoys(img, random)
	img = waveWarp(img, random)
	addNoise(img, random)
	img = blurImage(img)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return Media{}, err
	}
	return Media{Data: buf.Bytes(), MIMEType: "image/png", Width: imageWidth, Height: imageHeight}, nil
}

func renderGlyph(face font.Face, char string, ink color.Color) *image.RGBA {
	tile := image.NewRGBA(image.Rect(0, 0, glyphTile, glyphTile))
	drawer := font.Drawer{Dst: tile, Src: image.NewUniform(ink), Face: face}
	metrics := face.Metrics()
	drawer.Dot = fixed.Point26_6{
		X: (fixed.I(glyphTile) - drawer.MeasureString(char)) / 2,
		Y: (fixed.I(glyphTile) + metrics.Ascent - metrics.Descent) / 2,
	}
	drawer.DrawString(char)
	return tile
}

// glyphPlacement maps a glyph tile onto the image: scaled on each axis, sheared sideways and
// rotated around the tile's centre, which lands at (centerX, centerY). Each letter gets its own
// shape, so no two letters of a code, or of two codes, are drawn alike.
func glyphPlacement(random *mrand.Rand, centerX, centerY float64) f64.Aff3 {
	// Stronger turns or shears make look-alikes (l and 1, O and D, 4 and A) hard to tell apart.
	sin, cos := math.Sincos((random.Float64()*2 - 1) * math.Pi / 9) // up to ±20°
	shear := (random.Float64()*2 - 1) * 0.25
	scaleX, scaleY := 0.85+random.Float64()*0.25, 0.9+random.Float64()*0.25
	// Rotation · shear · scale, as the matrix [a b; d e].
	a, b := cos*scaleX, (cos*shear-sin)*scaleY
	d, e := sin*scaleX, (sin*shear+cos)*scaleY
	halfTile := glyphTile / 2.0
	return f64.Aff3{
		a, b, centerX - (a+b)*halfTile,
		d, e, centerY - (d+e)*halfTile,
	}
}

// drawDecoys draws 2 to 4 strokes in the colours of the text across the band it is in: curves a
// segmenter takes for parts of letters, which a filter on colour cannot remove. They are one
// pixel wide, thinner than any letter, so a person still tells them from the letters.
func drawDecoys(img *image.RGBA, random *mrand.Rand) {
	for range 2 + random.IntN(3) {
		ink := randomColor(random, inkDarkest, inkLightest)
		// A quadratic Bézier curve between two points of the band, bent by a control point beside it.
		x0, y0 := random.Float64()*imageWidth, 15+random.Float64()*(imageHeight-30)
		length := 30 + random.Float64()*90
		angle := (random.Float64()*2 - 1) * math.Pi / 5
		x2, y2 := x0+length*math.Cos(angle), y0+length*math.Sin(angle)
		bend := (random.Float64()*2 - 1) * 18
		x1, y1 := (x0+x2)/2-bend*math.Sin(angle), (y0+y2)/2+bend*math.Cos(angle)
		steps := int(length * 3)
		for i := range steps + 1 {
			t := float64(i) / float64(steps)
			x := (1-t)*(1-t)*x0 + 2*(1-t)*t*x1 + t*t*x2
			y := (1-t)*(1-t)*y0 + 2*(1-t)*t*y1 + t*t*y2
			setPixel(img, int(x), int(y), ink)
		}
	}
}

func setPixel(img *image.RGBA, x, y int, ink color.RGBA) {
	if x >= 0 && x < imageWidth && y >= 0 && y < imageHeight {
		img.SetRGBA(x, y, ink)
	}
}

// waveWarp bends the whole image along two waves of random amplitude, period and phase: columns
// move up and down, rows sideways. It samples between pixels, so strokes bend without breaking.
func waveWarp(src *image.RGBA, random *mrand.Rand) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	amplitudeX, amplitudeY := 1.5+random.Float64()*2, 3+random.Float64()*3
	frequencyX, frequencyY := 2*math.Pi/(35+random.Float64()*35), 2*math.Pi/(70+random.Float64()*90)
	phaseX, phaseY := random.Float64()*2*math.Pi, random.Float64()*2*math.Pi
	for y := range imageHeight {
		for x := range imageWidth {
			sourceX := float64(x) + amplitudeX*math.Sin(float64(y)*frequencyX+phaseX)
			sourceY := float64(y) + amplitudeY*math.Sin(float64(x)*frequencyY+phaseY)
			dst.SetRGBA(x, y, bilinearAt(src, sourceX, sourceY))
		}
	}
	return dst
}

// bilinearAt blends the four pixels around (x, y).
func bilinearAt(img *image.RGBA, x, y float64) color.RGBA {
	left, top := math.Floor(x), math.Floor(y)
	fx, fy := x-left, y-top
	corners := [4]color.RGBA{
		pixelAt(img, int(left), int(top)), pixelAt(img, int(left)+1, int(top)),
		pixelAt(img, int(left), int(top)+1), pixelAt(img, int(left)+1, int(top)+1),
	}
	weights := [4]float64{(1 - fx) * (1 - fy), fx * (1 - fy), (1 - fx) * fy, fx * fy}
	var red, green, blue float64
	for i, corner := range corners {
		red += weights[i] * float64(corner.R)
		green += weights[i] * float64(corner.G)
		blue += weights[i] * float64(corner.B)
	}
	return color.RGBA{uint8(red + 0.5), uint8(green + 0.5), uint8(blue + 0.5), 255}
}

func addNoise(img *image.RGBA, random *mrand.Rand) {
	for range imageWidth * imageHeight / 10 {
		img.SetRGBA(random.IntN(imageWidth), random.IntN(imageHeight), randomColor(random, 0, 255))
	}
}

func blurImage(src *image.RGBA) *image.RGBA {
	kernel := [3][3]int{{1, 2, 1}, {2, 4, 2}, {1, 2, 1}}
	dst := image.NewRGBA(src.Bounds())
	for y := range imageHeight {
		for x := range imageWidth {
			var red, green, blue int
			for kernelY := range 3 {
				for kernelX := range 3 {
					sample, weight := pixelAt(src, x+kernelX-1, y+kernelY-1), kernel[kernelY][kernelX]
					red, green, blue = red+int(sample.R)*weight, green+int(sample.G)*weight, blue+int(sample.B)*weight
				}
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(red / 16), uint8(green / 16), uint8(blue / 16), 255})
		}
	}
	return dst
}

func pixelAt(img *image.RGBA, x, y int) color.RGBA {
	return img.RGBAAt(min(max(x, 0), imageWidth-1), min(max(y, 0), imageHeight-1))
}
