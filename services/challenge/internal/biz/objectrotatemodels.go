package biz

import "image/color"

// The objects of object_rotate, each built from simple solids with an unmistakable front. In a
// model's own space +X is its front, +Y up and +Z its right, the ground at y = 0; newRotateModel
// scales each to stand 1 tall, so sizes here are only relative. Each is kept about as deep as it is
// tall from every side, toy-like, so that it fills the frame alike in all eight directions.
var rotateModels = []rotateModel{
	newRotateModel("dog", modelDog()),
	newRotateModel("cat", modelCat()),
	newRotateModel("horse", modelHorse()),
	newRotateModel("cow", modelCow()),
	newRotateModel("elephant", modelElephant()),
	newRotateModel("rabbit", modelRabbit()),
	newRotateModel("duck", modelDuck()),
	newRotateModel("penguin", modelPenguin()),
	newRotateModel("fish", modelFish()),
	newRotateModel("turtle", modelTurtle()),
	newRotateModel("person", modelPerson()),
	newRotateModel("car", modelCar()),
	newRotateModel("truck", modelTruck()),
	newRotateModel("bus", modelBus()),
	newRotateModel("tractor", modelTractor()),
	newRotateModel("airplane", modelAirplane()),
	newRotateModel("helicopter", modelHelicopter()),
	newRotateModel("boat", modelBoat()),
	newRotateModel("train engine", modelTrainEngine()),
	newRotateModel("tank", modelTank()),
	newRotateModel("desk fan", modelDeskFan()),
	newRotateModel("desk lamp", modelDeskLamp()),
	newRotateModel("teapot", modelTeapot()),
	newRotateModel("sneaker", modelSneaker()),
	newRotateModel("armchair", modelArmchair()),
	newRotateModel("office chair", modelOfficeChair()),
	newRotateModel("watering can", modelWateringCan()),
	newRotateModel("camera", modelCamera()),
}

func rgb(r, g, b uint8) color.RGBA { return color.RGBA{r, g, b, 255} }

// Colours several models share.
var (
	rotateInk   = rgb(30, 30, 34)
	rotateTyre  = rgb(46, 46, 50)
	rotateGlass = rgb(150, 196, 226)
	rotateLamp  = rgb(255, 236, 140)
	rotateRed   = rgb(216, 46, 42)
	rotateChalk = rgb(244, 244, 240)
	rotateSteel = rgb(160, 164, 172)
)

// surface is the point of the ellipsoid (centre, size) in direction dir from its centre.
func surface(centre, size, dir vec3) vec3 {
	q := vec3{dir.x / size.x, dir.y / size.y, dir.z / size.z}.scale(2)
	return centre.add(dir.scale(1 / q.length()))
}

// eyes puts a dark eye at at and another mirrored across the middle.
func (m *rotateMesh) eyes(at vec3, size float64) {
	for _, side := range []float64{-1, 1} {
		m.ellipsoid(rotateInk, vec3{at.x, at.y, side * at.z}, vec3{size, size * 1.2, size}, noTurn, 4, 8)
	}
}

// wheel is a tyre turning about the Z axis with its hub on the outer side.
func (m *rotateMesh) wheel(at vec3, diameter, width float64, hub color.RGBA) {
	m.cylinder(rotateTyre, at, vec3{diameter, width, diameter}, roll(90), 1, 14)
	side := 1.0
	if at.z < 0 {
		side = -1
	}
	m.cylinder(hub, at.add(vec3{0, 0, side * width / 2}), vec3{diameter * 0.5, 0.02, diameter * 0.5}, roll(90), 1, 10)
}

func modelDog() rotateMesh {
	var m rotateMesh
	fur, ears, pale := rgb(198, 148, 94), rgb(104, 66, 38), rgb(240, 222, 192)
	m.ellipsoid(fur, vec3{-0.02, 0.5, 0}, vec3{0.66, 0.38, 0.36}, noTurn, 8, 14)
	m.ellipsoid(pale, vec3{0.16, 0.46, 0}, vec3{0.28, 0.28, 0.3}, noTurn, 6, 10)
	for _, x := range []float64{0.18, -0.2} {
		for _, side := range []float64{-1, 1} {
			m.rod(fur, vec3{x, 0.46, side * 0.1}, vec3{x, 0.04, side * 0.1}, 0.1, 0.85, 8)
			m.ellipsoid(pale, vec3{x + 0.03, 0.035, side * 0.1}, vec3{0.15, 0.07, 0.1}, noTurn, 4, 8)
		}
	}
	m.rod(fur, vec3{0.2, 0.56, 0}, vec3{0.32, 0.8, 0}, 0.18, 0.8, 8)
	m.rod(rotateRed, vec3{0.224, 0.608, 0}, vec3{0.242, 0.644, 0}, 0.2, 1, 10)
	head, headSize := vec3{0.36, 0.84, 0}, vec3{0.28, 0.27, 0.26}
	m.ellipsoid(fur, head, headSize, noTurn, 8, 12)
	m.ellipsoid(pale, vec3{0.51, 0.78, 0}, vec3{0.2, 0.13, 0.15}, noTurn, 6, 10)
	m.ellipsoid(rotateInk, vec3{0.61, 0.8, 0}, vec3{0.05, 0.05, 0.07}, noTurn, 4, 8)
	m.eyes(surface(head, headSize, vec3{0.75, 0.4, 0.5}), 0.045)
	for _, side := range []float64{-1, 1} {
		m.ellipsoid(ears, vec3{0.32, 0.8, side * 0.14}, vec3{0.12, 0.24, 0.05}, roll(-side*12), 5, 8)
	}
	m.rod(fur, vec3{-0.3, 0.58, 0}, vec3{-0.4, 0.84, 0}, 0.07, 0.5, 6)
	return m
}

