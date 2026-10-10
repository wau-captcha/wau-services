package biz

import (
	"image/color"
	"math"
)

// The ten species of animal_jump, each told apart by its outline and colours: ears, snout, beak or
// patches. Parts are listed back to front.
var allSpecies = []species{
	{"cat", func(tint func(color.NRGBA) color.NRGBA) []part {
		fur, stripe := tint(color.NRGBA{246, 180, 104, 255}), tint(color.NRGBA{216, 132, 62, 255})
		parts := mirrored(fur, true, func(s float64) shape { return polygon{{s * 0.92, -0.28}, {s * 0.66, -1.3}, {s * 0.16, -0.8}} })
		parts = append(parts, mirrored(innerPink, false, func(s float64) shape { return polygon{{s * 0.74, -0.5}, {s * 0.64, -1.04}, {s * 0.36, -0.78}} })...)
		parts = append(parts, part{fur, ellipse{0, 0, 1, 0.92, 0}, true},
			part{stripe, stroke(0, -0.9, 0, -0.62, 0.11), false})
		parts = append(parts, mirrored(stripe, false, func(s float64) shape { return stroke(s*0.2, -0.86, s*0.17, -0.6, 0.09) })...)
		parts = append(parts, eyes(0.36, -0.08, 0.1, 0.15)...)
		parts = append(parts, part{innerPink, polygon{{-0.1, 0.13}, {0.1, 0.13}, {0, 0.24}}, false})
		parts = append(parts, smile(0.27, 0.07)...)
		return append(parts, whiskers(0.24)...)
	}},
	{"dog", func(tint func(color.NRGBA) color.NRGBA) []part {
		fur, ear, muzzle := tint(color.NRGBA{222, 178, 128, 255}), tint(color.NRGBA{136, 90, 56, 255}), tint(color.NRGBA{246, 226, 196, 255})
		parts := []part{{fur, ellipse{0, 0, 0.94, 1, 0}, true}}
		parts = append(parts, mirrored(ear, true, func(s float64) shape { return ellipse{s * 0.7, -0.16, 0.21, 0.5, -s * 0.25} })...)
		parts = append(parts, part{muzzle, ellipse{0, 0.4, 0.42, 0.3, 0}, false},
			part{innerPink, ellipse{0, 0.62, 0.08, 0.08, 0}, true},
			part{eyePaint, ellipse{0, 0.22, 0.14, 0.1, 0}, false})
		parts = append(parts, smile(0.38, 0.08)...)
		return append(parts, eyes(0.34, -0.18, 0.1, 0.13)...)
	}},
	{"rabbit", func(tint func(color.NRGBA) color.NRGBA) []part {
		fur := tint(color.NRGBA{238, 236, 242, 255})
		parts := mirrored(fur, true, func(s float64) shape { return ellipse{s * 0.34, -1.22, 0.2, 0.56, s * 0.12} })
		parts = append(parts, mirrored(innerPink, false, func(s float64) shape { return ellipse{s * 0.34, -1.18, 0.09, 0.4, s * 0.12} })...)
		parts = append(parts, part{fur, ellipse{0, 0, 1, 0.9, 0}, true})
		parts = append(parts, eyes(0.36, -0.1, 0.1, 0.14)...)
		parts = append(parts, part{innerPink, polygon{{-0.08, 0.15}, {0.08, 0.15}, {0, 0.24}}, false},
			part{whitePaint, polygon{{-0.08, 0.31}, {0.08, 0.31}, {0.08, 0.45}, {-0.08, 0.45}}, true},
			part{outlinePaint, stroke(0, 0.31, 0, 0.45, 0.02), false})
		parts = append(parts, smile(0.26, 0.07)...)
		return append(parts, whiskers(0.26)...)
	}},
	{"bear", func(tint func(color.NRGBA) color.NRGBA) []part {
		fur, inner, muzzle := tint(color.NRGBA{156, 104, 66, 255}), tint(color.NRGBA{110, 70, 44, 255}), tint(color.NRGBA{228, 192, 148, 255})
		parts := mirrored(fur, true, func(s float64) shape { return ellipse{s * 0.68, -0.68, 0.3, 0.3, 0} })
		parts = append(parts, mirrored(inner, false, func(s float64) shape { return ellipse{s * 0.68, -0.68, 0.16, 0.16, 0} })...)
		parts = append(parts, part{fur, ellipse{0, 0, 1, 0.94, 0}, true},
			part{muzzle, ellipse{0, 0.33, 0.42, 0.32, 0}, false},
			part{eyePaint, ellipse{0, 0.19, 0.15, 0.1, 0}, false})
		parts = append(parts, smile(0.36, 0.08)...)
		return append(parts, eyes(0.36, -0.15, 0.09, 0.12)...)
	}},
	{"pig", func(tint func(color.NRGBA) color.NRGBA) []part {
		skin, snout := tint(color.NRGBA{248, 182, 192, 255}), tint(color.NRGBA{232, 136, 154, 255})
		parts := mirrored(snout, true, func(s float64) shape { return polygon{{s * 0.86, -0.42}, {s * 0.78, -1.08}, {s * 0.3, -0.78}} })
		parts = append(parts, part{skin, ellipse{0, 0, 1, 0.9, 0}, true}, part{snout, ellipse{0, 0.27, 0.38, 0.26, 0}, true})
		parts = append(parts, mirrored(color.NRGBA{150, 70, 92, 255}, false, func(s float64) shape { return ellipse{s * 0.13, 0.27, 0.06, 0.1, 0} })...)
		return append(parts, eyes(0.4, -0.22, 0.09, 0.12)...)
	}},
	{"frog", func(tint func(color.NRGBA) color.NRGBA) []part {
		skin := tint(color.NRGBA{118, 196, 96, 255})
		parts := mirrored(skin, true, func(s float64) shape { return ellipse{s * 0.5, -0.62, 0.36, 0.34, 0} })
		parts = append(parts, part{skin, ellipse{0, 0.06, 1, 0.82, 0}, true})
		parts = append(parts, mirrored(whitePaint, false, func(s float64) shape { return ellipse{s * 0.5, -0.62, 0.24, 0.23, 0} })...)
		parts = append(parts, mirrored(eyePaint, false, func(s float64) shape { return ellipse{s * 0.5, -0.58, 0.11, 0.13, 0} })...)
		parts = append(parts, mirrored(whitePaint, false, func(s float64) shape { return ellipse{s*0.5 - 0.04, -0.63, 0.04, 0.04, 0} })...)
		parts = append(parts, part{outlinePaint, arc(0, 0.0, 0.6, 0.42, 0.2*math.Pi, 0.8*math.Pi, 0.06), false})
		parts = append(parts, mirrored(eyePaint, false, func(s float64) shape { return ellipse{s * 0.1, 0.02, 0.03, 0.025, 0} })...)
		return append(parts, mirrored(innerPink, false, func(s float64) shape { return ellipse{s * 0.66, 0.3, 0.12, 0.08, 0} })...)
	}},
	{"panda", func(tint func(color.NRGBA) color.NRGBA) []part {
		fur, black := tint(color.NRGBA{250, 250, 248, 255}), color.NRGBA{40, 40, 46, 255}
		parts := mirrored(black, true, func(s float64) shape { return ellipse{s * 0.68, -0.7, 0.3, 0.3, 0} })
		parts = append(parts, part{fur, ellipse{0, 0, 1, 0.92, 0}, true})
		parts = append(parts, mirrored(black, false, func(s float64) shape { return ellipse{s * 0.38, -0.06, 0.2, 0.28, -s * 0.5} })...)
		parts = append(parts, mirrored(whitePaint, false, func(s float64) shape { return ellipse{s * 0.37, -0.1, 0.085, 0.085, 0} })...)
		parts = append(parts, mirrored(eyePaint, false, func(s float64) shape { return ellipse{s * 0.37, -0.1, 0.045, 0.045, 0} })...)
		parts = append(parts, part{black, ellipse{0, 0.25, 0.12, 0.08, 0}, false})
		return append(parts, smile(0.35, 0.07)...)
	}},
	{"chick", func(tint func(color.NRGBA) color.NRGBA) []part {
		feathers, beak := tint(color.NRGBA{252, 214, 68, 255}), tint(color.NRGBA{246, 142, 42, 255})
		parts := []part{{feathers, ellipse{0, -1.02, 0.08, 0.2, 0}, true}}
		parts = append(parts, mirrored(feathers, true, func(s float64) shape { return ellipse{s * 0.14, -0.98, 0.07, 0.17, s * 0.55} })...)
		parts = append(parts, part{feathers, ellipse{0, 0, 1, 1, 0}, true})
		parts = append(parts, eyes(0.34, -0.14, 0.09, 0.12)...)
		parts = append(parts, part{beak, polygon{{-0.18, 0.16}, {0, 0.04}, {0.18, 0.16}, {0, 0.36}}, true},
			part{outlinePaint, stroke(-0.17, 0.165, 0.17, 0.165, 0.025), false})
		return append(parts, mirrored(innerPink, false, func(s float64) shape { return ellipse{s * 0.6, 0.22, 0.13, 0.08, 0} })...)
	}},
	{"mouse", func(tint func(color.NRGBA) color.NRGBA) []part {
		fur, nose := tint(color.NRGBA{170, 170, 184, 255}), color.NRGBA{226, 110, 136, 255}
		parts := mirrored(fur, true, func(s float64) shape { return ellipse{s * 0.62, -0.66, 0.36, 0.36, 0} })
		parts = append(parts, mirrored(innerPink, false, func(s float64) shape { return ellipse{s * 0.62, -0.66, 0.22, 0.22, 0} })...)
		parts = append(parts, part{fur, ellipse{0, 0.04, 0.92, 0.88, 0}, true})
		parts = append(parts, eyes(0.3, -0.06, 0.085, 0.11)...)
		parts = append(parts, part{nose, ellipse{0, 0.3, 0.09, 0.08, 0}, true})
		parts = append(parts, smile(0.4, 0.06)...)
		return append(parts, whiskers(0.33)...)
	}},
	{"fox", func(tint func(color.NRGBA) color.NRGBA) []part {
		fur, tip, cream := tint(color.NRGBA{234, 122, 50, 255}), color.NRGBA{66, 42, 34, 255}, tint(color.NRGBA{250, 240, 226, 255})
		parts := mirrored(fur, true, func(s float64) shape { return polygon{{s * 0.9, -0.3}, {s * 0.7, -1.3}, {s * 0.2, -0.78}} })
		parts = append(parts, mirrored(tip, false, func(s float64) shape { return polygon{{s * 0.75, -1.05}, {s * 0.7, -1.3}, {s * 0.55, -1.14}} })...)
		parts = append(parts, mirrored(cream, false, func(s float64) shape { return polygon{{s * 0.76, -0.5}, {s * 0.69, -0.98}, {s * 0.38, -0.78}} })...)
		parts = append(parts, part{fur, ellipse{0, 0, 1, 0.92, 0}, true}, part{cream, ellipse{0, 0.42, 0.6, 0.42, 0}, false})
		parts = append(parts, mirrored(eyePaint, false, func(s float64) shape { return ellipse{s * 0.36, -0.12, 0.1, 0.13, s * 0.35} })...)
		parts = append(parts, mirrored(whitePaint, false, func(s float64) shape { return ellipse{s*0.36 - 0.03, -0.16, 0.035, 0.035, 0} })...)
		parts = append(parts, part{eyePaint, ellipse{0, 0.24, 0.13, 0.09, 0}, false})
		return append(parts, smile(0.36, 0.07)...)
	}},
}

