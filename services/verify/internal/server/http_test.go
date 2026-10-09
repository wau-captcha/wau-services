package server

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"verify/internal/biz"
	"verify/internal/conf"
	"verify/internal/pkg/ratelimit"
	"verify/internal/service"

	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	testChallengeID = "0f9c6b5e-1d2a-4c3b-8e7f-9a0b1c2d3e4f"
	testSecret      = "sk_4a43813384268b62be376cf7b81eb47ca9619c23e2b4c198"
)

// fakeChallenges remembers the last answer passed on and replies with solution, or err.
type fakeChallenges struct {
	mu       sync.Mutex
	err      error
	calls    int
	answer   biz.Answer
	behavior *biz.Behavior
	solution biz.Solution
}

func (f *fakeChallenges) CheckAnswer(_ context.Context, answer biz.Answer) (*biz.Solution, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.answer = answer
	f.behavior = answer.Behavior
	if f.err != nil {
		return nil, f.err
	}
	solution := f.solution
	return &solution, nil
}

type fakeGrants struct {
	mu     sync.Mutex
	grants map[string]*biz.Grant
}

func (f *fakeGrants) Save(_ context.Context, id string, grant *biz.Grant, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.grants[id] = grant
	return nil
}

func (f *fakeGrants) Take(_ context.Context, id string) (*biz.Grant, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	grant := f.grants[id]
	delete(f.grants, id)
	return grant, nil
}

type fakeSites struct{}

func (fakeSites) FindBySecretKey(_ context.Context, secretKey string) (*biz.Site, error) {
	if secretKey != testSecret {
		return nil, biz.ErrSecretKeyInvalid
	}
	return &biz.Site{APIKeyID: "key-1"}, nil
}
func (fakeSites) RecordAnswer(context.Context, string, *biz.Solution) error { return nil }

type fakeCounters struct {
	mu                                     sync.Mutex
	answered, solved, automated, successes int
}

func (f *fakeCounters) CountAnswer(_ context.Context, _, _ string, solved, automated bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answered++
	if solved {
		f.solved++
	}
	if automated {
		f.automated++
	}
	return nil
}

func (f *fakeCounters) CountSuccess(context.Context, string, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.successes++
	return nil
}

// fakeLimiter refuses the scopes in refuse for 30 s and lets everything else through.
type fakeLimiter struct{ refuse map[string]bool }

func (f fakeLimiter) Allow(_ context.Context, rule ratelimit.Rule, _ string) time.Duration {
	if f.refuse[rule.Scope] {
		return 30 * time.Second
	}
	return 0
}

type harness struct {
	server     http.Handler
	challenges *fakeChallenges
	counters   *fakeCounters
}

func newHarness(t *testing.T, refuse ...string) *harness {
	t.Helper()
	limiter := fakeLimiter{refuse: map[string]bool{}}
	for _, scope := range refuse {
		limiter.refuse[scope] = true
	}
	challenges := &fakeChallenges{solution: biz.Solution{Success: true, APIKeyID: "key-1", ChallengeType: "image_distort", UserID: "user-1",
		Behavior: biz.BehaviorResult{Verdict: biz.VerdictNotMeasured}, Hostname: "shop.example.com"}}
	counters := &fakeCounters{}
	usecase, err := biz.NewVerifyUsecase(challenges, &fakeGrants{grants: map[string]*biz.Grant{}}, fakeSites{}, counters, limiter,
		&conf.Token{Secret: "test-signing-secret", Ttl: durationpb.New(2 * time.Minute)}, &conf.Quota{TimeZone: "Asia/Ho_Chi_Minh"},
		&conf.RateLimit{VerifyPerIp: 60, SiteverifyPerSecretKey: 1200})
	if err != nil {
		t.Fatal(err)
	}
	serverConf := &conf.Server{Http: &conf.Server_HTTP{}}
	verify, err := service.NewVerifyService(usecase, serverConf)
	if err != nil {
		t.Fatal(err)
	}
	return &harness{server: NewHTTPServer(serverConf, verify), challenges: challenges, counters: counters}
}

type reply struct {
	status  int
	header  http.Header
	body    map[string]any
	rawBody string
}

func (h *harness) do(t *testing.T, method, target, contentType, body string, headers ...string) reply {
	t.Helper()
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.RemoteAddr = "203.0.113.4:5123"
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		request.Header.Set(headers[i], headers[i+1])
	}
	recorder := httptest.NewRecorder()
	h.server.ServeHTTP(recorder, request)
	parsed := map[string]any{}
	_ = json.Unmarshal(recorder.Body.Bytes(), &parsed)
	return reply{status: recorder.Code, header: recorder.Header(), body: parsed, rawBody: recorder.Body.String()}
}

func (h *harness) token(t *testing.T) string {
	t.Helper()
	answer := h.do(t, http.MethodPost, "/challenge/verify", "application/json", fmt.Sprintf(`{"challenge_id":%q,"answer":"abcde"}`, testChallengeID))
	token, _ := answer.body["token"].(string)
	if answer.status != 200 || token == "" {
		t.Fatalf("verify: %d %s", answer.status, answer.rawBody)
	}
	return token
}

