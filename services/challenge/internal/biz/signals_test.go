package biz

import (
	"slices"
	"testing"
	"time"
)

const chromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36"

func TestAutomationUA(t *testing.T) {
	tests := []struct {
		userAgent string
		automated bool
	}{
		{chromeUA, false},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.6 Mobile/15E148 Safari/604.1", false},
		{"", true},
		{"   ", true},
		{"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/141.0.0.0 Safari/537.36", true},
		{"python-requests/2.32.3", true},
		{"curl/8.9.1", true},
		{"Go-http-client/2.0", true},
		{"node-fetch/1.0 (+https://github.com/bitinn/node-fetch)", true},
		{"axios/1.7.7", true},
		{"Mozilla/5.0 Playwright/1.48", true},
		{"Mozilla/5.0 (Unknown; Linux x86_64) AppleWebKit/538.1 (KHTML, like Gecko) PhantomJS/2.1.1 Safari/538.1", true},
		{"okhttp/4.12.0", false},
	}
	for _, test := range tests {
		if got := automationUA(test.userAgent); got != test.automated {
			t.Errorf("automationUA(%q) = %v, want %v", test.userAgent, got, test.automated)
		}
	}
}

func TestMissingBrowserHeaders(t *testing.T) {
	tests := []struct {
		userAgent, secFetchSite, acceptLanguage string
		missing                                 bool
	}{
		{chromeUA, "cross-site", "vi-VN,vi;q=0.9", false},
		{chromeUA, "", "vi-VN,vi;q=0.9", true},
		{chromeUA, "cross-site", "", true},
		{chromeUA, "", "", true},
		// Not a browser: the headers are not expected.
		{"okhttp/4.12.0", "", "", false},
	}
	for _, test := range tests {
		if got := missingBrowserHeaders(test.userAgent, test.secFetchSite, test.acceptLanguage); got != test.missing {
			t.Errorf("missingBrowserHeaders(%q, %q, %q) = %v, want %v", test.userAgent, test.secFetchSite, test.acceptLanguage, got, test.missing)
		}
	}
}

// humanBehavior scores 1: typed at an uneven pace, with a mouse, well within the time since issue.
func humanBehavior() *Behavior {
	return &Behavior{Version: 1, ElapsedMs: 4000, PointerType: "mouse", KeyIntervalsMs: []int{180, 240, 150, 300}, InputEvents: 5}
}

func TestCheckBehaviorSignals(t *testing.T) {
	withHints := humanBehavior()
	withHints.HeadlessUA, withHints.NoLanguages, withHints.ChromeWithoutWindowChrome = true, true, true
	oneHint := humanBehavior()
	oneHint.HeadlessUA = true
	webdriver := humanBehavior()
	webdriver.Webdriver = true
	tests := []struct {
		name     string
		behavior *Behavior
		signals  Signals
		verdict  string
		score    float64
		reasons  []string
	}{
		{"human", humanBehavior(), Signals{}, VerdictHuman, 1, []string{}},
		{"automation UA without behaviour", nil, Signals{AutomationUA: true}, VerdictAutomated, 0, []string{"automation_ua"}},
		{"automation UA over human behaviour", humanBehavior(), Signals{AutomationUA: true}, VerdictAutomated, 0, []string{"automation_ua"}},
		{"wrong proof of work", humanBehavior(), Signals{PowInvalid: true}, VerdictAutomated, 0, []string{"pow_invalid"}},
		{"no behaviour, no proof of work", nil, Signals{NoPow: true}, VerdictNotMeasured, 0, nil},
		{"no proof of work: at most suspicious", humanBehavior(), Signals{NoPow: true}, VerdictSuspicious, 0.4, []string{"no_pow"}},
		{"address changed", humanBehavior(), Signals{IPChanged: true}, VerdictHuman, 0.8, []string{"ip_changed"}},
		{"address, browser and headers", humanBehavior(), Signals{IPChanged: true, UAChanged: true, MissingBrowserHeaders: true},
			VerdictSuspicious, 0.3, []string{"ip_changed", "ua_changed", "missing_browser_headers"}},
		{"bad history", humanBehavior(), Signals{BadHistory: true}, VerdictHuman, 0.7, []string{"ip_bad_history"}},
		{"one client hint", oneHint, Signals{}, VerdictHuman, 0.75, []string{"headless_ua"}},
		{"three client hints", withHints, Signals{}, VerdictSuspicious, 0.45, []string{"headless_ua", "no_languages", "chrome_without_window_chrome"}},
		{"webdriver and no proof of work", webdriver, Signals{NoPow: true}, VerdictSuspicious, 0, []string{"webdriver", "no_pow"}},
	}
	for _, test := range tests {
		got := checkBehavior(test.behavior, TypeImageDistort, 10*time.Second, test.signals)
		if got.Verdict != test.verdict || got.Score != test.score || !slices.Equal(got.Reasons, test.reasons) {
			t.Errorf("%s: got %s %.2f %q, want %s %.2f %q", test.name, got.Verdict, got.Score, got.Reasons, test.verdict, test.score, test.reasons)
		}
	}
}
