package filter

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var routes = map[string]Route{
	"/challenge/new":    {Methods: "GET, OPTIONS"},
	"/challenge/verify": {Methods: "POST, OPTIONS", ReflectOrigin: true},
}

func serve(handler http.Handler, method, path, origin string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestCORS(t *testing.T) {
	handler := CORS(routes)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	t.Run("listed route, any origin", func(t *testing.T) {
		reply := serve(handler, http.MethodGet, "/challenge/new", "https://shop.example")
		if reply.Header().Get("Access-Control-Allow-Origin") != "*" || reply.Code != http.StatusOK ||
			reply.Header().Get("Access-Control-Expose-Headers") != "Retry-After" {
			t.Fatalf("got %d %v", reply.Code, reply.Header())
		}
	})
	t.Run("preflight allows Content-Type only", func(t *testing.T) {
		reply := serve(handler, http.MethodOptions, "/challenge/verify", "https://shop.example")
		headers := reply.Header()
		if reply.Code != http.StatusNoContent || headers.Get("Access-Control-Allow-Headers") != "Content-Type" ||
			strings.Contains(headers.Get("Access-Control-Allow-Headers"), "Authorization") || headers.Get("Access-Control-Allow-Methods") != "POST, OPTIONS" {
			t.Fatalf("got %d %v", reply.Code, headers)
		}
		if headers.Get("Access-Control-Allow-Origin") != "https://shop.example" || headers.Get("Vary") != "Origin" {
			t.Fatalf("origin not reflected: %v", headers)
		}
	})
	for _, path := range []string{"/siteverify", "/healthz"} {
		t.Run(path+" gets no CORS headers", func(t *testing.T) {
			for _, method := range []string{http.MethodPost, http.MethodOptions} {
				reply := serve(handler, method, path, "https://evil.example")
				for name := range reply.Header() {
					if strings.HasPrefix(name, "Access-Control-") {
						t.Fatalf("%s %s sent %s", method, path, name)
					}
				}
			}
		})
	}
}

func TestLimitBody(t *testing.T) {
	var readErr error
	handler := LimitBody(map[string]int64{"/challenge/verify": 64}, 8)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}))
	post := func(path string, size int) error {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(strings.Repeat("a", size)))
		handler.ServeHTTP(httptest.NewRecorder(), request)
		return readErr
	}
	var tooLarge *http.MaxBytesError
	if err := post("/challenge/verify", 64); err != nil {
		t.Fatalf("64 bytes to a 64-byte path: %v", err)
	}
	if err := post("/challenge/verify", 65); !errors.As(err, &tooLarge) {
		t.Fatalf("65 bytes to a 64-byte path: %v", err)
	}
	if err := post("/other", 9); !errors.As(err, &tooLarge) {
		t.Fatalf("9 bytes past the fallback of 8: %v", err)
	}
}

func TestNoStore(t *testing.T) {
	reply := serve(NoStore(http.NotFoundHandler()), http.MethodGet, "/challenge/new", "")
	if reply.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", reply.Header().Get("Cache-Control"))
	}
}