// modelCat sits, its tail curled round on the ground.
func modelCat() rotateMesh {
	var m rotateMesh
	fur, cream, pink := rgb(234, 148, 68), rgb(248, 234, 210), rgb(240, 150, 162)
	m.ellipsoid(fur, vec3{-0.05, 0.26, 0}, vec3{0.52, 0.52, 0.42}, noTurn, 8, 14)
	m.ellipsoid(cream, vec3{0.09, 0.44, 0}, vec3{0.28, 0.44, 0.28}, noTurn, 8, 12)
	head, headSize := vec3{0.08, 0.78, 0}, vec3{0.3, 0.27, 0.33}
	m.ellipsoid(fur, head, headSize, noTurn, 8, 12)
	m.ellipsoid(cream, vec3{0.2, 0.74, 0}, vec3{0.12, 0.1, 0.17}, noTurn, 5, 8)
	m.ellipsoid(pink, vec3{0.26, 0.77, 0}, vec3{0.035, 0.03, 0.045}, noTurn, 4, 6)
	m.eyes(surface(head, headSize, vec3{0.7, 0.3, 0.55}), 0.05)
	for _, side := range []float64{-1, 1} {
		m.rod(fur, vec3{0.16, 0.42, side * 0.07}, vec3{0.18, 0.04, side * 0.07}, 0.08, 0.9, 8)
		m.ellipsoid(cream, vec3{0.21, 0.03, side * 0.07}, vec3{0.12, 0.06, 0.09}, noTurn, 4, 8)
		m.rod(fur, vec3{0.06, 0.86, side * 0.085}, vec3{0.05, 1.02, side * 0.125}, 0.11, 0, 6)
	}
	m.rod(fur, vec3{-0.27, 0.05, 0.04}, vec3{-0.08, 0.045, 0.23}, 0.08, 1, 8)
	m.rod(fur, vec3{-0.08, 0.045, 0.23}, vec3{0.16, 0.04, 0.22}, 0.08, 0.7, 8)
	return m
}

func modelHorse() rotateMesh {
	var m rotateMesh
	coat, mane, hoof := rgb(152, 98, 56), rgb(66, 44, 30), rgb(48, 42, 38)
	m.ellipsoid(coat, vec3{-0.02, 0.6, 0}, vec3{0.64, 0.36, 0.32}, noTurn, 8, 14)
	for _, x := range []float64{0.2, -0.22} {
		for _, side := range []float64{-1, 1} {
			m.rod(coat, vec3{x, 0.56, side * 0.09}, vec3{x, 0.05, side * 0.09}, 0.09, 0.75, 8)
			m.box(hoof, vec3{x + 0.01, 0.025, side * 0.09}, vec3{0.1, 0.05, 0.09}, noTurn)
		}
	}
	m.rod(coat, vec3{0.2, 0.66, 0}, vec3{0.3, 0.96, 0}, 0.2, 0.7, 8)
	m.rod(mane, vec3{0.13, 0.72, 0}, vec3{0.24, 1.03, 0}, 0.05, 1, 6)
	m.ellipsoid(coat, vec3{0.4, 0.97, 0}, vec3{0.32, 0.15, 0.15}, pitch(-40), 6, 10)
	m.ellipsoid(mane, vec3{0.52, 0.88, 0}, vec3{0.11, 0.1, 0.13}, pitch(-40), 5, 8)
	m.eyes(vec3{0.38, 1.01, 0.07}, 0.035)
	for _, side := range []float64{-1, 1} {
		m.rod(coat, vec3{0.3, 1.03, side * 0.04}, vec3{0.28, 1.13, side * 0.05}, 0.05, 0, 5)
	}
	m.rod(mane, vec3{-0.33, 0.7, 0}, vec3{-0.44, 0.38, 0}, 0.08, 0.6, 6)
	return m
}

func modelCow() rotateMesh {
	var m rotateMesh
	hide, patch, pink, horn := rgb(246, 244, 238), rgb(38, 36, 38), rgb(238, 164, 164), rgb(232, 222, 194)
	m.ellipsoid(hide, vec3{-0.04, 0.55, 0}, vec3{0.78, 0.4, 0.4}, noTurn, 8, 14)
	// Black ellipsoids that just break the surface make the patches.
	for _, p := range [][2]vec3{
		{{0.06, 0.64, 0.14}, {0.26, 0.22, 0.14}},
		{{-0.24, 0.52, -0.15}, {0.24, 0.24, 0.12}},
		{{-0.18, 0.7, 0.06}, {0.2, 0.14, 0.2}},
		{{0.12, 0.56, -0.16}, {0.16, 0.14, 0.1}},
	} {
		m.ellipsoid(patch, p[0], p[1], noTurn, 5, 8)
	}
	for _, x := range []float64{0.2, -0.28} {
		for _, side := range []float64{-1, 1} {
			m.rod(hide, vec3{x, 0.5, side * 0.12}, vec3{x, 0.05, side * 0.12}, 0.1, 0.8, 8)
			m.box(patch, vec3{x, 0.025, side * 0.12}, vec3{0.1, 0.05, 0.09}, noTurn)
		}
	}
	m.ellipsoid(pink, vec3{-0.2, 0.37, 0}, vec3{0.17, 0.1, 0.16}, noTurn, 5, 8)
	m.rod(hide, vec3{0.24, 0.58, 0}, vec3{0.42, 0.64, 0}, 0.24, 0.8, 8)
	head, headSize := vec3{0.47, 0.63, 0}, vec3{0.26, 0.24, 0.23}
	m.ellipsoid(hide, head, headSize, noTurn, 8, 12)
	m.ellipsoid(patch, vec3{0.47, 0.7, 0.05}, vec3{0.18, 0.12, 0.14}, noTurn, 5, 8)
	m.ellipsoid(pink, vec3{0.6, 0.57, 0}, vec3{0.14, 0.14, 0.19}, noTurn, 6, 10)
	m.eyes(surface(head, headSize, vec3{0.6, 0.35, 0.7}), 0.04)
	for _, side := range []float64{-1, 1} {
		m.rod(horn, vec3{0.45, 0.72, side * 0.07}, vec3{0.47, 0.81, side * 0.18}, 0.045, 0.3, 6)
		m.ellipsoid(hide, vec3{0.43, 0.67, side * 0.15}, vec3{0.06, 0.05, 0.13}, noTurn, 4, 8)
	}
	m.rod(hide, vec3{-0.42, 0.62, 0}, vec3{-0.46, 0.3, 0}, 0.03, 1, 5)
	m.ellipsoid(patch, vec3{-0.46, 0.28, 0}, vec3{0.05, 0.08, 0.05}, noTurn, 4, 6)
	return m
}

