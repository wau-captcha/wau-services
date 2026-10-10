package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"challenge/internal/biz"
	"challenge/internal/conf"
	_ "challenge/internal/pkg/codec"
	"challenge/internal/pkg/ratelimit"
	"challenge/internal/service"

	"google.golang.org/protobuf/types/known/durationpb"
)

type stubSites struct{ site biz.Site }

func (s stubSites) FindBySiteKey(_ context.Context, siteKey string) (*biz.Site, error) {
	if siteKey != "pk_shop" {
		return nil, biz.ErrSiteKeyInvalid
	}
	site := s.site
	return &site, nil
}
func (stubSites) SuspendForScraping(context.Context, string) (bool, error)          { return false, nil }
func (stubSites) FlagForScraping(context.Context, string, time.Time) (bool, error)  { return false, nil }
func (stubSites) RecordEvent(context.Context, string, string, map[string]any) error { return nil }

type memoryChallenges struct {
	mu     sync.Mutex
	stored map[string]*biz.Challenge
}

func (m *memoryChallenges) Save(_ context.Context, challenge *biz.Challenge) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stored[challenge.ID] = challenge
	return nil
}

func (m *memoryChallenges) Take(_ context.Context, id string) (*biz.Challenge, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	challenge, ok := m.stored[id]
	if !ok {
		return nil, biz.ErrChallengeNotFound
	}
	delete(m.stored, id)
	return challenge, nil
}

type quietCounters struct{}

func (quietCounters) QuotaUsed(context.Context, string, string) (int64, error) { return 0, nil }
func (quietCounters) CountChallenge(context.Context, string, string) (biz.DailyCounts, error) {
	return biz.DailyCounts{Issued: 1}, nil
}

type noUsage struct{}

func (noUsage) RecordIssued(context.Context, *biz.Challenge) error { return nil }

type openLimiter struct{}

func (openLimiter) Allow(context.Context, ratelimit.Rule, string) time.Duration { return 0 }

type cleanHistory struct{}

func (cleanHistory) WrongAnswers(context.Context, string, time.Time) (int64, error) { return 0, nil }
func (cleanHistory) AutomatedLastHour(context.Context, string, time.Time) (int64, error) {
	return 0, nil
}
func (cleanHistory) CountWrong(context.Context, string, time.Time) error     { return nil }
func (cleanHistory) CountAutomated(context.Context, string, time.Time) error { return nil }

func newChallengeServer(t *testing.T, domains ...string) http.Handler {
	t.Helper()
	site := biz.Site{APIKeyID: "key-1", UserID: "user-1", ChallengeTypes: []string{biz.TypeImageDistort, biz.TypeAnimalJump}, DailyQuota: 100, AllowedDomains: domains}
	usecase, err := biz.NewChallengeUsecase(stubSites{site}, &memoryChallenges{stored: map[string]*biz.Challenge{}}, quietCounters{}, noUsage{},
		openLimiter{}, cleanHistory{}, &conf.Challenge{Ttl: durationpb.New(2 * time.Minute), PowDifficulty: 15},
		&conf.Quota{TimeZone: "Asia/Ho_Chi_Minh"}, &conf.RateLimit{NewPerIp: 30, NewPerSiteKey: 600, NewPerFlaggedSiteKey: 60},
		&conf.Scraping{}, &conf.IPHistory{})
	if err != nil {
		t.Fatal(err)
	}
	serverConf := &conf.Server{Http: &conf.Server_HTTP{}}
	challenge, err := service.NewChallengeService(usecase, serverConf)
	if err != nil {
		t.Fatal(err)
	}
	return NewHTTPServer(serverConf, challenge)
}

func newChallenge(server http.Handler, method string, headers ...string) (*httptest.ResponseRecorder, map[string]any) {
	request := httptest.NewRequest(method, "/challenge/new?site_key=pk_shop", nil)
	request.RemoteAddr = "203.0.113.4:5123"
	for i := 0; i+1 < len(headers); i += 2 {
		request.Header.Set(headers[i], headers[i+1])
	}
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	body := map[string]any{}
	_ = json.Unmarshal(recorder.Body.Bytes(), &body)
	return recorder, body
}

func TestNewChallengeDomainBinding(t *testing.T) {
	tests := []struct {
		name    string
		domains []string
		headers []string
		status  int
		reason  string
		allow   string
		vary    bool
	}{
		{"bound key, its website", []string{"shop.example.com"}, []string{"Origin", "https://shop.example.com"}, 200, "", "https://shop.example.com", true},
		{"bound key, another website", []string{"shop.example.com"}, []string{"Origin", "https://evil.test"}, 403, "DOMAIN_NOT_ALLOWED", "https://evil.test", true},
		{"bound key, no website", []string{"shop.example.com"}, nil, 403, "DOMAIN_NOT_ALLOWED", "*", false},
		{"bound key, Referer only", []string{"shop.example.com"}, []string{"Referer", "https://shop.example.com/signup"}, 200, "", "*", false},
		{"open key, any website", nil, []string{"Origin", "https://evil.test"}, 200, "", "*", false},
	}
	for _, test := range tests {
		recorder, body := newChallenge(newChallengeServer(t, test.domains...), http.MethodGet, test.headers...)
		header := recorder.Header()
		if recorder.Code != test.status || (test.reason != "" && body["reason"] != test.reason) {
			t.Errorf("%s: got %d %s", test.name, recorder.Code, recorder.Body.String())
		}
		if got := header.Get("Access-Control-Allow-Origin"); got != test.allow || len(header.Values("Access-Control-Allow-Origin")) != 1 {
			t.Errorf("%s: Access-Control-Allow-Origin %q, want %q", test.name, header.Values("Access-Control-Allow-Origin"), test.allow)
		}
		if vary := strings.Contains(strings.Join(header.Values("Vary"), ","), "Origin"); vary != test.vary {
			t.Errorf("%s: Vary %q", test.name, header.Values("Vary"))
		}
	}
}

func TestNewChallengeProofOfWork(t *testing.T) {
	recorder, body := newChallenge(newChallengeServer(t), http.MethodGet, "Origin", "https://shop.example.com")
	if recorder.Code != 200 {
		t.Fatalf("got %d %s", recorder.Code, recorder.Body.String())
	}
	salt, _ := body["pow_salt"].(string)
	decoded, err := base64.StdEncoding.DecodeString(salt)
	if err != nil || len(decoded) != 16 || body["pow_difficulty"] != float64(15) {
		t.Fatalf("pow_salt %q (%d bytes, %v), pow_difficulty %v", salt, len(decoded), err, body["pow_difficulty"])
	}
	if _, ok := body["piece_y"]; ok {
		t.Fatal("piece fields on an image_distort challenge")
	}
}

func TestNewChallengePreflight(t *testing.T) {
	recorder, _ := newChallenge(newChallengeServer(t, "shop.example.com"), http.MethodOptions,
		"Origin", "https://shop.example.com", "Access-Control-Request-Method", "GET")
	header := recorder.Header()
	if recorder.Code != http.StatusNoContent || header.Get("Access-Control-Allow-Origin") != "*" || header.Get("Access-Control-Allow-Methods") != "GET, OPTIONS" {
		t.Fatalf("got %d %v", recorder.Code, header)
	}
}
