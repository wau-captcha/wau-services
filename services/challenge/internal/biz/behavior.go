package biz

// Behavior is what the SDK recorded while a challenge was on screen, from the moment it was shown
// to the moment its answer was sent. Times are milliseconds since it was shown; X and Y are CSS
// pixels from the top-left corner of the widget.
type Behavior struct {
	Version   int
	ElapsedMs int
	// Pointer moves, at most one every 16 ms, and the places it went down, oldest first.
	Pointer []PointerSample
	// "mouse", "touch", "pen", or empty when no pointer was used.
	PointerType string
	// Milliseconds between key presses in the answer box.
	KeyIntervalsMs []int
	InputEvents    int
	PasteEvents    int
	// Events a script dispatched on the widget (isTrusted false).
	UntrustedEvents int
	Webdriver       bool
	HiddenMs        int
	// Hints the browser gives about itself, from SDKs 1.3.0 on: navigator.userAgent names
	// HeadlessChrome, navigator.languages is empty, a Chrome user agent lacks window.chrome. A
	// script can fake each, so they only lower the score.
	HeadlessUA                bool
	NoLanguages               bool
	ChromeWithoutWindowChrome bool
}

type PointerSample struct {
	T    int
	X, Y float64
}

// Verdicts of the behaviour check.
const (
	VerdictHuman = "human"
	// The answer counts; the site decides what to do with it.
	VerdictSuspicious = "suspicious"
	// A hard rule broke: a right answer is refused.
	VerdictAutomated = "automated"
	// The client sent no behaviour, so only the answer was checked.
	VerdictNotMeasured = "not_measured"
)

// BehaviorResult is what the behaviour check concluded about one answer.
type BehaviorResult struct {
	Verdict string
	// From 0 (scripted) to 1 (human-like); meaningless when not measured.
	Score float64
	// Short codes of what lowered the score or broke a rule.
	Reasons []string
}

// Measured reports whether the client sent behaviour to judge.
func (r BehaviorResult) Measured() bool { return r.Verdict != VerdictNotMeasured }
