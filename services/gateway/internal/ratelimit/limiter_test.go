package ratelimit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAllowCountsWindow(t *testing.T) {
	limiter := New(2, time.Minute)
	now := time.Now()

	for i := range 2 {
		if ok, _ := limiter.Allow("a", now); !ok {
			t.Fatalf("request %d blocked, want allowed", i+1)
		}
	}
	ok, retry := limiter.Allow("a", now)
	if ok {
		t.Fatal("third request allowed, want blocked")
	}
	if retry <= 0 || retry > time.Minute {
		t.Fatalf("retry after = %v, want within the window", retry)
	}
	if ok, _ := limiter.Allow("b", now); !ok {
		t.Fatal("other key blocked, want independent counters")
	}
	if ok, _ := limiter.Allow("a", now.Add(time.Minute)); !ok {
		t.Fatal("blocked after the window passed")
	}
}

func TestMiddlewareLimitsOnlyConfiguredPrefixes(t *testing.T) {
	limiter := New(1, time.Minute)
	served := 0
	handler := Middleware(limiter, []string{"/v1/auth/"}, nil, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		served++
	}))

	send := func(path string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, path, nil)
		request.RemoteAddr = "203.0.113.7:1111"
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder
	}

	if code := send("/v1/auth/google").Code; code != http.StatusOK {
		t.Fatalf("first sign-in status = %d, want 200", code)
	}
	blocked := send("/v1/auth/google")
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("second sign-in status = %d, want 429", blocked.Code)
	}
	if blocked.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header is missing")
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(blocked.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Reason != "RATE_LIMITED" {
		t.Fatalf("reason = %q", body.Reason)
	}

	for range 3 {
		if code := send("/v1/me").Code; code != http.StatusOK {
			t.Fatalf("unlimited path status = %d, want 200", code)
		}
	}
	if served != 4 {
		t.Fatalf("handler ran %d times, want 4", served)
	}
}
