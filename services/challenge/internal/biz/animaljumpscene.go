package biz

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"math"
	mrand "math/rand/v2"

	"golang.org/x/image/vector"
)

const (
	// Drawn at the display size. At twice the size both rounds came to 500-730 KB, over the 400 KB
	// budget; at this size they stay well under it (see TestAnimalJumpSize).
	jumpScale      = 1
	jumpFrames     = 50 // 2000 ms at 25 fps, played once
	jumpFrameMs    = 40
	jumpFrameDelay = 4 // centiseconds
	// A head lands a little squashed for two frames, which makes a hop read as a jump.
	jumpSquashMs   = 80
	jumpSquashWide = 1.1
	jumpSquashFlat = 0.9
	// One palette for every frame of a round; index 0 is transparent, for the pixels a frame keeps
	// from the one before.
	jumpPaletteSize   = 64
	jumpGradientSteps = 14
	// Clouds and leaves drift sideways the whole round, so a frame difference finds more than the
	// heads; they sway at most this much either way, too little to pass for a jump.
	jumpMinDrifters = 3
	jumpMaxDrifters = 5
	jumpDriftSway   = 2.5
)

// sprite is a picture with alpha, at device position (x, y) when its owner is at rest.
type sprite struct {
	image *image.RGBA
	x, y  int
}

// drawSprite paints parts into a sprite covering the CSS box (x0, y0)–(x1, y1), whose corner is
// snapped to a device pixel so moving it by whole device pixels never resamples it.
func drawSprite(raster *vector.Rasterizer, parts []part, radius, x0, y0, x1, y1 float64, place func(x, y float64) (float64, float64)) sprite {
	left, top := int(math.Floor(x0*jumpScale)), int(math.Floor(y0*jumpScale))
	width, height := int(math.Ceil(x1*jumpScale))-left+1, int(math.Ceil(y1*jumpScale))-top+1
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	drawParts(img, raster, parts, radius, jumpScale, float64(left)/jumpScale, float64(top)/jumpScale, place)
	return sprite{image: img, x: left, y: top}
}

// headSprites draws a head at rest, and squashed as it lands: wider and flatter about its feet.
func headSprites(raster *vector.Rasterizer, head jumpHead, use func(color.NRGBA)) (rest, squashed sprite) {
	tint := func(paint color.NRGBA) color.NRGBA {
		return color.NRGBA{clampByte(float64(paint.R) * head.tint[0]), clampByte(float64(paint.G) * head.tint[1]), clampByte(float64(paint.B) * head.tint[2]), 255}
	}
	parts := allSpecies[head.species].parts(tint)
	for _, p := range parts {
		use(p.paint)
	}
	use(outlinePaint)
	for i, factor := range [][2]float64{{1, 1}, {jumpSquashWide, jumpSquashFlat}} {
		place := func(x, y float64) (float64, float64) {
			cssX, cssY := head.centreX+x*head.radius, head.centreY+y*head.radius
			return head.centreX + (cssX-head.centreX)*factor[0], jumpGround - (jumpGround-cssY)*factor[1]
		}
		spread := (head.right - head.left) * (factor[0] - 1) / 2
		drawn := drawSprite(raster, parts, head.radius, head.left-spread-3, head.top-3, head.right+spread+3, jumpGround+3, place)
		if i == 0 {
			rest = drawn
		} else {
			squashed = drawn
		}
	}
	return rest, squashed
}

// drifter is a cloud or a leaf crossing the sky at speed CSS px a second.
type drifter struct {
	sprite
	speed, sway, phase float64
}

func (d drifter) at(t float64) (int, int) {
	return d.x + int(math.Round(d.speed*t/1000*jumpScale)),
		d.y + int(math.Round(d.sway*math.Sin(d.phase+2*math.Pi*t/(jumpFrames*jumpFrameMs))*jumpScale))
}