func modelElephant() rotateMesh {
	var m rotateMesh
	skin, ear, tusk := rgb(150, 152, 162), rgb(172, 162, 172), rgb(246, 240, 222)
	m.ellipsoid(skin, vec3{-0.06, 0.56, 0}, vec3{0.8, 0.56, 0.54}, noTurn, 8, 14)
	for _, x := range []float64{0.18, -0.3} {
		for _, side := range []float64{-1, 1} {
			m.cylinder(skin, vec3{x, 0.18, side * 0.15}, vec3{0.18, 0.36, 0.18}, noTurn, 1, 10)
		}
	}
	head, headSize := vec3{0.36, 0.7, 0}, vec3{0.36, 0.4, 0.38}
	m.ellipsoid(skin, head, headSize, noTurn, 8, 12)
	for _, side := range []float64{-1, 1} {
		m.ellipsoid(ear, vec3{0.28, 0.7, side * 0.25}, vec3{0.06, 0.38, 0.3}, heading(side*30), 6, 10)
		m.rod(tusk, vec3{0.48, 0.56, side * 0.08}, vec3{0.64, 0.5, side * 0.1}, 0.05, 0.3, 6)
	}
	m.rod(skin, vec3{0.5, 0.66, 0}, vec3{0.6, 0.42, 0}, 0.13, 0.85, 8)
	m.rod(skin, vec3{0.6, 0.42, 0}, vec3{0.63, 0.2, 0}, 0.11, 0.85, 8)
	m.rod(skin, vec3{0.63, 0.2, 0}, vec3{0.72, 0.12, 0}, 0.09, 0.9, 8)
	m.eyes(surface(head, headSize, vec3{0.6, 0.4, 0.7}), 0.035)
	m.rod(skin, vec3{-0.45, 0.6, 0}, vec3{-0.5, 0.36, 0}, 0.03, 0.8, 5)
	return m
}

// modelRabbit sits up, its long ears standing.
func modelRabbit() rotateMesh {
	var m rotateMesh
	fur, white, pink := rgb(178, 152, 130), rgb(246, 244, 240), rgb(240, 160, 170)
	m.ellipsoid(fur, vec3{-0.06, 0.28, 0}, vec3{0.56, 0.5, 0.42}, noTurn, 8, 14)
	for _, side := range []float64{-1, 1} {
		m.ellipsoid(fur, vec3{-0.02, 0.05, side * 0.15}, vec3{0.42, 0.1, 0.13}, noTurn, 5, 10)
		m.ellipsoid(fur, vec3{0.2, 0.06, side * 0.07}, vec3{0.13, 0.1, 0.09}, noTurn, 4, 8)
		lean := pitch(14).then(roll(side * 8))
		m.ellipsoid(fur, vec3{0.1, 0.92, side * 0.065}, vec3{0.08, 0.4, 0.11}, lean, 6, 8)
		m.ellipsoid(pink, vec3{0.134, 0.928, side * 0.065}, vec3{0.03, 0.3, 0.07}, lean, 5, 8)
	}
	head, headSize := vec3{0.17, 0.6, 0}, vec3{0.3, 0.28, 0.27}
	m.ellipsoid(fur, head, headSize, noTurn, 8, 12)
	m.ellipsoid(white, vec3{0.29, 0.56, 0}, vec3{0.1, 0.1, 0.14}, noTurn, 5, 8)
	m.ellipsoid(pink, vec3{0.33, 0.6, 0}, vec3{0.035, 0.03, 0.045}, noTurn, 4, 6)
	m.eyes(surface(head, headSize, vec3{0.7, 0.35, 0.6}), 0.045)
	m.ellipsoid(white, vec3{-0.35, 0.26, 0}, vec3{0.14, 0.14, 0.14}, noTurn, 5, 8)
	return m
}

func modelDuck() rotateMesh {
	var m rotateMesh
	down, wing, beak := rgb(250, 214, 60), rgb(236, 188, 40), rgb(246, 132, 30)
	m.ellipsoid(down, vec3{-0.04, 0.3, 0}, vec3{0.7, 0.38, 0.44}, noTurn, 8, 14)
	m.rod(down, vec3{-0.3, 0.32, 0}, vec3{-0.46, 0.5, 0}, 0.2, 0.2, 8)
	m.rod(down, vec3{0.12, 0.38, 0}, vec3{0.18, 0.56, 0}, 0.2, 0.9, 8)
	head, headSize := vec3{0.2, 0.64, 0}, vec3{0.32, 0.32, 0.3}
	m.ellipsoid(down, head, headSize, noTurn, 8, 12)
	m.ellipsoid(beak, vec3{0.4, 0.6, 0}, vec3{0.22, 0.07, 0.16}, noTurn, 5, 10)
	m.eyes(surface(head, headSize, vec3{0.6, 0.35, 0.65}), 0.045)
	for _, side := range []float64{-1, 1} {
		m.ellipsoid(wing, vec3{-0.06, 0.34, side * 0.2}, vec3{0.38, 0.17, 0.07}, pitch(10), 6, 10)
		m.rod(beak, vec3{0.02, 0.14, side * 0.09}, vec3{0.02, 0.03, side * 0.09}, 0.04, 1, 6)
		m.box(beak, vec3{0.08, 0.015, side * 0.09}, vec3{0.17, 0.03, 0.11}, noTurn)
	}
	return m
}

func modelPenguin() rotateMesh {
	var m rotateMesh
	coat, belly, beak := rgb(44, 46, 56), rgb(246, 246, 242), rgb(246, 150, 30)
	m.ellipsoid(coat, vec3{-0.02, 0.42, 0}, vec3{0.46, 0.78, 0.44}, noTurn, 10, 14)
	m.ellipsoid(belly, vec3{0.05, 0.38, 0}, vec3{0.38, 0.62, 0.36}, noTurn, 8, 12)
	head, headSize := vec3{0, 0.84, 0}, vec3{0.32, 0.3, 0.32}
	m.ellipsoid(coat, head, headSize, noTurn, 8, 12)
	m.ellipsoid(belly, vec3{0.07, 0.84, 0}, vec3{0.2, 0.16, 0.22}, noTurn, 6, 10)
	m.rod(beak, vec3{0.15, 0.82, 0}, vec3{0.3, 0.79, 0}, 0.07, 0.15, 6)
	m.eyes(surface(head, headSize, vec3{0.75, 0.25, 0.45}), 0.04)
	for _, side := range []float64{-1, 1} {
		m.ellipsoid(coat, vec3{-0.02, 0.48, side * 0.23}, vec3{0.13, 0.42, 0.05}, roll(-side*16), 6, 8)
		m.box(beak, vec3{0.1, 0.02, side * 0.08}, vec3{0.2, 0.04, 0.1}, noTurn)
	}
	return m
}

