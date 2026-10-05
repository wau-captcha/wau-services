package biz

// Behavior is what the SDK recorded while a challenge was on screen. Verify does not read it: it
// passes it to the challenge service, which checks it with the answer.
type Behavior struct {
	Version         int
	ElapsedMs       int
	Pointer         []PointerSample
	PointerType     string
	KeyIntervalsMs  []int
	InputEvents     int
	PasteEvents     int
	UntrustedEvents int
	Webdriver       bool
	HiddenMs        int
}

type PointerSample struct {
	T    int
	X, Y float64
}

// VerdictNotMeasured is the verdict of an answer sent without behaviour, or checked by a
// challenge service that does not check behaviour yet.
const VerdictNotMeasured = "not_measured"

// BehaviorResult is what the challenge service concluded about the behaviour sent with an answer.
type BehaviorResult struct {
	// "human", "suspicious", "automated" or "not_measured".
	Verdict string
	// From 0 to 1; nil when not measured.
	Score   *float64
	Reasons []string
}
