// Package filter holds the net/http middleware in front of the public API: CORS, the cap on
// request bodies and the cache policy. challenge and verify each keep a copy of this package,
// billing keeps limits.go: keep the copies identical.
package filter

import "net/http"

// Route is how a path answers browsers calling it from other origins.
type Route struct {
	// Methods the preflight allows, e.g. "GET, OPTIONS".
	Methods string
	// ReflectOrigin answers with the caller's own Origin, and Vary: Origin, instead of "*".
	ReflectOrigin bool
}

// CORS lets site frontends call the routes listed from their own origin, and no other path:
// /siteverify, which only servers call, and /healthz get no Access-Control-* header at all, so
// browsers refuse to send them or to read the reply. Only Content-Type may be sent, never
// Authorization; Retry-After may be read, for the SDKs to say how long to wait. A handler may
// still set its own Access-Control-Allow-Origin, which wins.
func CORS(routes map[string]Route) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route, ok := routes[r.URL.Path]
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			header := w.Header()
			if origin := r.Header.Get("Origin"); route.ReflectOrigin && origin != "" {
				header.Set("Access-Control-Allow-Origin", origin)
				header.Add("Vary", "Origin")
			} else {
				header.Set("Access-Control-Allow-Origin", "*")
			}
			if r.Method == http.MethodOptions {
				header.Set("Access-Control-Allow-Methods", route.Methods)
				header.Set("Access-Control-Allow-Headers", "Content-Type")
				header.Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			header.Set("Access-Control-Expose-Headers", "Retry-After")
			next.ServeHTTP(w, r)
		})
	}
}