// modelFish rests on its belly, deep-bodied like a goldfish.
func modelFish() rotateMesh {
	var m rotateMesh
	scales, band, fin := rgb(255, 136, 28), rgb(250, 250, 246), rgb(252, 186, 60)
	body, bodySize := vec3{0, 0.36, 0}, vec3{0.66, 0.62, 0.34}
	m.ellipsoid(scales, body, bodySize, noTurn, 10, 16)
	m.ellipsoid(band, vec3{0.11, 0.36, 0}, vec3{0.08, 0.56, 0.35}, noTurn, 8, 12)
	flat := func(outline ...vec3) {
		var near, far []vec3
		for _, p := range outline {
			near, far = append(near, vec3{p.x, p.y, -0.018}), append(far, vec3{p.x, p.y, 0.018})
		}
		m.loft(fin, near, far)
	}
	flat(vec3{-0.28, 0.32, 0}, vec3{-0.28, 0.44, 0}, vec3{-0.5, 0.64, 0}, vec3{-0.5, 0.26, 0})
	flat(vec3{0.08, 0.62, 0}, vec3{-0.18, 0.6, 0}, vec3{-0.12, 0.78, 0})
	for _, side := range []float64{-1, 1} {
		eye := surface(body, bodySize, vec3{0.75, 0.25, side * 0.5})
		m.ellipsoid(band, eye, vec3{0.1, 0.1, 0.05}, noTurn, 5, 8)
		m.ellipsoid(rotateInk, eye.add(vec3{0.01, 0, side * 0.02}), vec3{0.05, 0.05, 0.03}, noTurn, 4, 8)
	}
	m.ellipsoid(rotateRed, vec3{0.325, 0.32, 0}, vec3{0.05, 0.06, 0.1}, noTurn, 4, 8)
	return m
}

func modelTurtle() rotateMesh {
	var m rotateMesh
	shell, rim, plate, skin := rgb(70, 132, 72), rgb(196, 170, 96), rgb(48, 100, 52), rgb(146, 170, 92)
	m.ellipsoid(rim, vec3{0, 0.17, 0}, vec3{0.84, 0.12, 0.7}, noTurn, 5, 16)
	m.ellipsoid(shell, vec3{0, 0.24, 0}, vec3{0.76, 0.44, 0.62}, noTurn, 8, 16)
	for _, p := range []vec3{{0.12, 0.44, 0}, {-0.14, 0.44, 0}, {0, 0.41, 0.17}, {0, 0.41, -0.17}} {
		m.ellipsoid(plate, p, vec3{0.2, 0.06, 0.18}, noTurn, 4, 8)
	}
	for _, x := range []float64{0.24, -0.24} {
		for _, side := range []float64{-1, 1} {
			m.ellipsoid(skin, vec3{x, 0.07, side * 0.28}, vec3{0.17, 0.14, 0.15}, noTurn, 5, 8)
		}
	}
	m.rod(skin, vec3{0.3, 0.16, 0}, vec3{0.46, 0.22, 0}, 0.13, 0.9, 8)
	head, headSize := vec3{0.52, 0.24, 0}, vec3{0.2, 0.15, 0.15}
	m.ellipsoid(skin, head, headSize, noTurn, 6, 10)
	m.eyes(surface(head, headSize, vec3{0.6, 0.45, 0.6}), 0.035)
	m.rod(skin, vec3{-0.38, 0.12, 0}, vec3{-0.5, 0.08, 0}, 0.06, 0, 5)
	return m
}

// modelPerson wears a cap with its peak to the front and a backpack.
func modelPerson() rotateMesh {
	var m rotateMesh
	skin, shirt, jeans, hair := rgb(240, 198, 162), rgb(220, 64, 52), rgb(52, 84, 156), rgb(82, 54, 36)
	hat, pack := rgb(40, 96, 206), rgb(60, 150, 90)
	for _, side := range []float64{-1, 1} {
		m.rod(jeans, vec3{0, 0.49, side * 0.07}, vec3{0, 0.06, side * 0.07}, 0.11, 0.9, 8)
		m.box(rotateInk, vec3{0.04, 0.03, side * 0.07}, vec3{0.17, 0.06, 0.09}, noTurn)
		m.rod(shirt, vec3{0, 0.8, side * 0.19}, vec3{0.05, 0.53, side * 0.21}, 0.08, 0.9, 8)
		m.ellipsoid(skin, vec3{0.055, 0.5, side * 0.21}, vec3{0.07, 0.08, 0.06}, noTurn, 4, 8)
	}
	m.box(shirt, vec3{0, 0.66, 0}, vec3{0.15, 0.34, 0.32}, noTurn)
	m.box(rotateChalk, vec3{0.076, 0.7, 0}, vec3{0.01, 0.08, 0.1}, noTurn)
	m.box(pack, vec3{-0.13, 0.67, 0}, vec3{0.11, 0.24, 0.22}, noTurn)
	head, headSize := vec3{0, 0.92, 0}, vec3{0.2, 0.22, 0.19}
	m.ellipsoid(skin, head, headSize, noTurn, 8, 12)
	m.ellipsoid(hair, vec3{-0.025, 0.95, 0}, vec3{0.19, 0.19, 0.2}, noTurn, 6, 10)
	m.ellipsoid(hat, vec3{0, 1.0, 0}, vec3{0.21, 0.12, 0.2}, noTurn, 5, 10)
	m.box(hat, vec3{0.13, 0.98, 0}, vec3{0.14, 0.02, 0.17}, noTurn)
	m.ellipsoid(skin, vec3{0.1, 0.9, 0}, vec3{0.04, 0.05, 0.04}, noTurn, 4, 6)
	m.eyes(surface(head, headSize, vec3{0.75, 0.2, 0.45}), 0.03)
	return m
}

func modelCar() rotateMesh {
	var m rotateMesh
	paint := rgb(40, 112, 222)
	m.box(paint, vec3{0, 0.32, 0}, vec3{1.36, 0.3, 0.84}, noTurn)
	// The cabin is glass under a painted roof, the windscreen sloping to the front.
	m.loft(rotateGlass,
		[]vec3{{-0.52, 0.47, -0.39}, {0.3, 0.47, -0.39}, {0.3, 0.47, 0.39}, {-0.52, 0.47, 0.39}},
		[]vec3{{-0.44, 0.76, -0.34}, {0.06, 0.76, -0.34}, {0.06, 0.76, 0.34}, {-0.44, 0.76, 0.34}})
	m.box(paint, vec3{-0.19, 0.775, 0}, vec3{0.52, 0.03, 0.7}, noTurn)
	m.box(rotateInk, vec3{0.685, 0.28, 0}, vec3{0.02, 0.08, 0.3}, noTurn)
	for _, side := range []float64{-1, 1} {
		m.box(rotateLamp, vec3{0.685, 0.36, side * 0.28}, vec3{0.02, 0.08, 0.16}, noTurn)
		m.box(rotateRed, vec3{-0.685, 0.38, side * 0.3}, vec3{0.02, 0.07, 0.14}, noTurn)
		for _, x := range []float64{0.42, -0.42} {
			m.wheel(vec3{x, 0.16, side * 0.4}, 0.32, 0.12, rotateSteel)
		}
	}
	m.box(rotateSteel, vec3{0.69, 0.2, 0}, vec3{0.06, 0.06, 0.8}, noTurn)
	m.box(rotateSteel, vec3{-0.69, 0.2, 0}, vec3{0.06, 0.06, 0.8}, noTurn)
	return m
}

