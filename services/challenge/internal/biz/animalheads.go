package biz

import (
	"image"
	"image/color"
	"math"
	mrand "math/rand/v2"

	"golang.org/x/image/vector"
)

// The heads of animal_jump are drawn from ellipses, triangles and strokes, never from image files,
// so there is no picture to look up. Shapes are given in units of the face's radius around the
// face's centre; every species keeps within the face's width, so a head is exactly its diameter
// wide and neighbouring heads cannot reach into each other's hit box.
const (
	headOutlinePx = 1.1 // CSS px around the outer parts
	kappa         = 0.5522847498
)

// shape traces itself into a rasterizer; grow widens it by that many units of the face's radius.
type shape interface {
	trace(z *vector.Rasterizer, at func(x, y float64) [2]float32, grow float64)
	extent() (minX, minY, maxX, maxY float64)
}

type ellipse struct{ x, y, rx, ry, angle float64 }

func (e ellipse) trace(z *vector.Rasterizer, at func(x, y float64) [2]float32, grow float64) {
	rx, ry := e.rx+grow, e.ry+grow
	sin, cos := math.Sincos(e.angle)
	point := func(u, v float64) [2]float32 { return at(e.x+u*cos-v*sin, e.y+u*sin+v*cos) }
	moveTo(z, point(rx, 0))
	cubeTo(z, point(rx, kappa*ry), point(kappa*rx, ry), point(0, ry))
	cubeTo(z, point(-kappa*rx, ry), point(-rx, kappa*ry), point(-rx, 0))
	cubeTo(z, point(-rx, -kappa*ry), point(-kappa*rx, -ry), point(0, -ry))
	cubeTo(z, point(kappa*rx, -ry), point(rx, -kappa*ry), point(rx, 0))
	z.ClosePath()
}

func (e ellipse) extent() (float64, float64, float64, float64) {
	// Exact for an ellipse at any angle.
	sin, cos := math.Sincos(e.angle)
	halfX := math.Hypot(e.rx*cos, e.ry*sin)
	halfY := math.Hypot(e.rx*sin, e.ry*cos)
	return e.x - halfX, e.y - halfY, e.x + halfX, e.y + halfY
}

// polygon grows by moving each corner away from the centre of its corners.
type polygon [][2]float64

func (p polygon) trace(z *vector.Rasterizer, at func(x, y float64) [2]float32, grow float64) {
	var midX, midY float64
	for _, corner := range p {
		midX, midY = midX+corner[0]/float64(len(p)), midY+corner[1]/float64(len(p))
	}
	for i, corner := range p {
		x, y := corner[0], corner[1]
		if distance := math.Hypot(x-midX, y-midY); grow > 0 && distance > 0 {
			x, y = x+(x-midX)/distance*grow*1.6, y+(y-midY)/distance*grow*1.6
		}
		if i == 0 {
			moveTo(z, at(x, y))
		} else {
			lineTo(z, at(x, y))
		}
	}
	z.ClosePath()
}

func (p polygon) extent() (float64, float64, float64, float64) {
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, corner := range p {
		minX, minY = min(minX, corner[0]), min(minY, corner[1])
		maxX, maxY = max(maxX, corner[0]), max(maxY, corner[1])
	}
	return minX, minY, maxX, maxY
}

// stroke is a line of the given width between two points, e.g. a whisker.
func stroke(x1, y1, x2, y2, width float64) polygon {
	length := math.Hypot(x2-x1, y2-y1)
	nx, ny := -(y2-y1)/length*width/2, (x2-x1)/length*width/2
	return polygon{{x1 + nx, y1 + ny}, {x2 + nx, y2 + ny}, {x2 - nx, y2 - ny}, {x1 - nx, y1 - ny}}
}