func makeDrifters(random *mrand.Rand, raster *vector.Rasterizer, use func(color.NRGBA)) []drifter {
	drifters := make([]drifter, jumpMinDrifters+random.IntN(jumpMaxDrifters-jumpMinDrifters+1))
	for i := range drifters {
		x := between(random, -10, jumpWidth-10)
		var parts []part
		var radius, y float64
		if random.IntN(10) < 7 {
			radius, y = between(random, 11, 18), between(random, 14, 70)
			white, shade := color.NRGBA{250, 251, 253, 255}, color.NRGBA{226, 232, 240, 255}
			parts = []part{
				{shade, ellipse{0, 0.15, 1, 0.42, 0}, false},
				{white, ellipse{-0.42, 0, 0.5, 0.42, 0}, false},
				{white, ellipse{0.1, -0.18, 0.55, 0.5, 0}, false},
				{white, ellipse{0.55, 0.02, 0.42, 0.36, 0}, false},
			}
		} else {
			radius, y = between(random, 4.5, 6.5), between(random, 12, 100)
			leaf := []color.NRGBA{{110, 176, 82, 255}, {236, 156, 60, 255}, {222, 190, 70, 255}}[random.IntN(3)]
			angle := random.Float64() * math.Pi
			sin, cos := math.Sincos(angle)
			parts = []part{
				{leaf, ellipse{0, 0, 1, 0.45, angle}, false},
				{color.NRGBA{leaf.R / 2, leaf.G / 2, leaf.B / 2, 255}, stroke(-0.8*cos, -0.8*sin, 0.8*cos, 0.8*sin, 0.1), false},
			}
		}
		for _, p := range parts {
			use(p.paint)
		}
		place := func(px, py float64) (float64, float64) { return x + px*radius, y + py*radius }
		speed := between(random, 6, 24)
		if random.IntN(2) == 0 {
			speed = -speed
		}
		drifters[i] = drifter{
			sprite: drawSprite(raster, parts, radius, x-radius-2, y-radius-2, x+radius+2, y+radius+2, place),
			speed:  speed, sway: random.Float64() * jumpDriftSway, phase: random.Float64() * 2 * math.Pi,
		}
	}
	return drifters
}

// drawScenery paints the still background: a soft two-colour gradient and the grass the heads
// stand on. The gradient's steps go into the palette, so it stays smooth once quantized.
func drawScenery(random *mrand.Rand, raster *vector.Rasterizer, use func(color.NRGBA)) *image.RGBA {
	from, to := randomColor(random, 172, 236), randomColor(random, 172, 236)
	mix := func(share float64) color.RGBA {
		blend := func(a, b uint8) uint8 { return clampByte(float64(a) + (float64(b)-float64(a))*share) }
		return color.RGBA{blend(from.R, to.R), blend(from.G, to.G), blend(from.B, to.B), 255}
	}
	for step := range jumpGradientSteps {
		paint := mix(float64(step) / (jumpGradientSteps - 1))
		use(color.NRGBA{paint.R, paint.G, paint.B, 255})
	}
	img := image.NewRGBA(image.Rect(0, 0, jumpWidth*jumpScale, jumpHeight*jumpScale))
	sin, cos := math.Sincos(math.Pi/2 + (random.Float64()*2-1)*0.5) // mostly top to bottom
	reach := math.Abs(cos)*jumpWidth/2 + math.Abs(sin)*jumpHeight/2
	for y := range jumpHeight * jumpScale {
		for x := range jumpWidth * jumpScale {
			along := (float64(x)/jumpScale-jumpWidth/2)*cos + (float64(y)/jumpScale-jumpHeight/2)*sin
			img.SetRGBA(x, y, mix(along/reach/2+0.5))
		}
	}
	light, dark := color.NRGBA{124, 190, 92, 255}, color.NRGBA{86, 152, 66, 255}
	use(light)
	use(dark)
	phase := random.Float64() * 2 * math.Pi
	grass := polygon{{-1, jumpHeight + 1}}
	for x := -1.0; x <= jumpWidth+1; x += 3 {
		edge := jumpGround - 1 + math.Sin(x*0.08+phase)*1.2
		if int(x)%9 == 2 {
			edge -= 3 // a blade
		}
		grass = append(grass, [2]float64{x, edge})
	}
	grass = append(grass, [2]float64{jumpWidth + 1, jumpHeight + 1})
	identity := func(x, y float64) (float64, float64) { return x, y }
	drawParts(img, raster, []part{{light, grass, false}, {dark, polygon{{-1, 145}, {jumpWidth + 1, 145}, {jumpWidth + 1, jumpHeight + 1}, {-1, jumpHeight + 1}}, false}},
		1, jumpScale, 0, 0, identity)
	return img
}

