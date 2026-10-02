package biz

import (
	"encoding/json"
	"math"
	"strconv"
	"time"
)

// Every threshold of the slide_puzzle check. The values come from synthetic drags and how pointer
// movement is known to look; tune them once real traffic shows how people and bots score.
const (
	// Hard limits: an answer that breaks one is wrong, however it moved.
	slideMaxAnswerBytes = 16 << 10 // the SDK sends at most slideMaxPoints points, about 9 KiB
	slideMinPoints      = 8        // fewer say nothing about how the piece moved
	slideMaxPoints      = 400      // the SDK downsamples longer drags to this
	slideStartSlackMs   = 50.0     // the first point is the pointerdown, at t = 0
	slideStartSlackPx   = 2.0      // the piece starts at x = 0
	slideEndSlackPx     = 1.0      // "x" is where the last point left the piece
	slideAimSlackPx     = 4.0      // people line the piece up within a pixel or two of the hole
	slideMinDragMs      = 250.0    // faster than a hand can aim
	slideMaxDragMs      = 20000.0
	// From issue to check: loading the image, finding the hole and dragging take longer.
	slideMinSolveTime = 500 * time.Millisecond

	// Behaviour signals: a pass needs slideMinSignals of the five, measured on the movement only
	// (see movement).
	slideMinSignals = 3
	// Coefficient of variation of the speed between points. A hand speeds up and brakes: even the
	// smoothest, bell-shaped speed profile scores about 0.65. A script moving at constant speed
	// through a real browser scores 0.02-0.3, from the jitter of event timing alone.
	slideMinSpeedVariation = 0.4
	// Mean speed over the last 25% of the distance ÷ over the middle 50%. People brake to aim
	// (Fitts's law) and correct at the end; a linear or eased-in script does not.
	slideMaxEndSpeedRatio = 0.8
	// R² of a straight-line fit of x against t. A constant-speed tween fits it exactly; the S-curve
	// of a real drag, with its braking and corrections, does not.
	slideMaxLinearFit = 0.995
	// The pointer drifts up and down as the hand moves sideways; scripts tend to keep y at 0. It
	// counts when y spans 2 whole pixels or moves at least this much (high-DPI screens report
	// fractions).
	slideMinYSpreadPx = 0.3
	// Longest minus shortest interval between points. Browsers deliver pointer events unevenly; a
	// loop with a fixed step produces equal intervals.
	slideMinIntervalSpreadMs = 0.5
)

// slideVerdict is the outcome of checkSlide. reason names the first hard limit broken, or is
// empty; signals counts the behaviour signals seen. Both are for tests and tuning.
type slideVerdict struct {
	pass    bool
	reason  string
	signals int
}

type slidePoint struct{ t, x, y float64 }

// checkSlidePuzzle checks a slide_puzzle answer against the target x stored with it.
func checkSlidePuzzle(answer, expected string, sinceIssue time.Duration) bool {
	targetX, err := strconv.Atoi(expected)
	return err == nil && checkSlide(answer, targetX, sinceIssue).pass
}

