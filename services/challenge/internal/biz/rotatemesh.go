package biz

import (
	"image/color"
	"math"
)

// vec3 is a point or a direction. In a model's own space +X is its front, +Y up and +Z its right,
// with the ground at y = 0.
type vec3 struct{ x, y, z float64 }

func (a vec3) add(b vec3) vec3      { return vec3{a.x + b.x, a.y + b.y, a.z + b.z} }
func (a vec3) sub(b vec3) vec3      { return vec3{a.x - b.x, a.y - b.y, a.z - b.z} }
func (a vec3) scale(s float64) vec3 { return vec3{a.x * s, a.y * s, a.z * s} }
func (a vec3) times(b vec3) vec3    { return vec3{a.x * b.x, a.y * b.y, a.z * b.z} }
func (a vec3) dot(b vec3) float64   { return a.x*b.x + a.y*b.y + a.z*b.z }
func (a vec3) cross(b vec3) vec3 {
	return vec3{a.y*b.z - a.z*b.y, a.z*b.x - a.x*b.z, a.x*b.y - a.y*b.x}
}
func (a vec3) length() float64        { return math.Sqrt(a.dot(a)) }
func (a vec3) unit() vec3             { return a.scale(1 / a.length()) }
func (a vec3) flat() float64          { return math.Hypot(a.x, a.z) }
func lerp3(a, b vec3, t float64) vec3 { return a.add(b.sub(a).scale(t)) }

// turn is a rotation, as the matrix that applies it to a column vector.
type turn [3][3]float64

var noTurn = turn{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}

func (m turn) apply(v vec3) vec3 {
	return vec3{
		m[0][0]*v.x + m[0][1]*v.y + m[0][2]*v.z,
		m[1][0]*v.x + m[1][1]*v.y + m[1][2]*v.z,
		m[2][0]*v.x + m[2][1]*v.y + m[2][2]*v.z,
	}
}

// then is m followed by n.
func (m turn) then(n turn) turn {
	var out turn
	for i := range 3 {
		for j := range 3 {
			for k := range 3 {
				out[i][j] += n[i][k] * m[k][j]
			}
		}
	}
	return out
}

func radians(degrees float64) float64 { return degrees * math.Pi / 180 }

// heading turns about the vertical, clockwise as seen from above: +X towards +Z, the model to its right.
func heading(degrees float64) turn {
	s, c := math.Sincos(radians(degrees))
	return turn{{c, 0, -s}, {0, 1, 0}, {s, 0, c}}
}

// pitch tilts the front up: +X towards +Y.
func pitch(degrees float64) turn {
	s, c := math.Sincos(radians(degrees))
	return turn{{c, -s, 0}, {s, c, 0}, {0, 0, 1}}
}

// roll tilts the top to the right: +Y towards +Z.
func roll(degrees float64) turn {
	s, c := math.Sincos(radians(degrees))
	return turn{{1, 0, 0}, {0, c, -s}, {0, s, c}}
}

// rotateTri is one flat face of a model, its normal pointing out of the solid it belongs to.
type rotateTri struct {
	a, b, c vec3
	normal  vec3
	paint   color.RGBA
}

// rotateMesh collects the solids of a model. Every solid is convex, which makes the outward side
// of each face the one away from the solid's centre.
type rotateMesh struct{ tris []rotateTri }

func (m *rotateMesh) face(paint color.RGBA, centre, a, b, c vec3) {
	normal := b.sub(a).cross(c.sub(a))
	if normal.length() < 1e-12 {
		return
	}
	normal = normal.unit()
	if normal.dot(a.add(b).add(c).scale(1.0/3).sub(centre)) < 0 {
		b, c, normal = c, b, normal.scale(-1)
	}
	m.tris = append(m.tris, rotateTri{a: a, b: b, c: c, normal: normal, paint: paint})
}

// solid places points of a unit shape: scaled by size, turned, then moved to centre. Faces are
// turned outward from the mean of their corners, which lies inside any convex solid; centre itself
// need not, and on a wedge it lies on the slope.
func (m *rotateMesh) solid(paint color.RGBA, centre, size vec3, orient turn, faces [][3]vec3) {
	place := func(p vec3) vec3 { return orient.apply(p.times(size)).add(centre) }
	var inner vec3
	for _, f := range faces {
		inner = inner.add(f[0]).add(f[1]).add(f[2])
	}
	inner = place(inner.scale(1 / float64(3*len(faces))))
	for _, f := range faces {
		m.face(paint, inner, place(f[0]), place(f[1]), place(f[2]))
	}
}

