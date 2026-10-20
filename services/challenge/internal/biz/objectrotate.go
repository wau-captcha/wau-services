package biz

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
	mrand "math/rand/v2"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// One object_rotate round: a 3D object stands on the ground, an arrow on the ground points in one
// of 8 directions, and the person turns the object a step at a time until its front points the way
// the arrow does. A round is a strip of rotateFrames frames side by side, frame k the object turned
// k steps clockwise as seen from above. Each frame shows two panels, seen from the same camera: the
// arrow alone on the left and the object alone on the right, so the arrow never covers the object.
// Sizes are CSS pixels of one 300 × 150 frame.
const (
	rotateFrameWidth, rotateFrameHeight = 300, 150
	rotateInset                         = 6  // backdrop around and between the panels
	rotateArrowPanelWidth               = 96 // the object's panel takes the rest
	rotatePanelRadius                   = 10
	rotateFrames                        = 8
	rotateStep                          = 360.0 / rotateFrames
	rotateRounds                        = 2 // a random final frame is right 1 time in 8; two in a row, 1 in 64
	// Output pixels per display pixel: at 2× both strips together stay well under 400 KB, about 200
	// (TestObjectRotateChallenges), so 2× it is.
	rotateScale       = 2
	rotateJPEGQuality = 80
	// The object fills this share of its panel's height in every frame; one too wide for that at
	// some heading is made smaller, down to rotateLeastFill.
	rotateMinFill, rotateMaxFill = 0.55, 0.75
	rotateLeastFill              = 0.4
	// Looking down this steeply sets the eight directions apart, the diagonals included.
	rotateMinElevation, rotateMaxElevation = 20, 32
	// The camera turns this far at most off the view square to the side, and each frame up to
	// rotateJitter off its step, so that no two challenges show the same eight views.
	rotateAzimuthSpread = 10
	rotateJitter        = 5
	// Each colour channel of a round is scaled by up to this much either way.
	rotateTint = 0.05
)

// rotatePlan is what a round is about: which object, where frame 0 faces (a step from facing
// right on screen), the frame that faces the arrow, and the camera.
type rotatePlan struct {
	model, start, target int
	elevation, azimuth   float64
}

// The panels of a frame, in display pixels.
var (
	rotateArrowPanel  = rotateBounds{rotateInset, rotateInset, rotateInset + rotateArrowPanelWidth, rotateFrameHeight - rotateInset}
	rotateObjectPanel = rotateBounds{2*rotateInset + rotateArrowPanelWidth, rotateInset, rotateFrameWidth - rotateInset, rotateFrameHeight - rotateInset}
)

// rotateRound is a plan with everything else drawn for it.
type rotateRound struct {
	plan        rotatePlan
	headings    [rotateFrames]float64
	arrow       float64
	tint        [3]float64
	light       vec3
	layout      rotateLayout // of the object's panel
	arrowLayout rotateLayout // of the arrow's panel
	arrowPaint  color.RGBA
}

// randomRotatePlan draws a round's plan; the object differs from avoid, the other round's.
func randomRotatePlan(random *mrand.Rand, avoid int) rotatePlan {
	model := random.IntN(len(rotateModels) - 1)
	if avoid >= 0 && model >= avoid {
		model++
	}
	return rotatePlan{
		model: model, start: random.IntN(rotateFrames), target: 1 + random.IntN(rotateFrames-1),
		elevation: between(random, rotateMinElevation, rotateMaxElevation),
		azimuth:   between(random, -rotateAzimuthSpread, rotateAzimuthSpread),
	}
}

func newRotateRound(random *mrand.Rand, plan rotatePlan) (rotateRound, error) {
	round := rotateRound{plan: plan}
	camera := newRotateCamera(plan.elevation, plan.azimuth)
	// Heading -azimuth faces right on screen; frame 0 starts plan.start steps clockwise from it.
	first := -plan.azimuth + float64(plan.start)*rotateStep
	for k := range round.headings {
		round.headings[k] = first + float64(k)*rotateStep + between(random, -rotateJitter, rotateJitter)
	}
	round.arrow = first + float64(plan.target)*rotateStep
	for c := range round.tint {
		round.tint[c] = 1 + (random.Float64()*2-1)*rotateTint
	}
	// The light comes from one upper side of the camera, so the faces it sees are lit.
	side := between(random, 25, 75)
	if random.IntN(2) == 0 {
		side = -side
	}
	lightTurn := heading(-(plan.azimuth + side))
	up, out := math.Sincos(radians(between(random, 40, 65)))
	round.light = lightTurn.apply(vec3{0, 0, 1}).scale(out).add(vec3{0, up, 0}).unit()
	model := &rotateModels[plan.model]
	layout, ok := fitRotate(random, camera, model, round.headings[:])
	if !ok {
		return rotateRound{}, fmt.Errorf("object_rotate: %s does not fit its panel", model.name)
	}
	round.layout = layout
	round.arrowLayout = fitArrow(camera, round.arrow)
	round.arrowPaint = arrowColor(random, model)
	return round, nil
}

