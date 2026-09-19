package server

import (
	"testing"
	"time"

	"gateway/internal/conf"

	"google.golang.org/protobuf/types/known/durationpb"
)

func TestNewHandlerReadsCommaSeparatedTrustedProxies(t *testing.T) {
	bc := &conf.Bootstrap{
		Routes: []*conf.Route{{Prefix: "/v1/", Upstream: "http://127.0.0.1:8002"}},
		RateLimit: &conf.RateLimit{
			Prefixes:       []string{"/v1/auth/"},
			Requests:       20,
			Window:         durationpb.New(time.Minute),
			TrustedProxies: "127.0.0.0/8, ::1/128",
		},
	}
	if _, err := NewHandler(bc); err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	bc.RateLimit.TrustedProxies = "127.0.0.0/8,not-a-cidr"
	if _, err := NewHandler(bc); err == nil {
		t.Fatal("want an error for an invalid trusted proxy")
	}
}
