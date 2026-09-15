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
	rng := newRand()
	face, err := opentype.NewFace(codeFont, &opentype.FaceOptions{Size: 42, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return Media{}, err
	}
	defer face.Close()

	img := image.NewRGBA(image.Rect(0, 0, imageWidth, imageHeight))
	draw.Draw(img, img.Bounds(), image.NewUniform(randomColor(rng, 210, 255)), image.Point{}, draw.Src)
	step := float64(imageWidth-20) / codeLength
	for i, c := range code {
		glyph := renderGlyph(face, string(c), randomColor(rng, 0, 120))
		sin, cos := math.Sincos((rng.Float64()*2 - 1) * math.Pi / 6) // up to ±30°
		scale := 0.9 + rng.Float64()*0.2
		cx := 10 + step*(float64(i)+0.5) + rng.Float64()*6 - 3
		cy := imageHeight/2 + rng.Float64()*10 - 5
		a, b, d, e := scale*cos, -scale*sin, scale*sin, scale*cos
		h := glyphTile / 2.0
		draw.BiLinear.Transform(img, f64.Aff3{a, b, cx - (a+b)*h, d, e, cy - (d+e)*h}, glyph, glyph.Bounds(), draw.Over, nil)
	}
	for range 2 {
		drawCurve(img, rng)
	}
	img = warp(img, rng)
	addNoise(img, rng)
	img = blur(img)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return Media{}, err
	}
	return Media{Data: buf.Bytes(), MIMEType: "image/png"}, nil
}

func renderGlyph(face font.Face, s string, c color.Color) *image.RGBA {
	tile := image.NewRGBA(image.Rect(0, 0, glyphTile, glyphTile))
	d := font.Drawer{Dst: tile, Src: image.NewUniform(c), Face: face}
	m := face.Metrics()
	d.Dot = fixed.Point26_6{
		X: (fixed.I(glyphTile) - d.MeasureString(s)) / 2,
		Y: (fixed.I(glyphTile) + m.Ascent - m.Descent) / 2,
	}
	d.DrawString(s)
	return tile
}

func drawCurve(img *image.RGBA, rng *mrand.Rand) {
	c := randomColor(rng, 0, 140)
	y0 := imageHeight * (0.3 + rng.Float64()*0.4)
	amp := 5 + rng.Float64()*10
	freq := (1 + rng.Float64()*2) * 2 * math.Pi / imageWidth
	phase := rng.Float64() * 2 * math.Pi
	for x := range imageWidth {
		y := int(y0 + amp*math.Sin(freq*float64(x)+phase))
		img.SetRGBA(x, y, c)
		img.SetRGBA(x, y+1, c)
	}
}

func warp(src *image.RGBA, rng *mrand.Rand) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	ax, ay := 2+rng.Float64()*2, 2+rng.Float64()*3
	fx, fy := 2*math.Pi/(40+rng.Float64()*40), 2*math.Pi/(60+rng.Float64()*80)
	px, py := rng.Float64()*2*math.Pi, rng.Float64()*2*math.Pi
	for y := range imageHeight {
		for x := range imageWidth {
			dx := ax * math.Sin(float64(y)*fx+px)
			dy := ay * math.Sin(float64(x)*fy+py)
			dst.SetRGBA(x, y, pixel(src, x+int(dx), y+int(dy)))
		}
	}
	return dst
}

func addNoise(img *image.RGBA, rng *mrand.Rand) {
	for range imageWidth * imageHeight / 10 {
		img.SetRGBA(rng.IntN(imageWidth), rng.IntN(imageHeight), randomColor(rng, 0, 255))
	}
}

func blur(src *image.RGBA) *image.RGBA {
	kernel := [3][3]int{{1, 2, 1}, {2, 4, 2}, {1, 2, 1}}
	dst := image.NewRGBA(src.Bounds())
	for y := range imageHeight {
		for x := range imageWidth {
			var r, g, b int
			for ky := range 3 {
				for kx := range 3 {
					p, w := pixel(src, x+kx-1, y+ky-1), kernel[ky][kx]
					r, g, b = r+int(p.R)*w, g+int(p.G)*w, b+int(p.B)*w
				}
			}
			dst.SetRGBA(x, y, color.RGBA{uint8(r / 16), uint8(g / 16), uint8(b / 16), 255})
		}
	}
	return dst
}

func pixel(img *image.RGBA, x, y int) color.RGBA {
	return img.RGBAAt(min(max(x, 0), imageWidth-1), min(max(y, 0), imageHeight-1))
}
