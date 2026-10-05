package service

import (
	v1 "verify/api/verify/v1"
	"verify/internal/biz"
)

// toBehavior converts the behaviour the SDK sent; nil when it sent none.
func toBehavior(behavior *v1.Behavior) *biz.Behavior {
	if behavior == nil {
		return nil
	}
	samples := make([]biz.PointerSample, len(behavior.GetPointer()))
	for i, sample := range behavior.GetPointer() {
		samples[i] = biz.PointerSample{T: int(sample.GetT()), X: float64(sample.GetX()), Y: float64(sample.GetY())}
	}
	intervals := make([]int, len(behavior.GetKeyIntervalsMs()))
	for i, interval := range behavior.GetKeyIntervalsMs() {
		intervals[i] = int(interval)
	}
	return &biz.Behavior{
		Version:         int(behavior.GetVersion()),
		ElapsedMs:       int(behavior.GetElapsedMs()),
		Pointer:         samples,
		PointerType:     behavior.GetPointerType(),
		KeyIntervalsMs:  intervals,
		InputEvents:     int(behavior.GetInputEvents()),
		PasteEvents:     int(behavior.GetPasteEvents()),
		UntrustedEvents: int(behavior.GetUntrustedEvents()),
		Webdriver:       behavior.GetWebdriver(),
		HiddenMs:        int(behavior.GetHiddenMs()),
	}
}

func toBehaviorResult(result biz.BehaviorResult) *v1.BehaviorResult {
	return &v1.BehaviorResult{Verdict: result.Verdict, Score: result.Score, Reasons: result.Reasons}
}