func TestVerifyChallengeLimits(t *testing.T) {
	t.Run("a 70 KB body is 413", func(t *testing.T) {
		h := newHarness(t)
		body := fmt.Sprintf(`{"challenge_id":%q,"answer":%q}`, testChallengeID, strings.Repeat("a", 70<<10))
		got := h.do(t, http.MethodPost, "/challenge/verify", "application/json", body)
		if got.status != 413 || got.body["reason"] != "PAYLOAD_TOO_LARGE" || h.challenges.calls != 0 {
			t.Fatalf("got %d %s, %d calls", got.status, got.rawBody, h.challenges.calls)
		}
	})
	t.Run("an 8 KB challenge_id is 400, not 500", func(t *testing.T) {
		h := newHarness(t)
		body := fmt.Sprintf(`{"challenge_id":%q,"answer":"abcde"}`, strings.Repeat("x", 8<<10))
		got := h.do(t, http.MethodPost, "/challenge/verify", "application/json", body)
		if got.status != 400 || got.body["reason"] != "CHALLENGE_ID_INVALID" || h.challenges.calls != 0 {
			t.Fatalf("got %d %s", got.status, got.rawBody)
		}
	})
	t.Run("201 pointer samples are handled without error", func(t *testing.T) {
		h := newHarness(t)
		samples := make([]string, 400)
		for i := range samples {
			samples[i] = fmt.Sprintf(`{"t":%d,"x":%d,"y":1}`, i*16, i)
		}
		body := fmt.Sprintf(`{"challenge_id":%q,"answer":"abcde","behavior":{"version":1,"elapsed_ms":7000,"pointer":[%s],"pointer_type":%q}}`,
			testChallengeID, strings.Join(samples, ","), strings.Repeat("m", 300))
		got := h.do(t, http.MethodPost, "/challenge/verify", "application/json", body)
		if got.status != 200 || h.challenges.behavior == nil || len(h.challenges.behavior.Pointer) != 201 || h.challenges.behavior.PointerType != "invalid" {
			t.Fatalf("got %d %s, forwarded %+v", got.status, got.rawBody, h.challenges.behavior)
		}
	})
	t.Run("over the per-address limit: 429 with Retry-After", func(t *testing.T) {
		h := newHarness(t, "verify_ip")
		got := h.do(t, http.MethodPost, "/challenge/verify", "application/json", fmt.Sprintf(`{"challenge_id":%q,"answer":"abcde"}`, testChallengeID))
		metadata, _ := got.body["metadata"].(map[string]any)
		if got.status != 429 || got.body["reason"] != "RATE_LIMITED" || got.header.Get("Retry-After") != "30" || metadata["retry_after"] != "30" {
			t.Fatalf("got %d %v %s", got.status, got.header, got.rawBody)
		}
	})
	t.Run("an unexpected failure is internal error", func(t *testing.T) {
		h := newHarness(t)
		h.challenges.err = stderrors.New("dial tcp 10.43.0.12:9000: connect: connection refused")
		got := h.do(t, http.MethodPost, "/challenge/verify", "application/json", fmt.Sprintf(`{"challenge_id":%q,"answer":"abcde"}`, testChallengeID))
		if got.status != 500 || got.body["reason"] != "INTERNAL" || got.body["message"] != "internal error" || strings.Contains(got.rawBody, "10.43") {
			t.Fatalf("got %d %s", got.status, got.rawBody)
		}
	})
}

func TestCORSAndCache(t *testing.T) {
	h := newHarness(t)
	preflight := h.do(t, http.MethodOptions, "/challenge/verify", "", "", "Origin", "https://shop.example", "Access-Control-Request-Method", "POST")
	if preflight.status != 204 || preflight.header.Get("Access-Control-Allow-Headers") != "Content-Type" {
		t.Fatalf("/challenge/verify preflight: %d %v", preflight.status, preflight.header)
	}
	for _, method := range []string{http.MethodPost, http.MethodOptions} {
		got := h.do(t, method, "/siteverify", "", "", "Origin", "https://evil.example", "Authorization", "Bearer "+testSecret)
		for name := range got.header {
			if strings.HasPrefix(name, "Access-Control-") {
				t.Fatalf("%s /siteverify sent %s", method, name)
			}
		}
	}
	got := h.do(t, http.MethodPost, "/siteverify", "", "", "Authorization", "Bearer "+testSecret)
	if got.header.Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", got.header.Get("Cache-Control"))
	}
}

func TestSiteVerifyToken(t *testing.T) {
	auth := []string{"Authorization", "Bearer " + testSecret}
	tests := []struct {
		name string
		send func(h *harness, token string) reply
		want bool
	}{
		{"query, empty body (SDKs before 1.3.0)", func(h *harness, token string) reply {
			return h.do(t, http.MethodPost, "/siteverify?token="+url.QueryEscape(token), "", "", auth...)
		}, true},
		{"JSON body", func(h *harness, token string) reply {
			return h.do(t, http.MethodPost, "/siteverify", "application/json", fmt.Sprintf(`{"token":%q}`, token), auth...)
		}, true},
		{"form body", func(h *harness, token string) reply {
			return h.do(t, http.MethodPost, "/siteverify", "application/x-www-form-urlencoded", "token="+url.QueryEscape(token), auth...)
		}, true},
		{"both: the body wins", func(h *harness, token string) reply {
			return h.do(t, http.MethodPost, "/siteverify?token=not-a-token", "application/json", fmt.Sprintf(`{"token":%q}`, token), auth...)
		}, true},
		{"a token past 512 characters is success false", func(h *harness, token string) reply {
			return h.do(t, http.MethodPost, "/siteverify", "application/json", fmt.Sprintf(`{"token":%q}`, token+strings.Repeat("A", 600)), auth...)
		}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newHarness(t)
			got := test.send(h, h.token(t))
			if got.status != 200 || (got.body["success"] == true) != test.want {
				t.Fatalf("got %d %s, want success %v", got.status, got.rawBody, test.want)
			}
		})
	}
	t.Run("over the per-key limit: 429", func(t *testing.T) {
		h := newHarness(t, "siteverify_key")
		got := h.do(t, http.MethodPost, "/siteverify", "application/json", `{"token":"x"}`, auth...)
		if got.status != 429 || got.header.Get("Retry-After") != "30" {
			t.Fatalf("got %d %v", got.status, got.header)
		}
	})
}