// box is a cuboid of the given full size around centre.
func (m *rotateMesh) box(paint color.RGBA, centre, size vec3, orient turn) {
	corner := func(i int) vec3 {
		return vec3{float64(i&1) - 0.5, float64(i>>1&1) - 0.5, float64(i>>2&1) - 0.5}
	}
	quads := [6][4]int{{0, 1, 3, 2}, {4, 5, 7, 6}, {0, 1, 5, 4}, {2, 3, 7, 6}, {0, 2, 6, 4}, {1, 3, 7, 5}}
	var faces [][3]vec3
	for _, q := range quads {
		faces = append(faces, [3]vec3{corner(q[0]), corner(q[1]), corner(q[2])}, [3]vec3{corner(q[0]), corner(q[2]), corner(q[3])})
	}
	m.solid(paint, centre, size, orient, faces)
}

// cylinder stands along its own +Y, size.y tall and size.x by size.z across at the bottom; top is
// the share of that width left at the top, 0 for a cone.
func (m *rotateMesh) cylinder(paint color.RGBA, centre, size vec3, orient turn, top float64, sides int) {
	var faces [][3]vec3
	ring := func(i int, y, r float64) vec3 {
		s, c := math.Sincos(2 * math.Pi * float64(i) / float64(sides))
		return vec3{c * 0.5 * r, y, s * 0.5 * r}
	}
	low, high := vec3{0, -0.5, 0}, vec3{0, 0.5, 0}
	for i := range sides {
		b0, b1 := ring(i, -0.5, 1), ring(i+1, -0.5, 1)
		t0, t1 := ring(i, 0.5, top), ring(i+1, 0.5, top)
		faces = append(faces, [3]vec3{low, b0, b1})
		if top > 0 {
			faces = append(faces, [3]vec3{b0, b1, t1}, [3]vec3{b0, t1, t0}, [3]vec3{high, t0, t1})
		} else {
			faces = append(faces, [3]vec3{b0, b1, high})
		}
	}
	m.solid(paint, centre, size, orient, faces)
}

// rod is a cylinder from one point to another, width across at from and top times that at to.
func (m *rotateMesh) rod(paint color.RGBA, from, to vec3, width, top float64, sides int) {
	axis := to.sub(from)
	y := axis.unit()
	// Any x across the axis will do: the sides are evenly spaced around it.
	x := vec3{0, 0, 1}.cross(y)
	if x.length() < 1e-6 {
		x = vec3{1, 0, 0}.cross(y)
	}
	x = x.unit()
	z := x.cross(y)
	orient := turn{{x.x, y.x, z.x}, {x.y, y.y, z.y}, {x.z, y.z, z.z}}
	m.cylinder(paint, lerp3(from, to, 0.5), vec3{width, axis.length(), width}, orient, top, sides)
}

// loft joins two convex rings of as many corners, bottom and top, into one convex solid.
func (m *rotateMesh) loft(paint color.RGBA, bottom, top []vec3) {
	var centre vec3
	for i := range bottom {
		centre = centre.add(bottom[i]).add(top[i])
	}
	centre = centre.scale(0.5 / float64(len(bottom)))
	n := len(bottom)
	for i := range n {
		j := (i + 1) % n
		m.face(paint, centre, bottom[i], bottom[j], top[j])
		m.face(paint, centre, bottom[i], top[j], top[i])
		if i > 0 && i < n-1 {
			m.face(paint, centre, bottom[0], bottom[i], bottom[j])
			m.face(paint, centre, top[0], top[i], top[j])
		}
	}
}

// ellipsoid has the full width, height and depth of size.
func (m *rotateMesh) ellipsoid(paint color.RGBA, centre, size vec3, orient turn, stacks, slices int) {
	point := func(i, j int) vec3 {
		lat := math.Pi * (float64(i)/float64(stacks) - 0.5)
		lon := 2 * math.Pi * float64(j) / float64(slices)
		sinLat, cosLat := math.Sincos(lat)
		sinLon, cosLon := math.Sincos(lon)
		return vec3{0.5 * cosLat * cosLon, 0.5 * sinLat, 0.5 * cosLat * sinLon}
	}
	var faces [][3]vec3
	for i := range stacks {
		for j := range slices {
			a, b, c, d := point(i, j), point(i, j+1), point(i+1, j+1), point(i+1, j)
			if i > 0 {
				faces = append(faces, [3]vec3{a, b, c})
			}
			if i < stacks-1 {
				faces = append(faces, [3]vec3{a, c, d})
			}
		}
	}
	m.solid(paint, centre, size, orient, faces)
}

