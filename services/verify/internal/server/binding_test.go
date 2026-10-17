package server

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"
	"testing"

	"verify/internal/biz"
)

const chromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36"

func TestVerifyChallengePassesTheRequestOn(t *testing.T) {
	h := newHarness(t)
	body := fmt.Sprintf(`{"challenge_id":%q,"answer":"abcde","site_key":"pk_shop","pow_nonce":"1233",`+
		`"behavior":{"version":1,"headless_ua":true,"no_languages":true,"chrome_without_window_chrome":true}}`, testChallengeID)
	got := h.do(t, http.MethodPost, "/challenge/verify", "application/json", body, "Origin", "https://shop.example.com",
		"User-Agent", chromeUA, "Sec-Fetch-Site", "cross-site", "Accept-Language", "vi-VN,vi;q=0.9")
	if got.status != 200 {
		t.Fatalf("got %d %s", got.status, got.rawBody)
	}
	answer := h.challenges.answer
	behavior := answer.Behavior
	answer.Behavior = nil
	want := biz.Answer{ChallengeID: testChallengeID, Text: "abcde", SiteKey: "pk_shop", PowNonce: "1233", Origin: "https://shop.example.com",
		Client: netip.MustParseAddr("203.0.113.4"), UserAgent: chromeUA, SecFetchSite: "cross-site", AcceptLanguage: "vi-VN,vi;q=0.9"}
	if answer != want {
		t.Fatalf("passed on %+v\nwant %+v", answer, want)
	}
	if behavior == nil || !behavior.HeadlessUA || !behavior.NoLanguages || !behavior.ChromeWithoutWindowChrome {
		t.Fatalf("client hints lost: %+v", behavior)
	}
	// The page reads the reply: its own origin is named back, never "*".
	if got.header.Get("Access-Control-Allow-Origin") != "https://shop.example.com" || !strings.Contains(strings.Join(got.header.Values("Vary"), ","), "Origin") {
		t.Fatalf("CORS headers %v", got.header)
	}
}

func TestVerifyChallengeWithoutOrigin(t *testing.T) {
	h := newHarness(t)
	got := h.do(t, http.MethodPost, "/challenge/verify", "application/json", fmt.Sprintf(`{"challenge_id":%q,"answer":"abcde"}`, testChallengeID),
		"Referer", "https://shop.example.com/signup?step=2")
	if got.status != 200 || h.challenges.answer.Origin != "https://shop.example.com" || got.header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("got %d, origin %q, %v", got.status, h.challenges.answer.Origin, got.header)
	}
}

func TestBehaviorRequired(t *testing.T) {
	h := newHarness(t)
	h.challenges.solution = biz.Solution{APIKeyID: "key-1", ChallengeType: "image_distort", UserID: "user-1",
		Behavior: biz.BehaviorResult{Verdict: biz.VerdictNotMeasured}, BehaviorRequired: true}
	got := h.do(t, http.MethodPost, "/challenge/verify", "application/json", fmt.Sprintf(`{"challenge_id":%q,"answer":"abcde"}`, testChallengeID))
	message, _ := got.body["message"].(string)
	if got.status != 400 || got.body["reason"] != "BEHAVIOR_REQUIRED" || !strings.Contains(message, "@waucaptcha-sdk/client") {
		t.Fatalf("got %d %s", got.status, got.rawBody)
	}
	// Counted as an answer, but neither solved nor against the quota.
	if c := h.counters; c.answered != 1 || c.solved != 0 || c.successes != 0 {
		t.Fatalf("counters %+v", c)
	}
}

func TestAnswerCounters(t *testing.T) {
	tests := []struct {
		name                                   string
		success                                bool
		verdict                                string
		answered, solved, automated, successes int
	}{
		{"solved", true, "human", 1, 1, 0, 1},
		{"wrong", false, "human", 1, 0, 0, 0},
		{"automated", false, biz.VerdictAutomated, 1, 0, 1, 0},
		{"suspicious but right", true, "suspicious", 1, 1, 0, 1},
	}
	for _, test := range tests {
		h := newHarness(t)
		h.challenges.solution.Success = test.success
		h.challenges.solution.Behavior = biz.BehaviorResult{Verdict: test.verdict}
		got := h.do(t, http.MethodPost, "/challenge/verify", "application/json", fmt.Sprintf(`{"challenge_id":%q,"answer":"abcde"}`, testChallengeID))
		c := h.counters
		if got.status != 200 || (got.body["success"] == true) != test.success ||
			c.answered != test.answered || c.solved != test.solved || c.automated != test.automated || c.successes != test.successes {
			t.Errorf("%s: got %d %s, counters %+v", test.name, got.status, got.rawBody, c)
		}
	}
}

func TestSiteVerifyHostname(t *testing.T) {
	h := newHarness(t)
	got := h.do(t, http.MethodPost, "/siteverify", "application/json", fmt.Sprintf(`{"token":%q}`, h.token(t)), "Authorization", "Bearer "+testSecret)
	if got.status != 200 || got.body["success"] != true || got.body["hostname"] != "shop.example.com" {
		t.Fatalf("got %d %s", got.status, got.rawBody)
	}
}
