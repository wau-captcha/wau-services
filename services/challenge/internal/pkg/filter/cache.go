package filter

import "net/http"

// NoStore keeps every response out of caches: challenges, tokens and verdicts are good for one
// use and one visitor, and a shared cache must never hand them to another.
func NoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
