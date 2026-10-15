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
	wrong, automated, passes, firstVisits map[string]int64
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
func (f *fakeHistory) AutoPassesLastHour(_ context.Context, addr string, _ time.Time) (int64, error) {
	return f.passes[addr], nil
}
func (f *fakeHistory) CountAutoPass(_ context.Context, addr string, _ time.Time) error {
	f.passes[addr]++
	return nil
}
func (f *fakeHistory) FirstVisitPassesLastHour(_ context.Context, addr string, _ time.Time) (int64, error) {
	return f.firstVisits[addr], nil
}
func (f *fakeHistory) CountFirstVisitPass(_ context.Context, addr string, _ time.Time) error {
	f.firstVisits[addr]++
	return nil
}

// fakeClearances counts the passes of each clearance in the last hour and per id and day; err
// stands for a failure of Redis.
type fakeClearances struct {
	hour, day map[string]int64
	err       error
}

func (f *fakeClearances) Passes(_ context.Context, id string, _ time.Time, day string) (int64, int64, error) {
	if f.err != nil {
		return 0, 0, f.err
	}
	return f.hour[id], f.day[id+" "+day], nil
}

func (f *fakeClearances) CountPass(_ context.Context, id string, _ time.Time, day string) error {
	if f.err != nil {
		return f.err
	}
	f.hour[id]++
	f.day[id+" "+day]++
	return nil
}

// testTokenSecret stands for KRATOS_TOKEN_SECRET.
const testTokenSecret = "test-token-secret"

// testAutoConf is the auto_check section of configs/config.yaml.
func testAutoConf() *conf.AutoCheck {
	return &conf.AutoCheck{PowDifficulty: 18, MinScorePercent: 80, PassesPerIpHour: 30, Mode: "balanced",
		FirstVisitPassesPerIpHour: 5, ClearanceTtl: durationpb.New(168 * time.Hour), ClearancePassesPerHour: 6,
		ClearancePassesPerDay: 24, BlockHostingAddresses: true, ClearanceSecret: testTokenSecret}
}

type fixture struct {
	usecase    *ChallengeUsecase
	sites      *fakeSites
	challenges *fakeChallenges
	counters   *fakeCounters
	limiter    *fakeLimiter
	history    *fakeHistory
	clearances *fakeClearances
	now        time.Time
}

// newFixture is a usecase over fakes with the limits of configs/config.yaml and a fixed clock;
// each tune changes the auto_check settings first.
func newFixture(site *Site, tune ...func(*conf.AutoCheck)) *fixture {
	f := &fixture{
		sites:      &fakeSites{site: site},
		challenges: &fakeChallenges{stored: map[string]*Challenge{}},
		counters:   &fakeCounters{},
		limiter:    &fakeLimiter{refuse: map[string]bool{}},
		history: &fakeHistory{wrong: map[string]int64{}, automated: map[string]int64{}, passes: map[string]int64{},
			firstVisits: map[string]int64{}},
		clearances: &fakeClearances{hour: map[string]int64{}, day: map[string]int64{}},
		now:        time.Date(2026, 10, 8, 3, 0, 0, 0, time.UTC),
	}
	autoConf := testAutoConf()
	for _, change := range tune {
		change(autoConf)
	}
	usecase, err := NewChallengeUsecase(f.sites, f.challenges, f.counters, fakeUsage{}, f.limiter, f.history, f.clearances,
		&conf.Challenge{Ttl: durationpb.New(2 * time.Minute), PowDifficulty: 8},
		&conf.Quota{TimeZone: "Asia/Ho_Chi_Minh"},
		&conf.RateLimit{NewPerIp: 30, NewPerSiteKey: 600, NewPerFlaggedSiteKey: 60},
		&conf.Scraping{FlagMinIssued: 500, FlagMaxAnsweredPercent: 10, FlagMinAnswered: 200, FlagMaxSolvedPercent: 10,
			FlagMaxAutomatedPercent: 50, SuspendMinIssued: 5000, SuspendMaxAnsweredPercent: 5},
		&conf.IPHistory{WrongPer_10Minutes: 20, AutomatedPerHour: 5},
		autoConf)
	if err != nil {
		panic(err)
	}
	usecase.now = func() time.Time { return f.now }
	f.usecase = usecase
	return f
}
