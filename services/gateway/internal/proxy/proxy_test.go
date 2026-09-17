package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// upstream records the last request it served and answers with its own name.
type upstream struct {
	server  *httptest.Server
	request *http.Request
	status  int
	header  map[string]string
}

func newUpstream(t *testing.T, name string) *upstream {
	t.Helper()
	u := &upstream{status: http.StatusOK}
	u.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u.request = r.Clone(r.Context())
		for key, value := range u.header {
			w.Header().Set(key, value)
		}
		w.WriteHeader(u.status)
		_, _ = w.Write([]byte(name))
	}))
	t.Cleanup(u.server.Close)
	return u
}

func (u *upstream) url(t *testing.T) *url.URL {
	t.Helper()
	parsed, err := url.Parse(u.server.URL)
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}
	return parsed
}

func newProxy(t *testing.T, routes []Route) http.Handler {
	t.Helper()
	handler, err := New(routes)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return handler
}

func call(handler http.Handler, method, target string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, nil)
	request.RemoteAddr = "203.0.113.7:12345"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func reasonOf(t *testing.T, body []byte) string {
	t.Helper()
	var parsed struct {
		Code   int    `json:"code"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("decode error body %q: %v", body, err)
	}
	return parsed.Reason
}

func TestLongestPrefixWins(t *testing.T) {
	general := newUpstream(t, "tenant")
	auth := newUpstream(t, "auth")
	handler := newProxy(t, []Route{
		{Prefix: "/v1/", Upstream: general.url(t)},
		{Prefix: "/v1/auth/", Upstream: auth.url(t)},
	})

	if got := call(handler, http.MethodGet, "/v1/auth/login").Body.String(); got != "auth" {
		t.Fatalf("/v1/auth/login went to %q, want auth", got)
	}
	if got := call(handler, http.MethodGet, "/v1/me").Body.String(); got != "tenant" {
		t.Fatalf("/v1/me went to %q, want tenant", got)
	}
}

func TestPathAndQueryPreserved(t *testing.T) {
	challenge := newUpstream(t, "challenge")
	handler := newProxy(t, []Route{{Prefix: "/challenge/new", Upstream: challenge.url(t)}})

	call(handler, http.MethodGet, "/challenge/new?site_key=pk_1")

	if challenge.request.URL.Path != "/challenge/new" {
		t.Fatalf("path = %q", challenge.request.URL.Path)
	}
	if challenge.request.URL.RawQuery != "site_key=pk_1" {
		t.Fatalf("query = %q", challenge.request.URL.RawQuery)
	}
}

func TestForwardedHeaders(t *testing.T) {
	tenant := newUpstream(t, "tenant")
	handler := newProxy(t, []Route{{Prefix: "/v1/", Upstream: tenant.url(t)}})

	call(handler, http.MethodGet, "/v1/me")

	if got := tenant.request.Header.Get("X-Forwarded-For"); got != "203.0.113.7" {
		t.Fatalf("X-Forwarded-For = %q, want 203.0.113.7", got)
	}
	if tenant.request.Header.Get("X-Forwarded-Host") == "" {
		t.Fatal("X-Forwarded-Host is missing")
	}
}

func TestUnknownRouteIsNotFound(t *testing.T) {
	tenant := newUpstream(t, "tenant")
	handler := newProxy(t, []Route{{Prefix: "/v1/", Upstream: tenant.url(t)}})

	response := call(handler, http.MethodGet, "/nope")

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
	if got := reasonOf(t, response.Body.Bytes()); got != "ROUTE_NOT_FOUND" {
		t.Fatalf("reason = %q", got)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", contentType)
	}
}

func TestUpstreamDownIsBadGateway(t *testing.T) {
	down := newUpstream(t, "down")
	target := down.url(t)
	down.server.Close()
	handler := newProxy(t, []Route{{Prefix: "/v1/", Upstream: target}})

	response := call(handler, http.MethodGet, "/v1/me")

	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", response.Code)
	}
	if got := reasonOf(t, response.Body.Bytes()); got != "UPSTREAM_UNAVAILABLE" {
		t.Fatalf("reason = %q", got)
	}
}

func TestUpstreamResponsePassesThrough(t *testing.T) {
	tenant := newUpstream(t, "tenant")
	tenant.status = http.StatusUnauthorized
	tenant.header = map[string]string{"Access-Control-Allow-Origin": "*"}
	handler := newProxy(t, []Route{{Prefix: "/v1/", Upstream: tenant.url(t)}})

	response := call(handler, http.MethodGet, "/v1/me")

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("CORS header = %q, want *", got)
	}
}

func TestNewRequiresRoutes(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("want error when no route is configured")
	}
}
