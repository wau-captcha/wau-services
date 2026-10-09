package filter

import "net/http"

// LimitBody caps request bodies, per path and at fallback for any other path. Reading past the
// cap fails with *http.MaxBytesError, which the request decoders answer with 413
// PAYLOAD_TOO_LARGE; an oversized body is never read whole, nor answered with a 500.
func LimitBody(limits map[string]int64, fallback int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limit, ok := limits[r.URL.Path]
			if !ok {
				limit = fallback
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}
