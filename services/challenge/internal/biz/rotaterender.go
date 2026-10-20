package biz

import (
	"image"
	"image/color"
	"math"
	mrand "math/rand/v2"

	"golang.org/x/image/vector"
)

// A small software renderer for object_rotate: perspective, a depth buffer, light worked out at the
// corners of each face and blended across it (smooth across curved solids, flat on flat faces),
// drawn at rotateSupersample times the output size and averaged down for smooth edges. Lengths in
// the scene are in model heights: every model stands 1 tall.
const (
	rotateSupersample = 2
	rotateLookHeight  = 0.45 // the camera aims at this height of the model
	rotateDistance    = 4.6  // far enough for mild perspective
	rotateMargin      = 4    // display pixels kept clear at each edge of a panel
	rotateAmbient     = 0.5
	rotateDiffuse     = 0.6
	rotateBackdrop    = 0.8 // the backdrop is the panels' gradient this much as bright
	// The arrow in its panel, in model heights: across the middle of a disc of ground, from its tail
	// to its tip along its heading.
	rotateArrowDisc     = 0.6 // radius
	rotateArrowLength   = 1.15
	rotateArrowShaft    = 0.09 // half widths
	rotateArrowHead     = 0.25
	rotateArrowHeadLong = 0.38
	rotateArrowEdge     = 0.04
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

// discRadius is the soft disc of ground under the model, just wider than its base.
func discRadius(model *rotateModel) float64 { return max(model.base+0.08, 0.42) }

// discBounds is where the disc lands on the image plane.
func discBounds(camera rotateCamera, radius float64) rotateBounds {
	bounds := emptyBounds()
	for i := range 48 {
		sin, cos := math.Sincos(2 * math.Pi * float64(i) / 48)
		x, y, _ := camera.project(vec3{cos * radius, 0, sin * radius})
		bounds.add(x, y)
	}
	return bounds
}

// arrowShape is the arrow's outline on the ground along heading degrees, across the middle of its
// disc; outline grows it on every side for the light edge drawn under it.
func arrowShape(degrees float64, outline bool) []vec3 {
	edge := 0.0
	if outline {
		edge = rotateArrowEdge
	}
	orient := heading(degrees)
	at := func(along, across float64) vec3 { return orient.apply(vec3{along, 0, across}) }
	tail, tip := -rotateArrowLength/2, rotateArrowLength/2
	neck := tip - rotateArrowHeadLong
	shaft, head := rotateArrowShaft+edge, rotateArrowHead+edge*1.5
	return []vec3{
		at(tail-edge, -shaft), at(neck, -shaft), at(neck-edge*0.6, -head), at(tip+edge*1.8, 0),
		at(neck-edge*0.6, head), at(neck, shaft), at(tail-edge, shaft),
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

// fitRotate sizes and places a round's object in its panel: zoom such that the model fills
// rotateMinFill to rotateMaxFill of the panel's height in every frame, the model and its disc
// inside the margin. A model too wide for that fill is drawn smaller, down to rotateLeastFill;
// false when even that does not fit.
func fitRotate(random *mrand.Rand, camera rotateCamera, model *rotateModel, headings []float64) (rotateLayout, bool) {
	panel := rotateObjectPanel
	panelWidth, panelHeight := panel.x1-panel.x0, panel.y1-panel.y0
	all := discBounds(camera, discRadius(model))
	lowest, tallest := math.Inf(1), 0.0
	for _, h := range headings {
		b := modelBounds(camera, model, h)
		all = all.union(b)
		lowest, tallest = min(lowest, b.y1-b.y0), max(tallest, b.y1-b.y0)
	}
	width, height := all.x1-all.x0, all.y1-all.y0
	zoomHigh := min(rotateMaxFill*panelHeight/tallest, (panelWidth-2*rotateMargin)/width, (panelHeight-2*rotateMargin)/height)
	for fill := rotateMinFill; fill >= rotateLeastFill-1e-9; fill -= 0.05 {
		zoomLow := fill * panelHeight / lowest
		if zoomHigh < zoomLow {
			continue
		}
		zoom := between(random, zoomLow, zoomHigh)
		slackX := panelWidth - 2*rotateMargin - zoom*width
		slackY := panelHeight - 2*rotateMargin - zoom*height
		left := panel.x0 + rotateMargin + random.Float64()*slackX
		top := panel.y0 + rotateMargin + random.Float64()*slackY
		return rotateLayout{zoom: zoom, cx: left - zoom*all.x0, cy: top + zoom*all.y1}, true
	}
	return rotateLayout{}, false
}

// fitArrow centres the disc in the arrow's panel, sized so that the disc and the arrow fit it
// whichever of the eight directions, the round's and those a step apart, the arrow points: so every
// arrow of a camera is drawn alike, and only the perspective tells the directions apart. The camera
// is the object's, so that the arrow is foreshortened as the object's heading is.
func fitArrow(camera rotateCamera, degrees float64) rotateLayout {
	panel := rotateArrowPanel
	ox, oy, _ := camera.project(vec3{})
	reachX, reachY := 0.0, 0.0
	grow := func(b rotateBounds) {
		reachX = max(reachX, ox-b.x0, b.x1-ox)
		reachY = max(reachY, oy-b.y0, b.y1-oy)
	}
	grow(discBounds(camera, rotateArrowDisc))
	for k := range rotateFrames {
		grow(shapeBounds(camera, arrowShape(degrees+float64(k)*rotateStep, true)))
	}
	zoom := min((panel.x1-panel.x0-2*rotateMargin)/(2*reachX), (panel.y1-panel.y0-2*rotateMargin)/(2*reachY))
	return rotateLayout{zoom: zoom, cx: (panel.x0+panel.x1)/2 - zoom*ox, cy: (panel.y0+panel.y1)/2 + zoom*oy}
}

// rotateScene is what every frame of a round shares.
type rotateScene struct {
	camera      rotateCamera
	layout      rotateLayout // of the object's panel
	arrowLayout rotateLayout // of the arrow's panel
	scale       float64      // output pixels per display pixel
	light       vec3
	model       *rotateModel
	tint        [3]float64
	// base is the backdrop, both panels with their ground discs, and the arrow: everything but
	// the object and its shadow, the size of one output frame.
	base *image.RGBA
}

// panelPixels is a panel in output pixels, clipped to w × h.
func panelPixels(panel rotateBounds, scale float64, w, h int) image.Rectangle {
	return image.Rect(int(panel.x0*scale), int(panel.y0*scale), int(math.Ceil(panel.x1*scale)), int(math.Ceil(panel.y1*scale))).
		Intersect(image.Rect(0, 0, w, h))
}

// panelCover is how much of output pixel (x, y) the rounded panel covers, from 0 to 1: by the
// distance of the pixel's centre from the panel's edge, which is its distance from the panel shrunk
// by the radius on every side, less the radius.
func panelCover(panel rotateBounds, scale, x, y float64) float64 {
	radius := rotatePanelRadius * scale
	x0, y0, x1, y1 := panel.x0*scale+radius, panel.y0*scale+radius, panel.x1*scale-radius, panel.y1*scale-radius
	cx, cy := x+0.5, y+0.5
	dx, dy := max(x0-cx, 0, cx-x1), max(y0-cy, 0, cy-y1)
	switch {
	case dx == 0 && dy == 0:
		return 1
	case dx > radius+1 || dy > radius+1:
		return 0
	}
	return min(max(0.5-(math.Hypot(dx, dy)-radius), 0), 1)
}

// softColor is a light colour for the background.
func softColor(random *mrand.Rand) [3]float64 {
	return [3]float64{float64(185 + random.IntN(55)), float64(185 + random.IntN(55)), float64(185 + random.IntN(55))}
}

// drawBackground fills frame with the backdrop, then the two rounded panels on it: a soft
// two-colour gradient, a few faint blurred blobs and the soft disc of ground, in each panel seen
// through its own layout. Light noise covers everything.
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
	// Only pixels in a disc's box on screen look for the ground under them.
	type panel struct {
		rect   rotateBounds
		layout rotateLayout
		radius float64      // of the disc
		disc   rotateBounds // its box, in output pixels
	}
	panels := [2]panel{
		{rect: rotateArrowPanel, layout: s.arrowLayout, radius: rotateArrowDisc},
		{rect: rotateObjectPanel, layout: s.layout, radius: discRadius(s.model)},
	}
	for i := range panels {
		b := discBounds(s.camera, panels[i].radius)
		x0, y0 := panels[i].layout.pixel(b.x0, b.y1)
		x1, y1 := panels[i].layout.pixel(b.x1, b.y0)
		panels[i].disc = rotateBounds{x0 * s.scale, y0 * s.scale, x1 * s.scale, y1 * s.scale}
	}
	noise := random.Uint64() | 1
	for y := range h {
		row := s.base.Pix[y*s.base.Stride:]
		fy := float64(y)
		for x := range w {
			fx := float64(x)
			t := ((fx-float64(w)/2)*dx + (fy-float64(h)/2)*dy) / span
			t = min(max(t+0.5, 0), 1)
			var back [3]float64
			for c := range back {
				back[c] = from[c] + (to[c]-from[c])*t
			}
			px := back
			for c := range px {
				px[c] *= rotateBackdrop
			}
			for _, p := range panels {
				cover := panelCover(p.rect, s.scale, fx, fy)
				if cover == 0 {
					continue
				}
				inside := back
				for _, b := range blobs {
					ox, oy := fx-b.x, fy-b.y
					if d2 := (ox*ox + oy*oy) * b.inverse; d2 < 1 {
						k := (1 - d2) * (1 - d2) * b.alpha
						for c := range inside {
							inside[c] += b.shade * k
						}
					}
				}
				// The disc: darker ground, its edge fading over the outer fifth.
				if fy+1 >= p.disc.y0 && fy <= p.disc.y1 && fx+1 >= p.disc.x0 && fx <= p.disc.x1 {
					if g, ok := s.camera.ground(p.layout.plane(fx/s.scale, fy/s.scale)); ok {
						if d := g.flat() / p.radius; d < 1 {
							k := 0.13 * smooth(min((1-d)/0.22, 1))
							for c := range inside {
								inside[c] *= 1 - k
							}
						}
					}
				}
				for c := range px {
					px[c] += (inside[c] - px[c]) * cover
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

// drawArrow draws the arrow once for the round, on its disc in its panel: a light outline, then
// the arrow in a strong colour.
func (s *rotateScene) drawArrow(degrees float64, paint color.RGBA) {
	bounds := s.base.Bounds()
	raster := vector.NewRasterizer(bounds.Dx(), bounds.Dy())
	fill := func(points []vec3, c color.Color) {
		raster.Reset(bounds.Dx(), bounds.Dy())
		for i, p := range points {
			x, y, _ := s.camera.project(p)
			px, py := s.arrowLayout.pixel(x, y)
			if i == 0 {
				raster.MoveTo(float32(px*s.scale), float32(py*s.scale))
			} else {
				raster.LineTo(float32(px*s.scale), float32(py*s.scale))
			}
		}
		raster.ClosePath()
		raster.Draw(s.base, bounds, image.NewUniform(c), image.Point{})
	}
	fill(arrowShape(degrees, true), color.RGBA{250, 250, 246, 255})
	fill(arrowShape(degrees, false), paint)
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

// triangle fills one face: corners in supersampled pixels with 1/depth, and the colour at each
// corner, blended across the face.
func (r *rotateRaster) triangle(p [3][3]float64, paints [3][3]float64) {
	area := (p[1][0]-p[0][0])*(p[2][1]-p[0][1]) - (p[1][1]-p[0][1])*(p[2][0]-p[0][0])
	if math.Abs(area) < 1e-9 {
		return
	}
	if area < 0 {
		p[1], p[2], area = p[2], p[1], -area
		paints[1], paints[2] = paints[2], paints[1]
	}
	flat := paints[0] == paints[1] && paints[1] == paints[2]
	uniform := packPaint(paints[0])
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
	// The colour is linear across the face too: at a row's start from the weights, then stepped.
	blend := func(w0, w1, w2 float64, k int) float64 {
		return (w0*paints[0][k] + w1*paints[1][k] + w2*paints[2][k]) * inv
	}
	step := [3]float64{blend(stepX[0], stepX[1], stepX[2], 0), blend(stepX[0], stepX[1], stepX[2], 1), blend(stepX[0], stepX[1], stepX[2], 2)}
	for y := minY; y <= maxY; y++ {
		py, px := float64(y)+0.5, float64(minX)+0.5
		w0, w1, w2 := edge(p[1], p[2], px, py), edge(p[2], p[0], px, py), edge(p[0], p[1], px, py)
		c := [3]float64{blend(w0, w1, w2, 0), blend(w0, w1, w2, 1), blend(w0, w1, w2, 2)}
		row := (y - r.y0) * r.w
		for x := minX; x <= maxX; x++ {
			if w0 >= 0 && w1 >= 0 && w2 >= 0 {
				z := float32((w0*p[0][2] + w1*p[1][2] + w2*p[2][2]) * inv)
				i := row + x - r.x0
				if z > r.depth[i] {
					paint := uniform
					if !flat {
						// Inside the face the blend lies between the corners' colours, never below 0.
						paint = uint32(min(c[0], 255)+0.5) | uint32(min(c[1], 255)+0.5)<<8 | uint32(min(c[2], 255)+0.5)<<16
					}
					r.depth[i], r.paint[i] = z, paint
				}
			}
			w0, w1, w2 = w0+stepX[0], w1+stepX[1], w2+stepX[2]
			c[0], c[1], c[2] = c[0]+step[0], c[1]+step[1], c[2]+step[2]
		}
	}
}

// drawFrame draws the model at degrees into dst, one output frame (at the origin) already holding
// the background: its shadow, then the model.
func (s *rotateScene) drawFrame(dst *image.RGBA, raster *rotateRaster, degrees float64) {
	s.drawShadow(dst, heading(degrees))
	if s.rasterize(raster, degrees, dst.Bounds().Dx(), dst.Bounds().Dy()) {
		s.composite(dst, raster)
	}
}

// rasterize draws the model at degrees into raster, over the part of the object's panel in a w × h
// output frame that it covers; false when none of it is in the panel.
func (s *rotateScene) rasterize(raster *rotateRaster, degrees float64, w, h int) bool {
	orient := heading(degrees)
	ss := s.scale * rotateSupersample
	bounds := modelBounds(s.camera, s.model, degrees)
	px0, py0 := s.layout.pixel(bounds.x0, bounds.y1)
	px1, py1 := s.layout.pixel(bounds.x1, bounds.y0)
	// The region in output pixels, then in supersampled ones.
	panel := panelPixels(rotateObjectPanel, s.scale, w, h)
	ox0, oy0 := max(int(px0*s.scale)-1, panel.Min.X), max(int(py0*s.scale)-1, panel.Min.Y)
	ox1, oy1 := min(int(px1*s.scale)+2, panel.Max.X), min(int(py1*s.scale)+2, panel.Max.Y)
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
		var paints [3][3]float64
		for i, n := range t.normals {
			light := rotateAmbient + rotateDiffuse*max(0, orient.apply(n).dot(s.light))
			paints[i] = shade(t.paint, s.tint, light)
		}
		raster.triangle(corners, paints)
	}
	return true
}

// shade is paint, tinted, in the given light.
func shade(paint color.RGBA, tint [3]float64, light float64) [3]float64 {
	return [3]float64{float64(paint.R) * tint[0] * light, float64(paint.G) * tint[1] * light, float64(paint.B) * tint[2] * light}
}

func packPaint(c [3]float64) uint32 {
	return uint32(clampByte(c[0])) | uint32(clampByte(c[1]))<<8 | uint32(clampByte(c[2]))<<16
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
	panel := panelPixels(rotateObjectPanel, s.scale, dst.Bounds().Dx(), dst.Bounds().Dy())
	x0, y0 := max(int(area.x0), panel.Min.X), max(int(area.y0), panel.Min.Y)
	x1, y1 := min(int(area.x1)+1, panel.Max.X), min(int(area.y1)+1, panel.Max.Y)
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