// renderRound draws the round's 50 frames and encodes them as a GIF that plays once. Every frame
// but the first holds only the pixels that changed, the rest transparent, which keeps it small.
func renderRound(random *mrand.Rand, round jumpRound) ([]byte, error) {
	raster := vector.NewRasterizer(1, 1)
	var paints []color.NRGBA
	seen := map[color.NRGBA]bool{}
	use := func(paint color.NRGBA) {
		if !seen[paint] {
			seen[paint] = true
			paints = append(paints, paint)
		}
	}
	scenery := drawScenery(random, raster, use)
	drifters := makeDrifters(random, raster, use)
	var rest, squashed [jumpHeads]sprite
	for i, head := range round.heads {
		rest[i], squashed[i] = headSprites(raster, head, use)
	}

	frame := image.NewRGBA(scenery.Bounds())
	compose := func(index int) {
		t := float64(index * jumpFrameMs)
		copy(frame.Pix, scenery.Pix)
		for _, d := range drifters {
			x, y := d.at(t)
			paste(frame, d.image, x, y)
		}
		for i, head := range round.heads {
			height, landed := 0.0, false
			for _, j := range head.jumps {
				height = max(height, j.heightAt(t))
				landed = landed || (t > j.end() && t <= j.end()+jumpSquashMs)
			}
			// The last frame stays on screen when the round ends: every head at rest.
			if landed && height == 0 && index < jumpFrames-1 {
				paste(frame, squashed[i].image, squashed[i].x, squashed[i].y)
				continue
			}
			paste(frame, rest[i].image, rest[i].x, rest[i].y-int(math.Round(height*jumpScale)))
		}
	}

	compose(0)
	palette := newJumpPalette(frame, paints)
	animation := &gif.GIF{LoopCount: -1, Config: image.Config{ColorModel: palette.colors, Width: frame.Rect.Dx(), Height: frame.Rect.Dy()}}
	var previous []uint8
	for index := range jumpFrames {
		if index > 0 {
			compose(index)
		}
		current := palette.quantize(frame)
		animation.Image = append(animation.Image, changedPixels(previous, current, frame.Rect, palette.colors))
		animation.Delay = append(animation.Delay, jumpFrameDelay)
		animation.Disposal = append(animation.Disposal, gif.DisposalNone)
		previous = current
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, animation); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func paste(dst, src *image.RGBA, x, y int) {
	draw.Draw(dst, src.Bounds().Add(image.Pt(x, y)), src, image.Point{}, draw.Over)
}

// changedPixels is the frame as a GIF image block: all of it the first time, afterwards the box
// around the pixels that differ from the previous frame. A pixel that did not change may show
// either its colour or transparency; it takes whichever continues the run before it, because LZW
// packs long runs of one index and a head moving over flat fur leaves changed and unchanged
// pixels side by side.
func changedPixels(previous, current []uint8, bounds image.Rectangle, colors color.Palette) *image.Paletted {
	width := bounds.Dx()
	if previous == nil {
		return &image.Paletted{Pix: current, Stride: width, Rect: bounds, Palette: colors}
	}
	box := image.Rectangle{}
	for i := range current {
		if current[i] != previous[i] {
			box = box.Union(image.Rect(i%width, i/width, i%width+1, i/width+1))
		}
	}
	if box.Empty() {
		box = image.Rect(0, 0, 1, 1)
	}
	img := image.NewPaletted(box, colors)
	last := uint8(0)
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			i := y*width + x
			if current[i] != previous[i] || current[i] == last {
				last = current[i]
			} else {
				last = 0
			}
			img.Pix[img.PixOffset(x, y)] = last
		}
	}
	return img
}