// modelTruck has its low cab in front of a tall cargo box.
func modelTruck() rotateMesh {
	var m rotateMesh
	cab, cargo := rgb(214, 52, 44), rgb(232, 232, 226)
	m.box(rotateTyre, vec3{0, 0.2, 0}, vec3{1.3, 0.1, 0.62}, noTurn)
	m.box(cab, vec3{0.44, 0.48, 0}, vec3{0.42, 0.56, 0.76}, noTurn)
	m.box(rotateGlass, vec3{0.655, 0.6, 0}, vec3{0.01, 0.2, 0.62}, noTurn)
	m.box(rotateInk, vec3{0.655, 0.32, 0}, vec3{0.01, 0.08, 0.3}, noTurn)
	m.box(cargo, vec3{-0.2, 0.6, 0}, vec3{0.86, 0.8, 0.8}, noTurn)
	m.box(rotateSteel, vec3{-0.631, 0.6, 0}, vec3{0.01, 0.7, 0.02}, noTurn)
	for _, side := range []float64{-1, 1} {
		m.box(rotateGlass, vec3{0.5, 0.6, side * 0.381}, vec3{0.22, 0.18, 0.01}, noTurn)
		m.box(rotateLamp, vec3{0.655, 0.3, side * 0.28}, vec3{0.01, 0.07, 0.14}, noTurn)
		m.box(rotateRed, vec3{-0.631, 0.28, side * 0.32}, vec3{0.01, 0.06, 0.1}, noTurn)
		m.wheel(vec3{0.44, 0.16, side * 0.33}, 0.32, 0.12, rotateSteel)
		m.wheel(vec3{-0.4, 0.16, side * 0.33}, 0.32, 0.12, rotateSteel)
	}
	return m
}

// modelBus is a school bus, its engine under a short bonnet at the front.
func modelBus() rotateMesh {
	var m rotateMesh
	yellow := rgb(248, 192, 28)
	m.box(yellow, vec3{-0.1, 0.56, 0}, vec3{1.1, 0.76, 0.74}, noTurn)
	m.box(yellow, vec3{0.55, 0.36, 0}, vec3{0.22, 0.34, 0.64}, noTurn)
	m.box(rotateGlass, vec3{0.451, 0.74, 0}, vec3{0.01, 0.22, 0.62}, noTurn)
	m.box(rotateGlass, vec3{-0.651, 0.74, 0}, vec3{0.01, 0.2, 0.5}, noTurn)
	m.box(rotateInk, vec3{0.67, 0.22, 0}, vec3{0.04, 0.07, 0.68}, noTurn)
	m.box(rotateInk, vec3{-0.67, 0.22, 0}, vec3{0.04, 0.07, 0.76}, noTurn)
	for _, side := range []float64{-1, 1} {
		for i := range 4 {
			m.box(rotateGlass, vec3{0.28 - float64(i)*0.24, 0.76, side * 0.371}, vec3{0.18, 0.18, 0.01}, noTurn)
		}
		m.box(rotateInk, vec3{-0.1, 0.5, side * 0.371}, vec3{1.1, 0.04, 0.01}, noTurn)
		m.box(rotateLamp, vec3{0.661, 0.42, side * 0.22}, vec3{0.01, 0.08, 0.12}, noTurn)
		m.box(rotateRed, vec3{-0.651, 0.4, side * 0.3}, vec3{0.01, 0.08, 0.1}, noTurn)
		m.wheel(vec3{0.38, 0.16, side * 0.34}, 0.32, 0.12, rotateInk)
		m.wheel(vec3{-0.42, 0.16, side * 0.34}, 0.32, 0.12, rotateInk)
	}
	return m
}

// modelTractor has its big wheels at the back under the cab, the small ones under the bonnet.
func modelTractor() rotateMesh {
	var m rotateMesh
	green, yellow := rgb(46, 146, 64), rgb(250, 204, 40)
	m.box(green, vec3{0.22, 0.42, 0}, vec3{0.56, 0.28, 0.32}, noTurn)
	m.box(rotateInk, vec3{0.502, 0.42, 0}, vec3{0.01, 0.2, 0.26}, noTurn)
	m.box(green, vec3{-0.26, 0.4, 0}, vec3{0.44, 0.22, 0.46}, noTurn)
	m.box(rotateGlass, vec3{-0.28, 0.74, 0}, vec3{0.4, 0.46, 0.42}, noTurn)
	m.box(green, vec3{-0.28, 0.985, 0}, vec3{0.46, 0.05, 0.48}, noTurn)
	m.rod(rotateInk, vec3{0.36, 0.56, 0.08}, vec3{0.36, 0.84, 0.08}, 0.05, 1, 6)
	for _, side := range []float64{-1, 1} {
		m.wheel(vec3{-0.28, 0.33, side * 0.33}, 0.66, 0.2, yellow)
		m.wheel(vec3{0.38, 0.17, side * 0.24}, 0.34, 0.12, yellow)
		m.box(rotateLamp, vec3{0.502, 0.5, side * 0.1}, vec3{0.01, 0.05, 0.06}, noTurn)
	}
	return m
}

// modelAirplane is a small propeller plane on its wheels.
func modelAirplane() rotateMesh {
	var m rotateMesh
	body, trim := rgb(240, 240, 236), rgb(216, 50, 46)
	m.rod(body, vec3{0.4, 0.36, 0}, vec3{-0.5, 0.42, 0}, 0.26, 0.4, 12)
	m.ellipsoid(body, vec3{0.4, 0.36, 0}, vec3{0.14, 0.26, 0.26}, noTurn, 6, 12)
	m.rod(trim, vec3{0.46, 0.36, 0}, vec3{0.55, 0.36, 0}, 0.12, 0.3, 8)
	m.box(rotateInk, vec3{0.53, 0.36, 0}, vec3{0.015, 0.44, 0.06}, noTurn)
	m.box(rotateInk, vec3{0.53, 0.36, 0}, vec3{0.015, 0.06, 0.44}, noTurn)
	m.ellipsoid(rotateGlass, vec3{0.14, 0.5, 0}, vec3{0.32, 0.16, 0.18}, noTurn, 6, 10)
	m.box(trim, vec3{0.08, 0.34, 0}, vec3{0.3, 0.04, 1.7}, noTurn)
	m.box(trim, vec3{-0.46, 0.43, 0}, vec3{0.15, 0.03, 0.56}, noTurn)
	m.loft(trim,
		[]vec3{{-0.36, 0.45, -0.015}, {-0.54, 0.45, -0.015}, {-0.56, 0.64, -0.015}, {-0.48, 0.64, -0.015}},
		[]vec3{{-0.36, 0.45, 0.015}, {-0.54, 0.45, 0.015}, {-0.56, 0.64, 0.015}, {-0.48, 0.64, 0.015}})
	for _, side := range []float64{-1, 1} {
		m.rod(rotateInk, vec3{0.12, 0.32, side * 0.2}, vec3{0.12, 0.07, side * 0.22}, 0.03, 1, 5)
		m.wheel(vec3{0.12, 0.07, side * 0.24}, 0.14, 0.05, rotateSteel)
	}
	m.rod(rotateInk, vec3{-0.44, 0.38, 0}, vec3{-0.45, 0.05, 0}, 0.025, 1, 5)
	m.wheel(vec3{-0.45, 0.04, 0.01}, 0.08, 0.03, rotateSteel)
	return m
}

