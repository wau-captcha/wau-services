package biz

import (
	"image"
	"image/color"
	"math"
	mrand "math/rand/v2"
)

// The background is drawn fresh for every challenge, never taken from a set of photos, so there is
// no original to diff it against. Many overlapping shapes give edge detection plenty to find
// besides the holes.
const (
	backgroundShapes = 36
	backgroundNoise  = 12 // most a pixel's brightness moves, out of 255
)

// canvas is an RGB image in floats, so many translucent layers blend without rounding.
type canvas struct {
	pixels []float64 // red, green, blue per pixel, row by row
}

func newCanvas() *canvas {
	return &canvas{pixels: make([]float64, slideWidth*slideHeight*3)}
}

func (c *canvas) at(x, y int) (red, green, blue float64) {
	index := (y*slideWidth + x) * 3
	return c.pixels[index], c.pixels[index+1], c.pixels[index+2]
}

// update replaces each channel of the pixel at (x, y) with change(channel).
func (c *canvas) update(x, y int, change func(channel float64) float64) {
	index := (y*slideWidth + x) * 3
	for channel := range 3 {
		c.pixels[index+channel] = change(c.pixels[index+channel])
	}
}

// blend lays paint over the pixel at (x, y) with opacity alpha.
func (c *canvas) blend(x, y int, paint [3]float64, alpha float64) {
	if x < 0 || x >= slideWidth || y < 0 || y >= slideHeight || alpha <= 0 {
		return
	}
	index := (y*slideWidth + x) * 3
	for channel := range 3 {
		c.pixels[index+channel] += (paint[channel] - c.pixels[index+channel]) * min(alpha, 1)
	}
}

func (c *canvas) image() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, slideWidth, slideHeight))
	for y := range slideHeight {
		for x := range slideWidth {
			red, green, blue := c.at(x, y)
			img.SetRGBA(x, y, color.RGBA{clampByte(red), clampByte(green), clampByte(blue), 255})
		}
	}
	return img
}

// drawBackground paints a gradient, then soft circles, rectangles and polygons in many colours,
// then light noise and a slight blur.
func drawBackground(random *mrand.Rand) *canvas {
	background := newCanvas()
	from, to := randomPaint(random, 30, 225), randomPaint(random, 30, 225)
	sin, cos := math.Sincos(random.Float64() * 2 * math.Pi)
	reach := math.Hypot(slideWidth, slideHeight) / 2
	for y := range slideHeight {
		for x := range slideWidth {
			share := ((float64(x)-slideWidth/2)*cos+(float64(y)-slideHeight/2)*sin)/reach/2 + 0.5
			for channel := range 3 {
				background.pixels[(y*slideWidth+x)*3+channel] = from[channel] + (to[channel]-from[channel])*share
			}
		}
	}
	for range backgroundShapes {
		drawShape(background, random)
	}
	for y := range slideHeight {
		for x := range slideWidth {
			shift := (random.Float64()*2 - 1) * backgroundNoise
			background.update(x, y, func(channel float64) float64 { return channel + shift })
		}
	}
	background.blur()
	return background
}

// drawShape paints one random shape. Its edge is soft over feather pixels, from crisp to blurry,
// so the picture has edges of every strength.
func drawShape(background *canvas, random *mrand.Rand) {
	paint := randomPaint(random, 0, 255)
	opacity := 0.3 + random.Float64()*0.5
	feather := 0.7 + random.Float64()*2.5
	centreX, centreY := random.Float64()*slideWidth, random.Float64()*slideHeight
	var distance func(x, y float64) float64 // negative inside, in pixels from the edge
	var extent float64
	switch random.IntN(3) {
	case 0:
		radius := 6 + random.Float64()*32
		extent = radius
		distance = func(x, y float64) float64 { return math.Hypot(x-centreX, y-centreY) - radius }
	case 1:
		halfWidth, halfHeight := 5+random.Float64()*35, 5+random.Float64()*25
		sin, cos := math.Sincos(random.Float64() * math.Pi)
		extent = math.Hypot(halfWidth, halfHeight)
		distance = func(x, y float64) float64 {
			along, across := (x-centreX)*cos+(y-centreY)*sin, -(x-centreX)*sin+(y-centreY)*cos
			return math.Max(math.Abs(along)-halfWidth, math.Abs(across)-halfHeight)
		}
	default:
		corners := randomPolygon(random, centreX, centreY, 10+random.Float64()*32)
		extent = 45
		distance = func(x, y float64) float64 { return polygonDistance(corners, x, y) }
	}
	reach := extent + feather + 1
	for y := int(centreY - reach); y <= int(centreY+reach); y++ {
		for x := int(centreX - reach); x <= int(centreX+reach); x++ {
			edge := distance(float64(x)+0.5, float64(y)+0.5)
			background.blend(x, y, paint, opacity*min(max(0.5-edge/feather, 0), 1))
		}
	}
}

// randomPolygon returns 3 to 6 corners around (x, y), at uneven angles and distances.
func randomPolygon(random *mrand.Rand, x, y, radius float64) [][2]float64 {
	count := 3 + random.IntN(4)
	start := random.Float64() * 2 * math.Pi
	corners := make([][2]float64, count)
	for i := range corners {
		angle := start + (float64(i)+random.Float64()*0.6)*2*math.Pi/float64(count)
		distance := radius * (0.55 + random.Float64()*0.45)
		corners[i] = [2]float64{x + distance*math.Cos(angle), y + distance*math.Sin(angle)}
	}
	return corners
}

// polygonDistance is the distance from (x, y) to the polygon's outline, negative inside.
func polygonDistance(corners [][2]float64, x, y float64) float64 {
	nearest, inside := math.Inf(1), false
	for i := range corners {
		a, b := corners[i], corners[(i+1)%len(corners)]
		edgeX, edgeY := b[0]-a[0], b[1]-a[1]
		share := min(max(((x-a[0])*edgeX+(y-a[1])*edgeY)/(edgeX*edgeX+edgeY*edgeY), 0), 1)
		nearest = min(nearest, math.Hypot(x-a[0]-share*edgeX, y-a[1]-share*edgeY))
		if (a[1] > y) != (b[1] > y) && x < a[0]+(y-a[1])*edgeX/edgeY {
			inside = !inside
		}
	}
	if inside {
		return -nearest
	}
	return nearest
}

// blur applies a 3 × 3 Gaussian blur, repeating the border pixels.
func (c *canvas) blur() {
	kernel := [3]float64{1, 2, 1}
	source := append([]float64(nil), c.pixels...)
	for y := range slideHeight {
		for x := range slideWidth {
			var sum [3]float64
			for ky := range 3 {
				for kx := range 3 {
					sampleX, sampleY := min(max(x+kx-1, 0), slideWidth-1), min(max(y+ky-1, 0), slideHeight-1)
					weight := kernel[kx] * kernel[ky] / 16
					for channel := range 3 {
						sum[channel] += source[(sampleY*slideWidth+sampleX)*3+channel] * weight
					}
				}
			}
			copy(c.pixels[(y*slideWidth+x)*3:], sum[:])
		}
	}
}

func randomPaint(random *mrand.Rand, darkest, lightest int) [3]float64 {
	paint := randomColor(random, darkest, lightest)
	return [3]float64{float64(paint.R), float64(paint.G), float64(paint.B)}
}