// checkSlide decides from the answer, {"x": <final left edge>, "t": [[ms, x, y], ...]}, whether the
// piece reached targetX and whether the drag moved like a hand.
func checkSlide(answer string, targetX int, sinceIssue time.Duration) slideVerdict {
	fail := func(reason string) slideVerdict { return slideVerdict{reason: reason} }
	if len(answer) > slideMaxAnswerBytes {
		return fail("answer too long")
	}
	var parsed struct {
		X *float64    `json:"x"`
		T [][]float64 `json:"t"`
	}
	if json.Unmarshal([]byte(answer), &parsed) != nil || parsed.X == nil {
		return fail("not the answer JSON")
	}
	if len(parsed.T) < slideMinPoints || len(parsed.T) > slideMaxPoints {
		return fail("point count")
	}
	finalX := *parsed.X
	points := make([]slidePoint, len(parsed.T))
	for i, raw := range parsed.T {
		if len(raw) != 3 || !finite(raw[0]) || !finite(raw[1]) || !finite(raw[2]) {
			return fail("malformed point")
		}
		points[i] = slidePoint{t: raw[0], x: raw[1], y: raw[2]}
		if points[i].x < 0 || points[i].x > slideWidth {
			return fail("x outside the image")
		}
		if i > 0 && points[i].t <= points[i-1].t {
			return fail("times not increasing")
		}
	}
	first, last := points[0], points[len(points)-1]
	switch {
	case !finite(finalX) || finalX < 0 || finalX > slideWidth:
		return fail("x outside the image")
	case math.Abs(first.t) > slideStartSlackMs:
		return fail("trail does not start at pointerdown")
	case math.Abs(first.x) > slideStartSlackPx:
		return fail("trail does not start at x = 0")
	case math.Abs(last.x-finalX) > slideEndSlackPx:
		return fail("trail does not end at x")
	case math.Abs(finalX-float64(targetX)) > slideAimSlackPx:
		return fail("wrong position")
	case last.t < slideMinDragMs || last.t > slideMaxDragMs:
		return fail("drag duration")
	case sinceIssue < slideMinSolveTime:
		return fail("solved too soon after issue")
	}

	moving := movement(points)
	signals := 0
	for _, seen := range []bool{
		speedVariation(moving) > slideMinSpeedVariation,
		brakesAtEnd(moving),
		linearFit(moving) < slideMaxLinearFit,
		wobblesVertically(moving),
		unevenIntervals(moving),
	} {
		if seen {
			signals++
		}
	}
	return slideVerdict{pass: signals >= slideMinSignals, signals: signals}
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

// movement drops the points after the piece came to rest at its final x, keeping the first of them.
// People and scripts alike hold still before they release; counted in, that hold looks like
// braking, uneven speed and uneven sampling, and a constant-speed script passed with it.
func movement(points []slidePoint) []slidePoint {
	end := len(points) - 1
	for end > 1 && points[end-1].x == points[end].x {
		end--
	}
	return points[:end+1]
}

// speedVariation is the coefficient of variation of the speed between consecutive points.
func speedVariation(points []slidePoint) float64 {
	speeds := make([]float64, len(points)-1)
	var sum float64
	for i := range speeds {
		speeds[i] = math.Abs(points[i+1].x-points[i].x) / (points[i+1].t - points[i].t)
		sum += speeds[i]
	}
	mean := sum / float64(len(speeds))
	if mean == 0 {
		return 0
	}
	var squares float64
	for _, speed := range speeds {
		squares += (speed - mean) * (speed - mean)
	}
	return math.Sqrt(squares/float64(len(speeds))) / mean
}

// brakesAtEnd reports whether the mean speed over the last 25% of the distance is below
// slideMaxEndSpeedRatio of the mean over the middle 50%. Each step counts where it starts.
func brakesAtEnd(points []slidePoint) bool {
	distance := points[len(points)-1].x - points[0].x
	if distance <= 0 {
		return false
	}
	var middleMoved, middleTime, endMoved, endTime float64
	for i := range len(points) - 1 {
		moved, took := math.Abs(points[i+1].x-points[i].x), points[i+1].t-points[i].t
		switch progress := (points[i].x - points[0].x) / distance; {
		case progress >= 0.75:
			endMoved, endTime = endMoved+moved, endTime+took
		case progress >= 0.25:
			middleMoved, middleTime = middleMoved+moved, middleTime+took
		}
	}
	if middleTime == 0 || endTime == 0 || middleMoved == 0 {
		return false
	}
	return endMoved/endTime < slideMaxEndSpeedRatio*middleMoved/middleTime
}

// linearFit is R² of the least-squares line of x against t; 1 when x never changes.
func linearFit(points []slidePoint) float64 {
	n := float64(len(points))
	var meanT, meanX float64
	for _, point := range points {
		meanT, meanX = meanT+point.t/n, meanX+point.x/n
	}
	var covariance, varianceT, varianceX float64
	for _, point := range points {
		dt, dx := point.t-meanT, point.x-meanX
		covariance, varianceT, varianceX = covariance+dt*dx, varianceT+dt*dt, varianceX+dx*dx
	}
	if varianceT == 0 || varianceX == 0 {
		return 1
	}
	return covariance * covariance / (varianceT * varianceX)
}

// wobblesVertically reports whether y spans 2 whole pixels or at least slideMinYSpreadPx.
func wobblesVertically(points []slidePoint) bool {
	lowest, highest := points[0].y, points[0].y
	pixels := map[float64]bool{}
	for _, point := range points {
		lowest, highest = min(lowest, point.y), max(highest, point.y)
		pixels[math.Round(point.y)] = true
	}
	return len(pixels) >= 2 || highest-lowest >= slideMinYSpreadPx
}

// unevenIntervals reports whether the intervals between points differ by slideMinIntervalSpreadMs.
func unevenIntervals(points []slidePoint) bool {
	shortest, longest := math.Inf(1), 0.0
	for i := range len(points) - 1 {
		interval := points[i+1].t - points[i].t
		shortest, longest = min(shortest, interval), max(longest, interval)
	}
	return longest-shortest >= slideMinIntervalSpreadMs
}