func modelHelicopter() rotateMesh {
	var m rotateMesh
	body := rgb(230, 70, 50)
	m.ellipsoid(body, vec3{0.1, 0.4, 0}, vec3{0.66, 0.46, 0.44}, noTurn, 8, 14)
	m.ellipsoid(rotateGlass, vec3{0.24, 0.45, 0}, vec3{0.42, 0.36, 0.38}, noTurn, 8, 12)
	m.rod(body, vec3{-0.12, 0.46, 0}, vec3{-0.86, 0.54, 0}, 0.13, 0.5, 8)
	m.box(body, vec3{-0.86, 0.64, 0}, vec3{0.14, 0.24, 0.03}, noTurn)
	m.box(rotateInk, vec3{-0.86, 0.62, 0.035}, vec3{0.035, 0.28, 0.02}, noTurn)
	m.rod(rotateInk, vec3{0.08, 0.62, 0}, vec3{0.08, 0.74, 0}, 0.05, 1, 6)
	m.box(rotateInk, vec3{0.08, 0.75, 0}, vec3{1.5, 0.02, 0.07}, heading(28))
	m.box(rotateInk, vec3{0.08, 0.75, 0}, vec3{1.5, 0.02, 0.07}, heading(118))
	for _, side := range []float64{-1, 1} {
		m.rod(rotateSteel, vec3{-0.22, 0.03, side * 0.22}, vec3{0.42, 0.03, side * 0.22}, 0.04, 1, 6)
		m.rod(rotateSteel, vec3{0.24, 0.03, side * 0.22}, vec3{0.2, 0.22, side * 0.16}, 0.03, 1, 5)
		m.rod(rotateSteel, vec3{-0.08, 0.03, side * 0.22}, vec3{-0.06, 0.22, side * 0.16}, 0.03, 1, 5)
	}
	return m
}

// modelBoat is a tugboat: a pointed bow, the wheelhouse forward and the funnel aft.
func modelBoat() rotateMesh {
	var m rotateMesh
	hull, stripe, cabin, funnel := rgb(34, 74, 150), rgb(214, 60, 48), rgb(246, 246, 240), rgb(250, 190, 40)
	m.loft(hull,
		[]vec3{{-0.36, 0, -0.2}, {0.2, 0, -0.18}, {0.38, 0, 0}, {0.2, 0, 0.18}, {-0.36, 0, 0.2}},
		[]vec3{{-0.4, 0.3, -0.33}, {0.2, 0.3, -0.33}, {0.5, 0.34, 0}, {0.2, 0.3, 0.33}, {-0.4, 0.3, 0.33}})
	m.loft(stripe,
		[]vec3{{-0.395, 0.25, -0.32}, {0.2, 0.25, -0.32}, {0.49, 0.28, 0}, {0.2, 0.25, 0.32}, {-0.395, 0.25, 0.32}},
		[]vec3{{-0.41, 0.31, -0.34}, {0.2, 0.31, -0.34}, {0.52, 0.35, 0}, {0.2, 0.31, 0.34}, {-0.41, 0.31, 0.34}})
	m.box(cabin, vec3{0.02, 0.5, 0}, vec3{0.34, 0.4, 0.38}, noTurn)
	m.box(rotateGlass, vec3{0.191, 0.6, 0}, vec3{0.01, 0.12, 0.3}, noTurn)
	m.box(stripe, vec3{0.02, 0.715, 0}, vec3{0.4, 0.03, 0.44}, noTurn)
	m.rod(funnel, vec3{-0.2, 0.3, 0}, vec3{-0.23, 0.78, 0}, 0.16, 1, 10)
	m.rod(rotateInk, vec3{-0.2275, 0.74, 0}, vec3{-0.231, 0.8, 0}, 0.165, 1, 10)
	return m
}

// modelTrainEngine is a steam engine: the chimney and cowcatcher in front, the cab behind.
func modelTrainEngine() rotateMesh {
	var m rotateMesh
	boiler, cab, brass := rgb(40, 120, 74), rgb(206, 52, 44), rgb(226, 182, 70)
	m.box(rotateInk, vec3{0.02, 0.2, 0}, vec3{0.98, 0.1, 0.5}, noTurn)
	m.cylinder(boiler, vec3{0.15, 0.44, 0}, vec3{0.4, 0.56, 0.4}, pitch(-90), 1, 14)
	m.cylinder(rotateInk, vec3{0.44, 0.44, 0}, vec3{0.36, 0.04, 0.36}, pitch(-90), 1, 14)
	m.cylinder(rotateInk, vec3{0.33, 0.74, 0}, vec3{0.15, 0.26, 0.15}, noTurn, 1.35, 10)
	m.ellipsoid(brass, vec3{0.08, 0.66, 0}, vec3{0.14, 0.12, 0.14}, noTurn, 5, 8)
	m.cylinder(rotateLamp, vec3{0.47, 0.6, 0}, vec3{0.08, 0.04, 0.08}, pitch(-90), 1, 8)
	m.box(cab, vec3{-0.28, 0.56, 0}, vec3{0.38, 0.62, 0.56}, noTurn)
	m.box(rotateInk, vec3{-0.28, 0.885, 0}, vec3{0.46, 0.04, 0.62}, noTurn)
	m.wedge(rotateSteel, vec3{0.56, 0.12, 0}, vec3{0.14, 0.2, 0.46}, noTurn)
	for _, side := range []float64{-1, 1} {
		m.box(rotateGlass, vec3{-0.28, 0.7, side * 0.281}, vec3{0.2, 0.14, 0.01}, noTurn)
		m.box(rotateGlass, vec3{-0.089, 0.72, side * 0.13}, vec3{0.01, 0.12, 0.12}, noTurn)
		for _, x := range []float64{-0.3, 0, 0.28} {
			m.wheel(vec3{x, 0.15, side * 0.27}, 0.28, 0.06, cab)
		}
	}
	return m
}