// renderRotateStrip draws the first frames of a round side by side, scale output pixels per display pixel.
func renderRotateStrip(random *mrand.Rand, round rotateRound, frames int, scale float64) *image.RGBA {
	w, h := int(rotateFrameWidth*scale), int(rotateFrameHeight*scale)
	scene := rotateScene{
		camera: newRotateCamera(round.plan.elevation, round.plan.azimuth), layout: round.layout, arrowLayout: round.arrowLayout,
		scale: scale, light: round.light, model: &rotateModels[round.plan.model], tint: round.tint,
		base: image.NewRGBA(image.Rect(0, 0, w, h)),
	}
	scene.drawBackground(random)
	scene.drawArrow(round.arrow, round.arrowPaint)
	strip := image.NewRGBA(image.Rect(0, 0, w*frames, h))
	frame := image.NewRGBA(image.Rect(0, 0, w, h))
	var raster rotateRaster
	for k := range frames {
		copy(frame.Pix, scene.base.Pix)
		scene.drawFrame(frame, &raster, round.headings[k])
		draw.Draw(strip, image.Rect(k*w, 0, (k+1)*w, h), frame, image.Point{}, draw.Src)
	}
	return strip
}

// rotateTarget is what the service stores of one round: the frame that faces the arrow, and the
// object's name for the samples. It is never sent to the client.
type rotateTarget struct {
	Target int    `json:"t"`
	Object string `json:"o"`
}

type rotateAnswer struct {
	R []rotateTarget `json:"r"`
}

// generateObjectRotate draws both rounds, with different objects; the stored answer holds each
// round's target frame.
func generateObjectRotate() (string, Media, error) {
	random := newRandomSource()
	var answer rotateAnswer
	var strips [rotateRounds][]byte
	avoid := -1
	for i := range strips {
		plan := randomRotatePlan(random, avoid)
		avoid = plan.model
		round, err := newRotateRound(random, plan)
		if err != nil {
			return "", Media{}, err
		}
		var encoded bytes.Buffer
		if err := jpeg.Encode(&encoded, renderRotateStrip(random, round, rotateFrames, rotateScale), &jpeg.Options{Quality: rotateJPEGQuality}); err != nil {
			return "", Media{}, fmt.Errorf("object_rotate round %d: %w", i+1, err)
		}
		strips[i] = encoded.Bytes()
		answer.R = append(answer.R, rotateTarget{Target: plan.target, Object: rotateModels[plan.model].name})
	}
	stored, err := json.Marshal(answer)
	if err != nil {
		return "", Media{}, err
	}
	return string(stored), Media{
		Data: strips[0], SecondRound: strips[1], MIMEType: "image/jpeg",
		Width: rotateFrameWidth, Height: rotateFrameHeight, Frames: rotateFrames,
	}, nil
}

// ObjectRotateSheet draws every object in each of the eight directions, the arrow pointing where it
// faces, one row per object, at display size: for looking over the models (cmd/samples -rotate-sheet).
func ObjectRotateSheet() (*image.RGBA, error) {
	const label = 150
	random := mrand.New(mrand.NewPCG(1, 2))
	sheet := image.NewRGBA(image.Rect(0, 0, label+rotateFrames*rotateFrameWidth, len(rotateModels)*rotateFrameHeight))
	draw.Draw(sheet, sheet.Bounds(), image.White, image.Point{}, draw.Src)
	face, err := opentype.NewFace(codeFont, &opentype.FaceOptions{Size: 18, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, err
	}
	for m := range rotateModels {
		writer := font.Drawer{Dst: sheet, Src: image.Black, Face: face, Dot: fixed.P(10, m*rotateFrameHeight+rotateFrameHeight/2+6)}
		writer.DrawString(rotateModels[m].name)
		for direction := range rotateFrames {
			round, err := newRotateRound(random, rotatePlan{model: m, start: direction, elevation: 26})
			if err != nil {
				return nil, err
			}
			cell := renderRotateStrip(random, round, 1, 1)
			at := image.Pt(label+direction*rotateFrameWidth, m*rotateFrameHeight)
			draw.Draw(sheet, cell.Bounds().Add(at), cell, image.Point{}, draw.Src)
		}
	}
	return sheet, nil
}
