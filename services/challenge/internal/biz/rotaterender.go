package biz

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	mrand "math/rand/v2"

	"golang.org/x/image/vector"
)

// A small software renderer for object_rotate: perspective, a depth buffer, flat shading per face,
// drawn at rotateSupersample times the output size and averaged down for smooth edges. Lengths in
// the scene are in model heights: every model stands 1 tall.
const (
	rotateSupersample = 2
	rotateLookHeight  = 0.45 // the camera aims at this height of the model
	rotateDistance    = 4.6  // far enough for mild perspective
	rotateMargin      = 4    // display pixels kept clear at each edge of a frame
	rotateAmbient     = 0.5
	rotateDiffuse     = 0.6
	// The arrow, sized for a model 1 tall (arrowSize): from the edge of the ground disc outward, as
	// long as this many model heights, cut down to rotateArrowShortest of that where it would not
	// fit, mostly when it points at the camera.
	rotateArrowLength   = 0.62
	rotateArrowShortest = 0.3
	rotateArrowShaft    = 0.075 // half widths
	rotateArrowHead     = 0.19
	rotateArrowHeadLong = 0.24 // the head keeps its size on a short arrow, so it still reads as one
	rotateArrowEdge     = 0.035
	rotateBaseHeight    = 0.25 // see rotateModel.base
)

// rotateCamera looks at the model from above the ground; scene points project onto the image plane
// at distance 1, x to the right and y up.
type rotateCamera struct{ eye, forward, right, up vec3 }

// newRotateCamera sits elevation degrees above the ground, turned about the vertical by azimuth:
// at 0 it looks along -Z, so a model at heading 0 faces right, and at 90 it faces the camera.
func newRotateCamera(elevation, azimuth float64) rotateCamera {
	se, ce := math.Sincos(radians(elevation))
	sa, ca := math.Sincos(radians(azimuth))
	back := vec3{sa * ce, se, ca * ce}
	forward := back.scale(-1)
	right := forward.cross(vec3{0, 1, 0}).unit()
	return rotateCamera{
		eye:     vec3{0, rotateLookHeight, 0}.add(back.scale(rotateDistance)),
		forward: forward, right: right, up: right.cross(forward),
	}
}

func (c rotateCamera) project(p vec3) (x, y, depth float64) {
	d := p.sub(c.eye)
	depth = d.dot(c.forward)
	return d.dot(c.right) / depth, d.dot(c.up) / depth, depth
}

// ground is the point of y = 0 seen at image-plane point (x, y); false above the horizon.
func (c rotateCamera) ground(x, y float64) (vec3, bool) {
	ray := c.forward.add(c.right.scale(x)).add(c.up.scale(y))
	if ray.y >= -1e-9 {
		return vec3{}, false
	}
	return c.eye.add(ray.scale(-c.eye.y / ray.y)), true
}

// rotateBounds is a box on the image plane.
type rotateBounds struct{ x0, y0, x1, y1 float64 }

