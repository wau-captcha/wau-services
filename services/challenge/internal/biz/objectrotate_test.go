package biz

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	mrand "math/rand/v2"
	"runtime"
	"sort"
	"sync"
	"testing"
)

// TestObjectRotateChallenges makes 1000 challenges as /challenge/new does and checks what each one
// stores and sends: targets in 1..7, two different objects, strips of eight frames at 2×, and both
// strips together under 400 KB, the budget that lets rotateScale be 2.
func TestObjectRotateChallenges(t *testing.T) {
	count := 1000
	if testing.Short() {
		count = 50
	}
	const budget = 400 << 10
	known := map[string]bool{}
	for _, model := range rotateModels {
		known[model.name] = true
	}
	var (
		lock    sync.Mutex
		sizes   []int
		targets [rotateFrames]int
		objects = map[string]int{}
	)
	work := make(chan int)
	var workers sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		workers.Go(func() {
			for i := range work {
				answer, media, err := generateObjectRotate()
				if err != nil {
					t.Error(err)
					continue
				}
				var stored rotateAnswer
				if err := json.Unmarshal([]byte(answer), &stored); err != nil || len(stored.R) != rotateRounds {
					t.Errorf("stored answer %q: %v", answer, err)
					continue
				}
				if stored.R[0].Object == stored.R[1].Object || !known[stored.R[0].Object] || !known[stored.R[1].Object] {
					t.Errorf("objects %q and %q", stored.R[0].Object, stored.R[1].Object)
				}
				if media.MIMEType != "image/jpeg" || media.Width != rotateFrameWidth || media.Height != rotateFrameHeight ||
					media.Frames != rotateFrames || media.Piece != nil || media.SecondRound == nil {
					t.Errorf("media %+v", media)
					continue
				}
				want := image.Pt(rotateFrames*rotateFrameWidth*rotateScale, rotateFrameHeight*rotateScale)
				for _, strip := range [][]byte{media.Data, media.SecondRound} {
					// Every strip's header, and one challenge in ten decoded whole.
					config, err := jpeg.DecodeConfig(bytes.NewReader(strip))
					if err == nil && i%10 == 0 {
						_, err = jpeg.Decode(bytes.NewReader(strip))
					}
					if err != nil {
						t.Error(err)
					} else if got := image.Pt(config.Width, config.Height); got != want {
						t.Errorf("strip of %v, want %v: eight frames of 300 × 150 at 2×", got, want)
					}
				}
				size := len(media.Data) + len(media.SecondRound)
				if size >= budget {
					t.Errorf("both strips %d KB, over the 400 KB that 2× is allowed", size>>10)
				}
				lock.Lock()
				sizes = append(sizes, size)
				for _, round := range stored.R {
					if round.Target < 1 || round.Target >= rotateFrames {
						t.Errorf("target frame %d, want 1 to 7", round.Target)
					} else {
						targets[round.Target]++
					}
					objects[round.Object]++
				}
				lock.Unlock()
			}
		})
	}
	for i := range count {
		work <- i
	}
	close(work)
	workers.Wait()
	if t.Failed() {
		return
	}
	if !testing.Short() {
		// 2000 rounds: about 286 per target frame and 71 per object.
		for frame := 1; frame < rotateFrames; frame++ {
			if targets[frame] < 200 || targets[frame] > 380 {
				t.Errorf("target frame %d drawn %d times in %d rounds", frame, targets[frame], 2*count)
			}
		}
		if len(objects) != len(rotateModels) {
			t.Errorf("%d of the %d objects drawn in %d rounds", len(objects), len(rotateModels), 2*count)
		}
	}
	sort.Ints(sizes)
	t.Logf("both strips: %d KB at least, %d KB median, %d KB at most", sizes[0]>>10, sizes[len(sizes)/2]>>10, sizes[len(sizes)-1]>>10)
}

