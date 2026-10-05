package biz

import (
	"math"
	"slices"
)

// strokes splits the pointer path into continuous movements and returns the speeds of each, in
// px/ms. Pauses and samples in place are left out: they would make a steady script look uneven.
func strokes(samples []PointerSample) [][]float64 {
	var all [][]float64
	var current []float64
	for i := 1; i < len(samples); i++ {
		dt := float64(samples[i].T - samples[i-1].T)
		distance := math.Hypot(samples[i].X-samples[i-1].X, samples[i].Y-samples[i-1].Y)
		if dt <= 0 || dt > strokePauseMs || distance == 0 {
			if len(current) > 0 {
				all, current = append(all, current), nil
			}
			continue
		}
		current = append(current, distance/dt)
	}
	if len(current) > 0 {
		all = append(all, current)
	}
	return all
}

// keyRhythm is the coefficient of variation and the median of the key intervals; judged is false
// when there are too few to tell.
func keyRhythm(intervals []int) (variation, median float64, judged bool) {
	if len(intervals) < minKeyIntervalsJudged {
		return 0, 0, false
	}
	values := make([]float64, len(intervals))
	for i, interval := range intervals {
		values[i] = float64(interval)
	}
	sorted := slices.Sorted(slices.Values(values))
	median = sorted[len(sorted)/2]
	if len(sorted)%2 == 0 {
		median = (sorted[len(sorted)/2-1] + median) / 2
	}
	return coefficientOfVariation(values), median, true
}

func coefficientOfVariation(values []float64) float64 {
	var sum float64
	for _, value := range values {
		sum += value
	}
	mean := sum / float64(len(values))
	if mean == 0 {
		return 0
	}
	var squares float64
	for _, value := range values {
		squares += (value - mean) * (value - mean)
	}
	return math.Sqrt(squares/float64(len(values))) / mean
}
