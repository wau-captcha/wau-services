package server

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"challenge/internal/biz"

	"github.com/go-kratos/kratos/v3/errors"
)

func TestSanitizeErrors(t *testing.T) {
	redisFailure := stderrors.New("ERR Request contains a key that is longer than the max key size of 4096 bytes")
	tests := []struct {
		name   string
		err    error
		code   int
		reason string
	}{
		{"redis error", redisFailure, 500, "INTERNAL"},
		{"wrapped redis error", fmt.Errorf("take challenge: %w", redisFailure), 500, "INTERNAL"},
		{"foreign 5xx", errors.ServiceUnavailable("", "connection error: dial tcp 10.43.0.12:9000"), 500, "INTERNAL"},
		{"our internal error", biz.ErrInternal, 500, "INTERNAL"},
		{"our 4xx", biz.ErrChallengeNotFound, 404, "CHALLENGE_NOT_FOUND"},
		{"validation", errors.BadRequest("VALIDATOR", "missing field site_key"), 400, "VALIDATOR"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := sanitizeErrors()(func(context.Context, any) (any, error) { return nil, test.err })
			_, err := handler(context.Background(), nil)
			got := errors.FromError(err)
			if int(got.Code) != test.code || got.Reason != test.reason {
				t.Fatalf("got %d %s %q, want %d %s", got.Code, got.Reason, got.Message, test.code, test.reason)
			}
			if test.reason == "INTERNAL" && got.Message != "internal error" {
				t.Fatalf("message %q leaks the cause", got.Message)
			}
		})
	}
}

func TestEncodeError(t *testing.T) {
	t.Run("a raw error is hidden", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		encodeError(recorder, httptest.NewRequest(http.MethodGet, "/challenge/new", nil), stderrors.New("redis: connection pool timeout"))
		var body struct {
			Code            int
			Reason, Message string
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if recorder.Code != 500 || body.Reason != "INTERNAL" || body.Message != "internal error" || strings.Contains(recorder.Body.String(), "redis") {
			t.Fatalf("got %d %s", recorder.Code, recorder.Body.String())
		}
	})
	t.Run("429 carries Retry-After", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		err := biz.ErrRateLimited.WithMetadata(map[string]string{"retry_after": "37"})
		encodeError(recorder, httptest.NewRequest(http.MethodGet, "/challenge/new", nil), err)
		if recorder.Code != 429 || recorder.Header().Get("Retry-After") != "37" || !strings.Contains(recorder.Body.String(), `"retry_after":"37"`) {
			t.Fatalf("got %d %v %s", recorder.Code, recorder.Header(), recorder.Body.String())
		}
	})
}