func emptyBounds() rotateBounds {
	return rotateBounds{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
}

func (b *rotateBounds) add(x, y float64) {
	b.x0, b.y0, b.x1, b.y1 = min(b.x0, x), min(b.y0, y), max(b.x1, x), max(b.y1, y)
}

func (b rotateBounds) union(o rotateBounds) rotateBounds {
	return rotateBounds{min(b.x0, o.x0), min(b.y0, o.y0), max(b.x1, o.x1), max(b.y1, o.y1)}
}

// rotateLayout puts the image plane on a frame: display pixel (cx + zoom·x, cy − zoom·y).
type rotateLayout struct{ zoom, cx, cy float64 }

func (l rotateLayout) pixel(x, y float64) (float64, float64) { return l.cx + l.zoom*x, l.cy - l.zoom*y }
func (l rotateLayout) plane(px, py float64) (float64, float64) {
	return (px - l.cx) / l.zoom, (l.cy - py) / l.zoom
}

// modelBounds is where model, turned to heading, lands on the image plane.
func modelBounds(camera rotateCamera, model *rotateModel, degrees float64) rotateBounds {
	orient := heading(degrees)
	bounds := emptyBounds()
	for _, p := range model.points {
		x, y, _ := camera.project(orient.apply(p))
		bounds.add(x, y)
	}
	return bounds
}

// discRadius is the soft disc of ground under the model, just wider than its base, so that the
// arrow starts clear of the model.
func discRadius(model *rotateModel) float64 { return max(model.base+0.08, 0.42) }

// arrowSize scales the arrow to the model: beside a long, low model one sized in model heights
// would look small.
func arrowSize(model *rotateModel) float64 { return min(max(model.base/0.7, 1), 1.4) }

// arrowShape is the arrow's outline on the ground along heading degrees, from the edge of the
// model's disc outward and share of its full length; outline grows it on every side for the light
// edge drawn under it.
func arrowShape(model *rotateModel, degrees, share float64, outline bool) []vec3 {
	size := arrowSize(model)
	edge := 0.0
	if outline {
		edge = rotateArrowEdge * size
	}
	orient := heading(degrees)
	at := func(along, across float64) vec3 { return orient.apply(vec3{along, 0, across}) }
	start, length := discRadius(model), rotateArrowLength*size*share
	tip := start + length
	neck := tip - min(rotateArrowHeadLong*size, length*0.8)
	shaft, head := rotateArrowShaft*size+edge, rotateArrowHead*size+edge*1.5
	return []vec3{
		at(start-edge, -shaft), at(neck, -shaft), at(neck-edge*0.6, -head), at(tip+edge*1.8, 0),
		at(neck-edge*0.6, head), at(neck, shaft), at(start-edge, shaft),
	}
}

func shapeBounds(camera rotateCamera, points []vec3) rotateBounds {
	bounds := emptyBounds()
	for _, p := range points {
		x, y, _ := camera.project(p)
		bounds.add(x, y)
	}
	return bounds
}

// fitRotate sizes and places a round: zoom such that the model fills rotateMinFill to rotateMaxFill
// of the frame's height in every frame, the model and the arrow inside the margin. The arrow is
// shortened when it would not fit; false when nothing fits.
func fitRotate(random *mrand.Rand, camera rotateCamera, model *rotateModel, headings []float64, arrow float64) (rotateLayout, float64, bool) {
	var all rotateBounds = emptyBounds()
	lowest, tallest := math.Inf(1), 0.0
	for _, h := range headings {
		b := modelBounds(camera, model, h)
		all = all.union(b)
		lowest, tallest = min(lowest, b.y1-b.y0), max(tallest, b.y1-b.y0)
	}
	for step := 0.0; 1-step*0.05 >= rotateArrowShortest-1e-9; step++ {
		share := 1 - step*0.05
		union := all.union(shapeBounds(camera, arrowShape(model, arrow, share, true)))
		width, height := union.x1-union.x0, union.y1-union.y0
		zoomLow := rotateMinFill * rotateFrameHeight / lowest
		zoomHigh := min(rotateMaxFill*rotateFrameHeight/tallest, (rotateFrameWidth-2*rotateMargin)/width, (rotateFrameHeight-2*rotateMargin)/height)
		if zoomHigh < zoomLow {
			continue
		}
		zoom := between(random, zoomLow, zoomHigh)
		slackX := rotateFrameWidth - 2*rotateMargin - zoom*width
		slackY := rotateFrameHeight - 2*rotateMargin - zoom*height
		left := rotateMargin + random.Float64()*slackX
		top := rotateMargin + random.Float64()*slackY
		return rotateLayout{zoom: zoom, cx: left - zoom*union.x0, cy: top + zoom*union.y1}, share, true
	}
	return rotateLayout{}, 0, false
}

// rotateScene is what every frame of a round shares.
type rotateScene struct {
	camera rotateCamera
	layout rotateLayout
	scale  float64 // output pixels per display pixel
	light  vec3
	model  *rotateModel
	tint   [3]float64
	// base is the background with the ground disc, the size of one output frame; arrow is drawn
	// over everything, in arrowRect.
	base      *image.RGBA
	arrow     *image.RGBA
	arrowRect image.Rectangle
}

// softColor is a light colour for the background.
func softColor(random *mrand.Rand) [3]float64 {
	return [3]float64{float64(185 + random.IntN(55)), float64(185 + random.IntN(55)), float64(185 + random.IntN(55))}
}

// drawBackground fills frame with a soft two-colour gradient, a few faint blurred blobs and light
// noise, then the soft disc of ground under the model.
func (s *rotateScene) drawBackground(random *mrand.Rand) {
	bounds := s.base.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	from, to := softColor(random), softColor(random)
	angle := random.Float64() * 2 * math.Pi
	dx, dy := math.Cos(angle), math.Sin(angle)
	span := math.Abs(dx)*float64(w) + math.Abs(dy)*float64(h)
	// A blob shades by (1 - d²)² of its alpha at d radii from its centre: squared distances only.
	type blob struct{ x, y, inverse, shade, alpha float64 }
	blobs := make([]blob, 3+random.IntN(3))
	for i := range blobs {
		r := (25 + random.Float64()*70) * s.scale
		blobs[i] = blob{
			x: random.Float64() * float64(w), y: random.Float64() * float64(h), inverse: 1 / (r * r),
			shade: (random.Float64()*2 - 1) * 26, alpha: 0.35 + random.Float64()*0.3,
		}
	}
	// Only pixels in the disc's box on screen look for the ground under them.
	disc := discRadius(s.model)
	discBox := emptyBounds()
	for i := range 48 {
		sin, cos := math.Sincos(2 * math.Pi * float64(i) / 48)
		x, y, _ := s.camera.project(vec3{cos * disc, 0, sin * disc})
		px, py := s.layout.pixel(x, y)
		discBox.add(px*s.scale, py*s.scale)
	}
	noise := random.Uint64() | 1
	for y := range h {
		row := s.base.Pix[y*s.base.Stride:]
		fy := float64(y)
		inDiscRows := fy+1 >= discBox.y0 && fy <= discBox.y1
		for x := range w {
			fx := float64(x)
			t := ((fx-float64(w)/2)*dx + (fy-float64(h)/2)*dy) / span
			t = min(max(t+0.5, 0), 1)
			var px [3]float64
			for c := range px {
				px[c] = from[c] + (to[c]-from[c])*t
			}
			for _, b := range blobs {
				ox, oy := fx-b.x, fy-b.y
				if d2 := (ox*ox + oy*oy) * b.inverse; d2 < 1 {
					k := (1 - d2) * (1 - d2) * b.alpha
					for c := range px {
						px[c] += b.shade * k
					}
				}
			}
			// The disc: darker ground, its edge fading over the outer fifth.
			if inDiscRows && fx+1 >= discBox.x0 && fx <= discBox.x1 {
				if g, ok := s.camera.ground(s.layout.plane(fx/s.scale, fy/s.scale)); ok {
					if d := g.flat() / disc; d < 1 {
						k := 0.13 * smooth(min((1-d)/0.22, 1))
						for c := range px {
							px[c] *= 1 - k
						}
					}
				}
			}
			noise ^= noise << 13
			noise ^= noise >> 7
			noise ^= noise << 17
			grain := float64(int(noise%7)) - 3
			for c := range px {
				row[x*4+c] = clampByte(px[c] + grain)
			}
			row[x*4+3] = 255
		}
	}
}

func smooth(t float64) float64 { return t * t * (3 - 2*t) }

// drawArrow draws the arrow once for the round: a light outline, then the arrow in a strong colour.
func (s *rotateScene) drawArrow(degrees, share float64, paint color.RGBA) {
	bounds := s.base.Bounds()
	s.arrow = image.NewRGBA(bounds)
	raster := vector.NewRasterizer(bounds.Dx(), bounds.Dy())
	fill := func(points []vec3, c color.Color) {
		raster.Reset(bounds.Dx(), bounds.Dy())
		for i, p := range points {
			x, y, _ := s.camera.project(p)
			px, py := s.layout.pixel(x, y)
			if i == 0 {
				raster.MoveTo(float32(px*s.scale), float32(py*s.scale))
			} else {
				raster.LineTo(float32(px*s.scale), float32(py*s.scale))
			}
		}
		raster.ClosePath()
		raster.Draw(s.arrow, bounds, image.NewUniform(c), image.Point{})
	}
	fill(arrowShape(s.model, degrees, share, true), color.RGBA{250, 250, 246, 255})
	fill(arrowShape(s.model, degrees, share, false), paint)
	s.arrowRect = image.Rectangle{}
	for y := range bounds.Dy() {
		for x := range bounds.Dx() {
			if s.arrow.Pix[y*s.arrow.Stride+x*4+3] != 0 {
				s.arrowRect = s.arrowRect.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
}

// arrowColors are strong colours to point with; the one farthest from the model's main colour is used.
var arrowColors = []color.RGBA{{224, 40, 48, 255}, {240, 116, 0, 255}, {214, 30, 160, 255}, {28, 96, 232, 255}, {0, 150, 136, 255}, {120, 56, 220, 255}}

func arrowColor(random *mrand.Rand, model *rotateModel) color.RGBA {
	distance := func(c color.RGBA) float64 {
		dr, dg, db := float64(c.R)-float64(model.main.R), float64(c.G)-float64(model.main.G), float64(c.B)-float64(model.main.B)
		return dr*dr + dg*dg + db*db
	}
	// The two farthest, at random, so the arrow is not always the same colour for one model.
	order := append([]color.RGBA(nil), arrowColors...)
	for i := range order {
		for j := i + 1; j < len(order); j++ {
			if distance(order[j]) > distance(order[i]) {
				order[i], order[j] = order[j], order[i]
			}
		}
	}
	return order[random.IntN(2)]
}

// rotateRaster holds one frame's model at supersampled resolution: 1/depth (0 where empty) and colour.
type rotateRaster struct {
	x0, y0, w, h int
	depth        []float32
	paint        []uint32
}

func (r *rotateRaster) reset(x0, y0, w, h int) {
	r.x0, r.y0, r.w, r.h = x0, y0, w, h
	if cap(r.depth) < w*h {
		r.depth, r.paint = make([]float32, w*h), make([]uint32, w*h)
	}
	r.depth, r.paint = r.depth[:w*h], r.paint[:w*h]
	clear(r.depth)
}

// triangle fills one face: corners in supersampled pixels with 1/depth, flat colour.
func (r *rotateRaster) triangle(p [3][3]float64, paint uint32) {
	area := (p[1][0]-p[0][0])*(p[2][1]-p[0][1]) - (p[1][1]-p[0][1])*(p[2][0]-p[0][0])
	if math.Abs(area) < 1e-9 {
		return
	}
	if area < 0 {
		p[1], p[2], area = p[2], p[1], -area
	}
	minX := max(int(math.Floor(min(p[0][0], p[1][0], p[2][0]))), r.x0)
	maxX := min(int(math.Ceil(max(p[0][0], p[1][0], p[2][0]))), r.x0+r.w-1)
	minY := max(int(math.Floor(min(p[0][1], p[1][1], p[2][1]))), r.y0)
	maxY := min(int(math.Ceil(max(p[0][1], p[1][1], p[2][1]))), r.y0+r.h-1)
	if minX > maxX || minY > maxY {
		return
	}
	// Edge functions, each the weight of the corner opposite its edge, stepped along a row.
	edge := func(a, b [3]float64, x, y float64) float64 { return (b[0]-a[0])*(y-a[1]) - (b[1]-a[1])*(x-a[0]) }
	stepX := [3]float64{-(p[2][1] - p[1][1]), -(p[0][1] - p[2][1]), -(p[1][1] - p[0][1])}
	inv := 1 / area
	for y := minY; y <= maxY; y++ {
		py, px := float64(y)+0.5, float64(minX)+0.5
		w0, w1, w2 := edge(p[1], p[2], px, py), edge(p[2], p[0], px, py), edge(p[0], p[1], px, py)
		row := (y - r.y0) * r.w
		for x := minX; x <= maxX; x++ {
			if w0 >= 0 && w1 >= 0 && w2 >= 0 {
				z := float32((w0*p[0][2] + w1*p[1][2] + w2*p[2][2]) * inv)
				i := row + x - r.x0
				if z > r.depth[i] {
					r.depth[i], r.paint[i] = z, paint
				}
			}
			w0, w1, w2 = w0+stepX[0], w1+stepX[1], w2+stepX[2]
		}
	}
}

// drawFrame draws the model at degrees into dst, one output frame (at the origin) already holding
// the background: its shadow, then the model, then the arrow.
func (s *rotateScene) drawFrame(dst *image.RGBA, raster *rotateRaster, degrees float64) {
	s.drawShadow(dst, heading(degrees))
	if s.rasterize(raster, degrees, dst.Bounds().Dx(), dst.Bounds().Dy()) {
		s.composite(dst, raster)
	}
	if !s.arrowRect.Empty() {
		draw.Draw(dst, s.arrowRect, s.arrow, s.arrowRect.Min, draw.Over)
	}
}

// rasterize draws the model at degrees into raster, over the part of a w × h output frame that it
// covers; false when none of it is in the frame.
func (s *rotateScene) rasterize(raster *rotateRaster, degrees float64, w, h int) bool {
	orient := heading(degrees)
	ss := s.scale * rotateSupersample
	bounds := modelBounds(s.camera, s.model, degrees)
	px0, py0 := s.layout.pixel(bounds.x0, bounds.y1)
	px1, py1 := s.layout.pixel(bounds.x1, bounds.y0)
	// The region in output pixels, then in supersampled ones.
	ox0, oy0 := max(int(px0*s.scale)-1, 0), max(int(py0*s.scale)-1, 0)
	ox1, oy1 := min(int(px1*s.scale)+2, w), min(int(py1*s.scale)+2, h)
	if ox0 >= ox1 || oy0 >= oy1 {
		return false
	}
	raster.reset(ox0*rotateSupersample, oy0*rotateSupersample, (ox1-ox0)*rotateSupersample, (oy1-oy0)*rotateSupersample)
	toEye := s.camera.eye
	for _, t := range s.model.tris {
		normal := orient.apply(t.normal)
		a := orient.apply(t.a)
		// A face turned away from the camera is hidden behind the front of its convex solid.
		if normal.dot(a.sub(toEye)) >= 0 {
			continue
		}
		b, c := orient.apply(t.b), orient.apply(t.c)
		var corners [3][3]float64
		for i, p := range [3]vec3{a, b, c} {
			x, y, depth := s.camera.project(p)
			sx, sy := s.layout.pixel(x, y)
			corners[i] = [3]float64{sx * ss, sy * ss, 1 / depth}
		}
		light := rotateAmbient + rotateDiffuse*max(0, normal.dot(s.light))
		raster.triangle(corners, packColor(t.paint, s.tint, light))
	}
	return true
}

func packColor(paint color.RGBA, tint [3]float64, light float64) uint32 {
	r := clampByte(float64(paint.R) * tint[0] * light)
	g := clampByte(float64(paint.G) * tint[1] * light)
	b := clampByte(float64(paint.B) * tint[2] * light)
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16
}

// composite averages each block of supersamples into one output pixel over what dst holds.
func (s *rotateScene) composite(dst *image.RGBA, raster *rotateRaster) {
	const n = rotateSupersample * rotateSupersample
	for oy := raster.y0 / rotateSupersample; oy < (raster.y0+raster.h)/rotateSupersample; oy++ {
		for ox := raster.x0 / rotateSupersample; ox < (raster.x0+raster.w)/rotateSupersample; ox++ {
			var covered int
			var r, g, b uint32
			for sy := range rotateSupersample {
				row := (oy*rotateSupersample + sy - raster.y0) * raster.w
				for sx := range rotateSupersample {
					i := row + ox*rotateSupersample + sx - raster.x0
					if raster.depth[i] > 0 {
						p := raster.paint[i]
						r, g, b, covered = r+p&0xff, g+p>>8&0xff, b+p>>16&0xff, covered+1
					}
				}
			}
			if covered == 0 {
				continue
			}
			i := dst.PixOffset(ox, oy)
			keep := uint32(n - covered)
			dst.Pix[i] = uint8((r + uint32(dst.Pix[i])*keep) / n)
			dst.Pix[i+1] = uint8((g + uint32(dst.Pix[i+1])*keep) / n)
			dst.Pix[i+2] = uint8((b + uint32(dst.Pix[i+2])*keep) / n)
		}
	}
}

// drawShadow darkens a soft ellipse of ground under the model's footprint, nudged away from the light.
func (s *rotateScene) drawShadow(dst *image.RGBA, orient turn) {
	m := s.model
	length, width := (m.front+m.back)/2*0.95, m.side*0.95+0.05
	centre := orient.apply(vec3{(m.front - m.back) / 2, 0, 0}).sub(vec3{s.light.x, 0, s.light.z}.scale(0.08))
	along := orient.apply(vec3{1, 0, 0})
	across := orient.apply(vec3{0, 0, 1})
	reach := max(length, width) * 1.3
	area := emptyBounds()
	for _, corner := range [4]vec3{{reach, 0, reach}, {reach, 0, -reach}, {-reach, 0, reach}, {-reach, 0, -reach}} {
		x, y, _ := s.camera.project(centre.add(corner))
		px, py := s.layout.pixel(x, y)
		area.add(px*s.scale, py*s.scale)
	}
	x0, y0 := max(int(area.x0), 0), max(int(area.y0), 0)
	x1, y1 := min(int(area.x1)+1, dst.Bounds().Dx()), min(int(area.y1)+1, dst.Bounds().Dy())
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			g, ok := s.camera.ground(s.layout.plane(float64(x)/s.scale, float64(y)/s.scale))
			if !ok {
				continue
			}
			d := g.sub(centre)
			u, v := d.dot(along)/length, d.dot(across)/width
			r := u*u + v*v
			if r >= 1 {
				continue
			}
			k := 0.3 * smooth(1-r)
			i := dst.PixOffset(x, y)
			for c := range 3 {
				dst.Pix[i+c] = uint8(float64(dst.Pix[i+c]) * (1 - k))
			}
		}
	}
}
