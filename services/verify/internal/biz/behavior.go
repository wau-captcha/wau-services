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
	// Hints the browser gives about itself, from SDKs 1.3.0 on.
	HeadlessUA                bool
	NoLanguages               bool
	ChromeWithoutWindowChrome bool
}

type PointerSample struct {
	T    int
	X, Y float64
}

const (
	// VerdictNotMeasured is the verdict of an answer sent without behaviour, or checked by a
	// challenge service that does not check behaviour yet.
	VerdictNotMeasured = "not_measured"
	// VerdictAutomated is the verdict of an answer that broke a rule of the check; it never solves
	// a challenge.
	VerdictAutomated = "automated"
)

// The limits of the challenge service's behaviour check (behaviorMaxPointer and
// behaviorMaxKeyIntervals there), and the longest pointer type it knows ("mouse").
const (
	maxPointerSamples = 200
	maxKeyIntervals   = 64
	maxPointerType    = 16
)

// boundBehavior keeps what verify forwards small. Lists are cut to one item past their limit, and
// an overlong pointer type is replaced by one the check does not know, so a payload over a limit
// still reaches the check, which judges it automated, without the rest of it. Numbers are left
// as sent: the check judges negative or absurd ones itself.
func boundBehavior(behavior *Behavior) *Behavior {
	if behavior == nil {
		return nil
	}
	bounded := *behavior
	if len(bounded.Pointer) > maxPointerSamples+1 {
		bounded.Pointer = bounded.Pointer[:maxPointerSamples+1]
	}
	if len(bounded.KeyIntervalsMs) > maxKeyIntervals+1 {
		bounded.KeyIntervalsMs = bounded.KeyIntervalsMs[:maxKeyIntervals+1]
	}
	if len(bounded.PointerType) > maxPointerType {
		bounded.PointerType = "invalid"
	}
	return &bounded
}

// BehaviorResult is what the challenge service concluded about the behaviour sent with an answer.
type BehaviorResult struct {
	// "human", "suspicious", "automated" or "not_measured".
	Verdict string
	// From 0 to 1; nil when not measured.
	Score   *float64
	Reasons []string
}