// eyes are two dark ovals, each with a glint on the same side, as if lit from the top left.
func eyes(x, y, rx, ry float64) []part {
	parts := mirrored(eyePaint, false, func(s float64) shape { return ellipse{s * x, y, rx, ry, 0} })
	return append(parts, mirrored(whitePaint, false, func(s float64) shape {
		return ellipse{s*x - rx*0.3, y - ry*0.35, rx * 0.38, rx * 0.38, 0}
	})...)
}

// smile is the "w" of a mouth under the nose.
func smile(y, size float64) []part {
	return mirrored(outlinePaint, false, func(s float64) shape { return arc(s*size, y, size, size, 0.1*math.Pi, 0.9*math.Pi, 0.04) })
}

func whiskers(y float64) []part {
	parts := mirrored(outlinePaint, false, func(s float64) shape { return stroke(s*0.38, y-0.04, s*0.9, y-0.12, 0.035) })
	return append(parts, mirrored(outlinePaint, false, func(s float64) shape { return stroke(s*0.38, y+0.04, s*0.9, y+0.08, 0.035) })...)
}

// speciesExtent is the box around each species' parts, in units of the face's radius; tints do
// not move parts, so it is worked out once.
var speciesExtent = func() [][4]float64 {
	same := func(paint color.NRGBA) color.NRGBA { return paint }
	extents := make([][4]float64, len(allSpecies))
	for i, kind := range allSpecies {
		minX, minY, maxX, maxY := partsExtent(kind.parts(same))
		extents[i] = [4]float64{minX, minY, maxX, maxY}
	}
	return extents
}()