// TestRotateModelsFit puts every object through rounds of every kind, at the lowest, middle and
// highest elevation, the camera straight and at either side, frame 0 in every direction and the
// arrow in every other: in all eight frames the object fills 55-75% of the frame's height, and the
// object and the arrow stay inside the frame. The bounds fitRotate works from are checked for
// every round; the drawn pixels, for rounds at the extremes, must agree with them.
func TestRotateModelsFit(t *testing.T) {
	if len(rotateModels) < 24 {
		t.Fatalf("%d objects, want at least 24", len(rotateModels))
	}
	names := map[string]bool{}
	for _, model := range rotateModels {
		if model.name == "" || names[model.name] {
			t.Fatalf("object name %q missing or repeated", model.name)
		}
		names[model.name] = true
	}
	random := mrand.New(mrand.NewPCG(3, 4))
	w, h := rotateFrameWidth*rotateScale, rotateFrameHeight*rotateScale
	// In output pixels; a pixel of slack for the edges the supersampling softens.
	inside := func(box image.Rectangle) bool {
		margin := rotateMargin*rotateScale - 1
		return box.Min.X >= margin && box.Min.Y >= margin && box.Max.X <= w-margin && box.Max.Y <= h-margin
	}
	fills := func(box image.Rectangle) bool {
		fill := float64(box.Dy()) / float64(h)
		return fill >= rotateMinFill-1.0/rotateFrameHeight && fill <= rotateMaxFill+1.0/rotateFrameHeight
	}
	onFrame := func(layout rotateLayout, b rotateBounds) image.Rectangle {
		x0, y0 := layout.pixel(b.x0, b.y1)
		x1, y1 := layout.pixel(b.x1, b.y0)
		return image.Rect(int(x0*rotateScale), int(y0*rotateScale), int(x1*rotateScale+0.999), int(y1*rotateScale+0.999))
	}
	var raster rotateRaster
	drawn := 0
	for m := range rotateModels {
		model := &rotateModels[m]
		for _, elevation := range []float64{rotateMinElevation, (rotateMinElevation + rotateMaxElevation) / 2, rotateMaxElevation} {
			for _, azimuth := range []float64{-rotateAzimuthSpread, 0, rotateAzimuthSpread} {
				camera := newRotateCamera(elevation, azimuth)
				for start := range rotateFrames {
					for target := 1; target < rotateFrames; target++ {
						plan := rotatePlan{model: m, start: start, target: target, elevation: elevation, azimuth: azimuth}
						round, err := newRotateRound(random, plan)
						if err != nil {
							t.Fatalf("%+v: %v", plan, err)
						}
						if box := onFrame(round.layout, shapeBounds(camera, arrowShape(model, round.arrow, round.share, true))); !inside(box) {
							t.Errorf("%s %+v: the arrow at %v leaves the frame", model.name, plan, box)
						}
						for k, degrees := range round.headings {
							if box := onFrame(round.layout, modelBounds(camera, model, degrees)); !fills(box) || !inside(box) {
								t.Errorf("%s %+v frame %d: at %v, %.3f of the height", model.name, plan, k, box, float64(box.Dy())/float64(h))
							}
						}
						// Drawing every round would take minutes: two per object at each extreme.
						extreme := elevation != (rotateMinElevation+rotateMaxElevation)/2 && azimuth != 0
						if !extreme || start != m%rotateFrames && start != (m+3)%rotateFrames || target != 1+(start+m)%(rotateFrames-1) {
							continue
						}
						drawn++
						scene := rotateScene{
							camera: camera, layout: round.layout, scale: rotateScale, light: round.light,
							model: model, tint: round.tint, base: image.NewRGBA(image.Rect(0, 0, w, h)),
						}
						scene.drawArrow(round.arrow, round.share, round.arrowPaint)
						if !inside(scene.arrowRect) {
							t.Errorf("%s %+v: the arrow drawn at %v leaves the frame", model.name, plan, scene.arrowRect)
						}
						for k, degrees := range round.headings {
							if !scene.rasterize(&raster, degrees, w, h) {
								t.Fatalf("%s %+v frame %d: nothing drawn", model.name, plan, k)
							}
							box := raster.covered()
							if !fills(box) || !inside(box) {
								t.Errorf("%s %+v frame %d: drawn at %v, %.3f of the height", model.name, plan, k, box, float64(box.Dy())/float64(h))
							}
							if want := onFrame(round.layout, modelBounds(camera, model, degrees)); !box.In(want.Inset(-1)) || !want.Inset(2).In(box) {
								t.Errorf("%s %+v frame %d: drawn at %v, its bounds %v", model.name, plan, k, box, want)
							}
						}
					}
				}
			}
		}
	}
	t.Logf("%d objects in %d rounds each; %d rounds drawn", len(rotateModels), 3*3*rotateFrames*(rotateFrames-1), drawn)
}

// covered is the box of output pixels the raster drew any of the model into.
func (r *rotateRaster) covered() image.Rectangle {
	var box image.Rectangle
	for i, z := range r.depth {
		if z > 0 {
			x, y := (r.x0+i%r.w)/rotateSupersample, (r.y0+i/r.w)/rotateSupersample
			box = box.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	return box
}

// BenchmarkObjectRotate makes one whole challenge: sixteen frames drawn and two strips encoded.
// Run with -cpu 1; it must stay under 150 ms a challenge.
func BenchmarkObjectRotate(b *testing.B) {
	for b.Loop() {
		if _, _, err := generateObjectRotate(); err != nil {
			b.Fatal(err)
		}
	}
}