// wedge is a ramp: a box whose top slopes from full height at its back (-X) down to its front edge.
func (m *rotateMesh) wedge(paint color.RGBA, centre, size vec3, orient turn) {
	b := [4]vec3{{-0.5, -0.5, -0.5}, {0.5, -0.5, -0.5}, {0.5, -0.5, 0.5}, {-0.5, -0.5, 0.5}}
	t := [2]vec3{{-0.5, 0.5, -0.5}, {-0.5, 0.5, 0.5}}
	faces := [][3]vec3{
		{b[0], b[1], b[2]}, {b[0], b[2], b[3]}, // bottom
		{b[0], b[3], t[1]}, {b[0], t[1], t[0]}, // back
		{b[1], b[2], t[1]}, {b[1], t[1], t[0]}, // slope
		{b[0], b[1], t[0]}, {b[3], b[2], t[1]}, // sides
	}
	m.solid(paint, centre, size, orient, faces)
}

// rotateModel is an object ready to draw: its faces scaled to stand 1 tall on y = 0 and centred
// over the vertical axis it turns about.
type rotateModel struct {
	name string
	tris []rotateTri
	// points are its distinct corners, for its bounds on screen.
	points []vec3
	// reach is how far it extends from that axis; front and back how far along +X and -X.
	reach, front, back, side float64
	// base is how far its parts near the ground reach, below rotateBaseHeight: the ground disc
	// covers them, while parts higher up, a beak or a wing, are seen above the ground anyway.
	base float64
	// main is the colour that covers the most of it, which the arrow must stand out from.
	main color.RGBA
}

// newRotateModel normalises a built mesh. Faces whose normal points straight down at the ground
// are never seen and are dropped.
func newRotateModel(name string, mesh rotateMesh) rotateModel {
	low := vec3{math.Inf(1), math.Inf(1), math.Inf(1)}
	high := low.scale(-1)
	for _, t := range mesh.tris {
		for _, p := range [3]vec3{t.a, t.b, t.c} {
			low = vec3{min(low.x, p.x), min(low.y, p.y), min(low.z, p.z)}
			high = vec3{max(high.x, p.x), max(high.y, p.y), max(high.z, p.z)}
		}
	}
	scale := 1 / (high.y - low.y)
	shift := vec3{(low.x + high.x) / 2, low.y, (low.z + high.z) / 2}
	model := rotateModel{name: name}
	// Areas by colour, in the order the colours first appear, so that a tie always breaks alike.
	var paints []color.RGBA
	areas := map[color.RGBA]float64{}
	for _, t := range mesh.tris {
		place := func(p vec3) vec3 { return p.sub(shift).scale(scale) }
		t.a, t.b, t.c = place(t.a), place(t.b), place(t.c)
		if t.normal.y < -0.999 && max(t.a.y, t.b.y, t.c.y) < 1e-6 {
			continue
		}
		model.tris = append(model.tris, t)
		if _, ok := areas[t.paint]; !ok {
			paints = append(paints, t.paint)
		}
		areas[t.paint] += t.b.sub(t.a).cross(t.c.sub(t.a)).length()
		for _, p := range [3]vec3{t.a, t.b, t.c} {
			model.reach = max(model.reach, p.flat())
			if p.y <= rotateBaseHeight {
				model.base = max(model.base, p.flat())
			}
			model.front, model.back = max(model.front, p.x), max(model.back, -p.x)
			model.side = max(model.side, math.Abs(p.z))
		}
	}
	seen := map[vec3]bool{}
	for _, t := range model.tris {
		for _, p := range [3]vec3{t.a, t.b, t.c} {
			if !seen[p] {
				seen[p] = true
				model.points = append(model.points, p)
			}
		}
	}
	best := -1.0
	for _, paint := range paints {
		if areas[paint] > best {
			best, model.main = areas[paint], paint
		}
	}
	return model
}