// modelTank points its gun to the front, a toy tank with a tall turret.
func modelTank() rotateMesh {
	var m rotateMesh
	olive, dark := rgb(112, 128, 72), rgb(84, 98, 56)
	for _, side := range []float64{-1, 1} {
		m.box(rotateTyre, vec3{0, 0.15, side * 0.27}, vec3{0.74, 0.3, 0.18}, noTurn)
		for _, x := range []float64{0.37, -0.37} {
			m.cylinder(rotateTyre, vec3{x, 0.15, side * 0.27}, vec3{0.3, 0.18, 0.3}, roll(90), 1, 12)
		}
		for i := range 4 {
			m.cylinder(dark, vec3{-0.27 + float64(i)*0.18, 0.13, side * 0.365}, vec3{0.15, 0.02, 0.15}, roll(90), 1, 10)
		}
	}
	m.box(olive, vec3{-0.03, 0.38, 0}, vec3{0.7, 0.18, 0.56}, noTurn)
	m.wedge(olive, vec3{0.4, 0.38, 0}, vec3{0.16, 0.18, 0.56}, noTurn)
	m.cylinder(dark, vec3{-0.05, 0.58, 0}, vec3{0.48, 0.24, 0.46}, noTurn, 0.82, 12)
	m.rod(dark, vec3{0.12, 0.6, 0}, vec3{0.56, 0.62, 0}, 0.08, 0.85, 8)
	m.rod(dark, vec3{0.53, 0.618, 0}, vec3{0.6, 0.62, 0}, 0.1, 1, 8)
	m.cylinder(olive, vec3{-0.13, 0.72, 0.08}, vec3{0.14, 0.06, 0.14}, noTurn, 1, 8)
	return m
}

// modelDeskFan blows to the front: blades on the face of the guard, the motor behind.
func modelDeskFan() rotateMesh {
	var m rotateMesh
	body, blade := rgb(240, 240, 236), rgb(70, 150, 230)
	m.cylinder(body, vec3{-0.04, 0.03, 0}, vec3{0.46, 0.06, 0.46}, noTurn, 0.9, 16)
	m.rod(body, vec3{-0.06, 0.05, 0}, vec3{-0.06, 0.52, 0}, 0.07, 1, 8)
	m.ellipsoid(body, vec3{-0.16, 0.6, 0}, vec3{0.22, 0.2, 0.2}, noTurn, 6, 10)
	m.cylinder(rotateSteel, vec3{0.02, 0.6, 0}, vec3{0.64, 0.04, 0.64}, pitch(-90), 1, 20)
	for i := range 3 {
		spin := roll(float64(i)*120 + 20)
		m.box(blade, vec3{0.055, 0.6, 0}.add(spin.apply(vec3{0, 0.15, 0})), vec3{0.02, 0.26, 0.13}, spin)
	}
	m.ellipsoid(rotateInk, vec3{0.07, 0.6, 0}, vec3{0.06, 0.1, 0.1}, noTurn, 4, 8)
	return m
}

// modelDeskLamp shines to the front: its shade opens forward and down over the base.
func modelDeskLamp() rotateMesh {
	var m rotateMesh
	metal, shade, bulb := rgb(60, 62, 70), rgb(222, 64, 52), rgb(255, 240, 170)
	m.cylinder(metal, vec3{0.06, 0.03, 0}, vec3{0.4, 0.06, 0.4}, noTurn, 0.85, 14)
	m.rod(metal, vec3{0.06, 0.05, 0}, vec3{-0.14, 0.5, 0}, 0.05, 1, 6)
	m.ellipsoid(metal, vec3{-0.14, 0.5, 0}, vec3{0.08, 0.08, 0.08}, noTurn, 4, 8)
	m.rod(metal, vec3{-0.14, 0.5, 0}, vec3{0.1, 0.8, 0}, 0.05, 1, 6)
	apex, mouth := vec3{0.08, 0.8, 0}, vec3{0.3, 0.6, 0}
	axis := mouth.sub(apex).unit()
	m.rod(shade, mouth, apex, 0.34, 0.25, 12)
	m.rod(bulb, mouth.add(axis.scale(0.01)), mouth.sub(axis.scale(0.04)), 0.3, 1, 12)
	return m
}

// modelTeapot faces where its spout points.
func modelTeapot() rotateMesh {
	var m rotateMesh
	glaze, white := rgb(44, 150, 168), rgb(250, 250, 246)
	m.cylinder(glaze, vec3{0, 0.03, 0}, vec3{0.38, 0.06, 0.38}, noTurn, 1, 14)
	m.ellipsoid(glaze, vec3{0, 0.33, 0}, vec3{0.64, 0.54, 0.64}, noTurn, 10, 16)
	m.ellipsoid(white, vec3{0, 0.36, 0}, vec3{0.65, 0.08, 0.65}, noTurn, 4, 16)
	m.cylinder(glaze, vec3{0, 0.6, 0}, vec3{0.32, 0.06, 0.32}, noTurn, 0.8, 14)
	m.ellipsoid(white, vec3{0, 0.67, 0}, vec3{0.09, 0.08, 0.09}, noTurn, 4, 8)
	m.rod(glaze, vec3{0.2, 0.24, 0}, vec3{0.5, 0.54, 0}, 0.12, 0.5, 8)
	m.rod(glaze, vec3{-0.26, 0.5, 0}, vec3{-0.44, 0.46, 0}, 0.06, 1, 6)
	m.rod(glaze, vec3{-0.44, 0.46, 0}, vec3{-0.44, 0.22, 0}, 0.06, 1, 6)
	m.rod(glaze, vec3{-0.44, 0.22, 0}, vec3{-0.28, 0.18, 0}, 0.06, 1, 6)
	return m
}

// modelSneaker is a high-top: the toe to the front, the ankle at the back.
func modelSneaker() rotateMesh {
	var m rotateMesh
	upper, sole := rgb(216, 54, 54), rgb(246, 246, 242)
	m.cylinder(sole, vec3{0.02, 0.04, 0}, vec3{0.84, 0.08, 0.4}, noTurn, 1, 18)
	m.ellipsoid(upper, vec3{0.08, 0.14, 0}, vec3{0.66, 0.28, 0.36}, noTurn, 8, 14)
	m.cylinder(upper, vec3{-0.16, 0.34, 0}, vec3{0.4, 0.56, 0.34}, noTurn, 0.9, 14)
	m.cylinder(rotateInk, vec3{-0.16, 0.62, 0}, vec3{0.28, 0.02, 0.24}, noTurn, 1, 14)
	m.ellipsoid(sole, vec3{0.3, 0.1, 0}, vec3{0.22, 0.17, 0.34}, noTurn, 6, 10)
	for i := range 4 {
		at := lerp3(vec3{0.2, 0.27, 0}, vec3{0.04, 0.54, 0}, float64(i)/3)
		m.box(rotateChalk, at, vec3{0.03, 0.025, 0.2 - float64(i)*0.01}, pitch(-40))
	}
	for _, side := range []float64{-1, 1} {
		m.cylinder(rotateChalk, vec3{-0.18, 0.42, side * 0.165}, vec3{0.14, 0.02, 0.14}, roll(90), 1, 12)
	}
	return m
}

