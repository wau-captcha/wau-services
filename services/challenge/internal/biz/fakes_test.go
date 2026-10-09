package biz

import (
	"context"
	"time"

	"challenge/internal/conf"
	"challenge/internal/pkg/ratelimit"

	"google.golang.org/protobuf/types/known/durationpb"
)

// Fakes of the repositories, in memory, for the tests of this package.

type fakeSites struct {
	site      *Site
	suspended int
	flagged   int
	events    []string
}

func (f *fakeSites) FindBySiteKey(context.Context, string) (*Site, error) {
	if f.site == nil {
		return nil, ErrSiteKeyInvalid
	}
	copied := *f.site
	return &copied, nil
}

func (f *fakeSites) SuspendForScraping(context.Context, string) (bool, error) {
	f.suspended++
	return f.suspended == 1, nil
}

func (f *fakeSites) FlagForScraping(context.Context, string, time.Time) (bool, error) {
	f.flagged++
	return true, nil
}

func (f *fakeSites) RecordEvent(_ context.Context, _ string, action string, _ map[string]any) error {
	f.events = append(f.events, action)
	return nil
}

type fakeChallenges struct {
	stored map[string]*Challenge
	takes  int
}

func (f *fakeChallenges) Save(_ context.Context, challenge *Challenge) error {
	f.stored[challenge.ID] = challenge
	return nil
}

func (f *fakeChallenges) Take(_ context.Context, id string) (*Challenge, error) {
	f.takes++
	challenge, ok := f.stored[id]
	if !ok {
		return nil, ErrChallengeNotFound
	}
	delete(f.stored, id)
	return challenge, nil
}

type fakeCounters struct {
	used   int64
	counts DailyCounts
}

func (f *fakeCounters) QuotaUsed(context.Context, string, string) (int64, error) { return f.used, nil }
func (f *fakeCounters) CountChallenge(context.Context, string, string) (DailyCounts, error) {
	f.counts.Issued++
	return f.counts, nil
}

type fakeUsage struct{}

func (fakeUsage) RecordIssued(context.Context, *Challenge) error { return nil }

// fakeLimiter remembers the rules it was asked about and refuses the scopes in refuse.
type fakeLimiter struct {
	rules  []ratelimit.Rule
	refuse map[string]bool
}

func (f *fakeLimiter) Allow(_ context.Context, rule ratelimit.Rule, _ string) time.Duration {
	f.rules = append(f.rules, rule)
	if f.refuse[rule.Scope] {
		return 1500 * time.Millisecond
	}
	return 0
}

type fakeHistory struct {
	wrong, automated map[string]int64
}

func (f *fakeHistory) WrongAnswers(_ context.Context, addr string, _ time.Time) (int64, error) {
	return f.wrong[addr], nil
}
func (f *fakeHistory) AutomatedLastHour(_ context.Context, addr string, _ time.Time) (int64, error) {
	return f.automated[addr], nil
}
func (f *fakeHistory) CountWrong(_ context.Context, addr string, _ time.Time) error {
	f.wrong[addr]++
	return nil
}
func (f *fakeHistory) CountAutomated(_ context.Context, addr string, _ time.Time) error {
	f.automated[addr]++
	return nil
}

type fixture struct {
	usecase    *ChallengeUsecase
	sites      *fakeSites
	challenges *fakeChallenges
	counters   *fakeCounters
	limiter    *fakeLimiter
	history    *fakeHistory
	now        time.Time
}

// newFixture is a usecase over fakes with the limits of configs/config.yaml and a fixed clock.
func newFixture(site *Site) *fixture {
	f := &fixture{
		sites:      &fakeSites{site: site},
		challenges: &fakeChallenges{stored: map[string]*Challenge{}},
		counters:   &fakeCounters{},
		limiter:    &fakeLimiter{refuse: map[string]bool{}},
		history:    &fakeHistory{wrong: map[string]int64{}, automated: map[string]int64{}},
		now:        time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC),
	}
	usecase, err := NewChallengeUsecase(f.sites, f.challenges, f.counters, fakeUsage{}, f.limiter, f.history,
		&conf.Challenge{Ttl: durationpb.New(2 * time.Minute), PowDifficulty: 8},
		&conf.Quota{TimeZone: "Asia/Ho_Chi_Minh"},
		&conf.RateLimit{NewPerIp: 30, NewPerSiteKey: 600, NewPerFlaggedSiteKey: 60},
		&conf.Scraping{FlagMinIssued: 500, FlagMaxAnsweredPercent: 10, FlagMinAnswered: 200, FlagMaxSolvedPercent: 10,
			FlagMaxAutomatedPercent: 50, SuspendMinIssued: 5000, SuspendMaxAnsweredPercent: 5},
		&conf.IPHistory{WrongPer_10Minutes: 20, AutomatedPerHour: 5})
	if err != nil {
		panic(err)
	}
	usecase.now = func() time.Time { return f.now }
	f.usecase = usecase
	return f
}
