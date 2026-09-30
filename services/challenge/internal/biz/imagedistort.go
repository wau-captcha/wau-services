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
		glyph := renderGlyph(face, string(char), randomColor(random, 0, 120))
		sin, cos := math.Sincos((random.Float64()*2 - 1) * math.Pi / 6) // up to ±30°
		scale := 0.9 + random.Float64()*0.2
		centerX := 10 + step*(float64(i)+0.5) + random.Float64()*6 - 3
		centerY := imageHeight/2 + random.Float64()*10 - 5
		// Rotate and scale the tile around its centre, then place that centre at (centerX, centerY).
		scaledCos, scaledSin := scale*cos, scale*sin
		halfTile := glyphTile / 2.0
		placement := f64.Aff3{
			scaledCos, -scaledSin, centerX - (scaledCos-scaledSin)*halfTile,
			scaledSin, scaledCos, centerY - (scaledSin+scaledCos)*halfTile,
		}
		draw.BiLinear.Transform(img, placement, glyph, glyph.Bounds(), draw.Over, nil)
	}
	for range 2 {
		drawCurve(img, random)
	}
	img = warpImage(img, random)
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

func drawCurve(img *image.RGBA, random *mrand.Rand) {
	ink := randomColor(random, 0, 140)
	baseline := imageHeight * (0.3 + random.Float64()*0.4)
	amplitude := 5 + random.Float64()*10
	frequency := (1 + random.Float64()*2) * 2 * math.Pi / imageWidth
	phase := random.Float64() * 2 * math.Pi
	for x := range imageWidth {
		y := int(baseline + amplitude*math.Sin(frequency*float64(x)+phase))
		img.SetRGBA(x, y, ink)
		img.SetRGBA(x, y+1, ink)
	}
}

func warpImage(src *image.RGBA, random *mrand.Rand) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	amplitudeX, amplitudeY := 2+random.Float64()*2, 2+random.Float64()*3
	frequencyX, frequencyY := 2*math.Pi/(40+random.Float64()*40), 2*math.Pi/(60+random.Float64()*80)
	phaseX, phaseY := random.Float64()*2*math.Pi, random.Float64()*2*math.Pi
	for y := range imageHeight {
		for x := range imageWidth {
			shiftX := amplitudeX * math.Sin(float64(y)*frequencyX+phaseX)
			shiftY := amplitudeY * math.Sin(float64(x)*frequencyY+phaseY)
			dst.SetRGBA(x, y, pixelAt(src, x+int(shiftX), y+int(shiftY)))
		}
	}
	return dst
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
