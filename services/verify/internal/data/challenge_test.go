package data

import (
	"context"
	"net/http"
	"net/netip"
	"testing"

	"verify/internal/biz"

	"github.com/go-kratos/kratos/v3/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestChallengeError(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		code   int
		reason string
	}{
		{"refusal of the challenge service is passed on", errors.NotFound("CHALLENGE_NOT_FOUND", "challenge not found"), 404, "CHALLENGE_NOT_FOUND"},
		{"a gRPC status without a reason is not ours", status.Error(codes.InvalidArgument, "challenge_id is not a valid challenge id"), 500, "INTERNAL"},
		{"unreachable service", status.Error(codes.Unavailable, `connection error: desc = "transport: Error while dialing: dial tcp 10.43.0.12:9000"`), 500, "INTERNAL"},
		{"timeout", status.Error(codes.DeadlineExceeded, "context deadline exceeded"), 500, "INTERNAL"},
		{"its own internal error", errors.InternalServer("INTERNAL", "internal error"), 500, "INTERNAL"},
		{"an unknown 4xx reason", errors.BadRequest("SOMETHING_NEW", "redis key too long"), 500, "INTERNAL"},
	}
	// A refusal with its reason, as the challenge service sends it over gRPC.
	withReason := errors.BadRequest("CHALLENGE_ID_INVALID", "challenge_id is not a valid challenge id").GRPCStatus().Err()
	tests = append(tests, struct {
		name   string
		err    error
		code   int
		reason string
	}{"a gRPC status with its reason", withReason, 400, "CHALLENGE_ID_INVALID"})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := errors.FromError(challengeError(context.Background(), test.err))
			if int(got.Code) != test.code || got.Reason != test.reason {
				t.Fatalf("got %d %s %q, want %d %s", got.Code, got.Reason, got.Message, test.code, test.reason)
			}
			if got.Code >= http.StatusInternalServerError && !errors.Is(got, biz.ErrInternal) {
				t.Fatalf("5xx that is not ErrInternal: %v", got)
			}
		})
	}
}

func TestCheckAnswerRequest(t *testing.T) {
	answer := biz.Answer{
		ChallengeID: "0f9c6b5e-1d2a-4c3b-8e7f-9a0b1c2d3e4f", Text: "abcde", SiteKey: "pk_shop", PowNonce: "1233",
		Origin: "https://shop.example.com", Client: netip.MustParseAddr("203.0.113.4"), UserAgent: "Mozilla/5.0",
		SecFetchSite: "cross-site", AcceptLanguage: "vi", Behavior: &biz.Behavior{Version: 1, HeadlessUA: true, ChromeWithoutWindowChrome: true},
	}
	request := checkAnswerRequest(answer)
	if request.GetChallengeId() != answer.ChallengeID || request.GetAnswer() != "abcde" || request.GetSiteKey() != "pk_shop" ||
		request.GetPowNonce() != "1233" || request.GetOrigin() != answer.Origin || request.GetClientIp() != "203.0.113.4" ||
		request.GetUserAgent() != "Mozilla/5.0" || request.GetSecFetchSite() != "cross-site" || request.GetAcceptLanguage() != "vi" {
		t.Fatalf("request %v", request)
	}
	if behavior := request.GetBehavior(); !behavior.GetHeadlessUa() || behavior.GetNoLanguages() || !behavior.GetChromeWithoutWindowChrome() {
		t.Fatalf("hints %v", behavior)
	}
	// Without the address, the challenge service compares nothing of the request.
	answer.Client = netip.Addr{}
	if request := checkAnswerRequest(answer); request.GetClientIp() != "" {
		t.Fatalf("client_ip %q for an unknown address", request.GetClientIp())
	}
}