// arc is a stroke along part of an ellipse, from and to in radians (0 is right, π/2 down).
func arc(x, y, rx, ry, from, to, width float64) polygon {
	const steps = 10
	var outer, inner polygon
	for i := range steps + 1 {
		angle := from + (to-from)*float64(i)/steps
		sin, cos := math.Sincos(angle)
		outer = append(outer, [2]float64{x + (rx+width/2)*cos, y + (ry+width/2)*sin})
		inner = append(inner, [2]float64{x + (rx-width/2)*cos, y + (ry-width/2)*sin})
	}
	for i := len(inner) - 1; i >= 0; i-- {
		outer = append(outer, inner[i])
	}
	return outer
}

func moveTo(z *vector.Rasterizer, p [2]float32) { z.MoveTo(p[0], p[1]) }
func lineTo(z *vector.Rasterizer, p [2]float32) { z.LineTo(p[0], p[1]) }
func cubeTo(z *vector.Rasterizer, b, c, d [2]float32) {
	z.CubeTo(b[0], b[1], c[0], c[1], d[0], d[1])
}

// part is one filled shape of a head; the outer ones get the dark outline, drawn just before them,
// so a part in front shows its edge over the one behind.
type part struct {
	paint color.NRGBA
	shape shape
	outer bool
}

// mirrored returns the shape build makes for the left side (-1) and the right side (+1).
func mirrored(paint color.NRGBA, outer bool, build func(side float64) shape) []part {
	return []part{{paint, build(-1), outer}, {paint, build(1), outer}}
}

var (
	outlinePaint = color.NRGBA{56, 40, 36, 255}
	eyePaint     = color.NRGBA{30, 24, 24, 255}
	whitePaint   = color.NRGBA{255, 255, 255, 255}
	innerPink    = color.NRGBA{240, 152, 166, 255}
)

// species is one kind of animal: its parts for a tint of its colours.
type species struct {
	name  string
	parts func(tint func(color.NRGBA) color.NRGBA) []part
}

// tintOf shifts each channel of a species' colours by up to ±6%, the same way for every part of
// one head, so no two heads of a kind are exactly alike.
func tintOf(random *mrand.Rand) func(color.NRGBA) color.NRGBA {
	shift := [3]float64{}
	for i := range shift {
		shift[i] = 1 + (random.Float64()*2-1)*0.06
	}
	return func(paint color.NRGBA) color.NRGBA {
		return color.NRGBA{clampByte(float64(paint.R) * shift[0]), clampByte(float64(paint.G) * shift[1]), clampByte(float64(paint.B) * shift[2]), 255}
	}
}

// partsExtent is the box around parts, in units of the face's radius.
func partsExtent(parts []part) (minX, minY, maxX, maxY float64) {
	minX, minY, maxX, maxY = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range parts {
		x0, y0, x1, y1 := p.shape.extent()
		minX, minY, maxX, maxY = min(minX, x0), min(minY, y0), max(maxX, x1), max(maxY, y1)
	}
	return minX, minY, maxX, maxY
}

// drawParts paints parts into dst, which covers the CSS box at (originX, originY) at scale device
// pixels per CSS pixel. place maps a point in units of the face's radius to CSS pixels.
func drawParts(dst *image.RGBA, raster *vector.Rasterizer, parts []part, radius, scale, originX, originY float64,
	place func(x, y float64) (float64, float64)) {
	size := dst.Bounds().Size()
	at := func(x, y float64) [2]float32 {
		cssX, cssY := place(x, y)
		return [2]float32{float32((cssX - originX) * scale), float32((cssY - originY) * scale)}
	}
	fill := func(paint color.NRGBA, s shape, grow float64) {
		raster.Reset(size.X, size.Y)
		s.trace(raster, at, grow)
		raster.Draw(dst, dst.Bounds(), image.NewUniform(paint), image.Point{})
	}
	for _, p := range parts {
		if p.outer {
			fill(outlinePaint, p.shape, headOutlinePx/radius)
		}
		fill(p.paint, p.shape, 0)
	}
}
