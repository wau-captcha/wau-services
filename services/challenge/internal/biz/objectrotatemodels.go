package biz

import "image/color"

// The objects of object_rotate: boxy vehicles and furniture, built mostly from boxes whose sharp
// edges show which way they face, each with an unmistakable front; round objects such as animals
// read alike from too many sides. In a
// model's own space +X is its front, +Y up and +Z its right, the ground at y = 0; newRotateModel
// scales each to stand 1 tall, so sizes here are only relative. Each is kept about as deep as it is
// tall from every side, toy-like, so that it fills the frame alike in all eight directions.
var rotateModels = []rotateModel{
	newRotateModel("car", modelCar()),
	newRotateModel("truck", modelTruck()),
	newRotateModel("bus", modelBus()),
	newRotateModel("tractor", modelTractor()),
	newRotateModel("boat", modelBoat()),
	newRotateModel("train engine", modelTrainEngine()),
	newRotateModel("tank", modelTank()),
	newRotateModel("armchair", modelArmchair()),
	newRotateModel("office chair", modelOfficeChair()),
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

// wheel is a tyre turning about the Z axis with its hub on the outer side.
func (m *rotateMesh) wheel(at vec3, diameter, width float64, hub color.RGBA) {
	m.cylinder(rotateTyre, at, vec3{diameter, width, diameter}, roll(90), 1, 14)
	side := 1.0
	if at.z < 0 {
		side = -1
	}
	m.cylinder(hub, at.add(vec3{0, 0, side * width / 2}), vec3{diameter * 0.5, 0.02, diameter * 0.5}, roll(90), 1, 10)
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
