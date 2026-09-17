// Package httpx writes gateway errors in the JSON shape the services use, so
// clients parse one error format no matter who answered.
package httpx

import (
	"encoding/json"
	"net/http"
)

type errorBody struct {
	Code     int               `json:"code"`
	Reason   string            `json:"reason"`
	Message  string            `json:"message"`
	Metadata map[string]string `json:"metadata"`
}

// WriteError replies with {code, reason, message, metadata}.
func WriteError(w http.ResponseWriter, status int, reason, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorBody{
		Code:     status,
		Reason:   reason,
		Message:  message,
		Metadata: map[string]string{},
	})
}