// modelArmchair is a club chair: a low back behind the seat, high arms either side.
func modelArmchair() rotateMesh {
	var m rotateMesh
	fabric, cushion, back, wood := rgb(60, 110, 176), rgb(96, 146, 206), rgb(44, 82, 140), rgb(120, 78, 44)
	m.box(fabric, vec3{0.04, 0.24, 0}, vec3{0.56, 0.2, 0.62}, noTurn)
	m.box(cushion, vec3{0.07, 0.38, 0}, vec3{0.5, 0.08, 0.46}, noTurn)
	m.box(back, vec3{-0.2, 0.54, 0}, vec3{0.12, 0.52, 0.68}, pitch(8))
	m.box(cushion, vec3{-0.13, 0.56, 0}, vec3{0.05, 0.36, 0.46}, pitch(8))
	for _, side := range []float64{-1, 1} {
		m.box(fabric, vec3{0.02, 0.48, side * 0.3}, vec3{0.6, 0.28, 0.12}, noTurn)
		for _, x := range []float64{0.26, -0.22} {
			m.cylinder(wood, vec3{x, 0.07, side * 0.26}, vec3{0.06, 0.14, 0.06}, noTurn, 0.8, 6)
		}
	}
	return m
}

// modelOfficeChair has its cushions to the front and the bare shell of the back behind.
func modelOfficeChair() rotateMesh {
	var m rotateMesh
	frame, pad := rgb(46, 48, 54), rgb(70, 110, 190)
	for i := range 5 {
		leg := heading(float64(i)*72 + 36)
		m.box(frame, leg.apply(vec3{0.15, 0.06, 0}), vec3{0.3, 0.04, 0.05}, leg)
		m.ellipsoid(rotateInk, leg.apply(vec3{0.29, 0.03, 0}), vec3{0.06, 0.06, 0.05}, leg, 4, 6)
	}
	m.rod(rotateSteel, vec3{0, 0.06, 0}, vec3{0, 0.4, 0}, 0.06, 1, 8)
	m.box(frame, vec3{0.02, 0.42, 0}, vec3{0.46, 0.05, 0.46}, noTurn)
	m.box(pad, vec3{0.03, 0.47, 0}, vec3{0.46, 0.06, 0.44}, noTurn)
	m.box(frame, vec3{-0.22, 0.52, 0}, vec3{0.04, 0.14, 0.08}, noTurn)
	m.box(frame, vec3{-0.24, 0.8, 0}, vec3{0.05, 0.46, 0.42}, pitch(6))
	m.box(pad, vec3{-0.21, 0.8, 0}, vec3{0.03, 0.4, 0.36}, pitch(6))
	for _, side := range []float64{-1, 1} {
		m.box(frame, vec3{-0.04, 0.56, side * 0.24}, vec3{0.04, 0.16, 0.04}, noTurn)
		m.box(frame, vec3{0.02, 0.645, side * 0.24}, vec3{0.24, 0.03, 0.07}, noTurn)
	}
	return m
}

// modelWateringCan faces where its spout points.
func modelWateringCan() rotateMesh {
	var m rotateMesh
	can, rose := rgb(60, 162, 92), rgb(250, 196, 40)
	m.cylinder(can, vec3{-0.04, 0.24, 0}, vec3{0.48, 0.48, 0.44}, noTurn, 0.92, 16)
	m.ellipsoid(can, vec3{-0.04, 0.48, 0}, vec3{0.44, 0.1, 0.4}, noTurn, 4, 14)
	m.rod(can, vec3{0.12, 0.1, 0}, vec3{0.42, 0.48, 0}, 0.07, 0.8, 8)
	m.rod(rose, vec3{0.41, 0.47, 0}, vec3{0.47, 0.54, 0}, 0.08, 1.8, 10)
	m.rod(can, vec3{-0.3, 0.38, 0}, vec3{-0.28, 0.62, 0}, 0.05, 1, 6)
	m.rod(can, vec3{-0.28, 0.62, 0}, vec3{0, 0.6, 0}, 0.05, 1, 6)
	m.rod(can, vec3{0, 0.6, 0}, vec3{0.04, 0.5, 0}, 0.05, 1, 6)
	return m
}

// modelCamera faces where its lens points; its screen is on the back.
func modelCamera() rotateMesh {
	var m rotateMesh
	body, trim := rgb(44, 44, 50), rgb(196, 198, 204)
	m.box(body, vec3{0, 0.23, 0}, vec3{0.32, 0.46, 0.82}, noTurn)
	m.box(trim, vec3{0, 0.48, 0}, vec3{0.33, 0.06, 0.83}, noTurn)
	m.loft(body,
		[]vec3{{-0.12, 0.5, -0.14}, {0.12, 0.5, -0.14}, {0.12, 0.5, 0.14}, {-0.12, 0.5, 0.14}},
		[]vec3{{-0.08, 0.64, -0.1}, {0.06, 0.64, -0.1}, {0.06, 0.64, 0.1}, {-0.08, 0.64, 0.1}})
	m.cylinder(trim, vec3{0.2, 0.25, 0}, vec3{0.36, 0.08, 0.36}, pitch(-90), 1, 16)
	m.cylinder(rotateInk, vec3{0.34, 0.25, 0}, vec3{0.32, 0.22, 0.32}, pitch(-90), 1, 16)
	m.cylinder(trim, vec3{0.44, 0.25, 0}, vec3{0.34, 0.03, 0.34}, pitch(-90), 1, 16)
	m.cylinder(rgb(60, 80, 130), vec3{0.455, 0.25, 0}, vec3{0.24, 0.01, 0.24}, pitch(-90), 1, 16)
	m.cylinder(rotateRed, vec3{0.02, 0.52, 0.3}, vec3{0.07, 0.04, 0.07}, noTurn, 1, 8)
	m.box(rotateChalk, vec3{0.161, 0.4, -0.28}, vec3{0.01, 0.08, 0.14}, noTurn)
	m.box(rgb(70, 84, 110), vec3{-0.161, 0.25, 0}, vec3{0.01, 0.26, 0.42}, noTurn)
	return m
}
