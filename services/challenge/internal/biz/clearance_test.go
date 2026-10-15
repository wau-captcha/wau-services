package biz

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"challenge/internal/conf"
	"challenge/internal/pkg/quota"
)

// testKeyID is an API key id as Postgres makes them.
const testKeyID = "2d6f0c8e-4b1a-4e9b-9c3d-7f1e2a3b4c5d"

func TestClearanceRoundTrip(t *testing.T) {
	key, now := clearanceKey(testTokenSecret), time.Unix(1_760_000_000, 0)
	text, err := issueClearance(key, testKeyID, chromeUA, now, 168*time.Hour)
	if err != nil || len(text) >= 200 || strings.Count(text, ".") != 1 {
		t.Fatalf("issued %q (%d characters), %v", text, len(text), err)
	}
	got, ok := parseClearance(key, text, testKeyID, chromeUA, now.Add(168*time.Hour-time.Second))
	if !ok || len(got.id) != 2*clearanceIDSize || !got.issuedAt.Equal(now) || !got.expiresAt.Equal(now.Add(168*time.Hour)) {
		t.Fatalf("parsed %+v, %v", got, ok)
	}
	again, _ := issueClearance(key, testKeyID, chromeUA, now, 168*time.Hour)
	if other, _ := parseClearance(key, again, testKeyID, chromeUA, now); other.id == got.id {
		t.Fatal("two clearances share an id")
	}
	// The User-Agent counts as verify passes it on: its first maxUserAgentBytes.
	long := chromeUA + strings.Repeat("x", maxUserAgentBytes)
	text, _ = issueClearance(key, testKeyID, long, now, time.Hour)
	if _, ok := parseClearance(key, text, testKeyID, long[:maxUserAgentBytes]+"y", now); !ok {
		t.Fatal("a User-Agent compared past what verify passes on")
	}
	// The longest key id still makes a clearance under 200 characters; a longer one or none makes none.
	if text, err := issueClearance(key, strings.Repeat("k", maxClearanceKeyID), chromeUA, now, time.Hour); err != nil || len(text) >= 200 {
		t.Fatalf("a 64-character key id: %d characters, %v", len(text), err)
	}
	for _, keyID := range []string{"", strings.Repeat("k", maxClearanceKeyID+1)} {
		if _, err := issueClearance(key, keyID, chromeUA, now, time.Hour); err == nil {
			t.Errorf("a clearance for key id %q", keyID)
		}
	}
}

func TestClearanceCountsAsNone(t *testing.T) {
	key, now := clearanceKey(testTokenSecret), time.Unix(1_760_000_000, 0)
	valid, _ := issueClearance(key, testKeyID, chromeUA, now, time.Hour)
	encoded, signature, _ := strings.Cut(valid, ".")
	payload, _ := base64.RawURLEncoding.DecodeString(encoded)
	signedWith := func(key []byte, payload []byte) string {
		encoded := base64.RawURLEncoding.EncodeToString(payload)
		return encoded + "." + base64.RawURLEncoding.EncodeToString(clearanceMAC(key, encoded))
	}
	changed := func(change func([]byte)) []byte {
		copied := slices.Clone(payload)
		change(copied)
		return copied
	}
	// A verify token: base64url(id | expiry) "." base64url(HMAC-SHA256 with the secret itself).
	tokenPayload := binary.BigEndian.AppendUint64(make([]byte, 16), uint64(now.Add(time.Hour).Unix()))
	tests := []struct {
		name, text, keyID, userAgent string
		at                           time.Time
	}{
		{"forged signature", signedWith([]byte("another secret"), payload), testKeyID, chromeUA, now},
		{"signed with the token secret itself", signedWith([]byte(testTokenSecret), payload), testKeyID, chromeUA, now},
		{"a verify token", signedWith([]byte(testTokenSecret), tokenPayload), testKeyID, chromeUA, now},
		{"another payload under the signature", base64.RawURLEncoding.EncodeToString(changed(func(p []byte) { p[1] ^= 1 })) + "." + signature,
			testKeyID, chromeUA, now},
		{"expired", valid, testKeyID, chromeUA, now.Add(time.Hour)},
		{"another key's", valid, "3e7a1d9f-5c2b-4f0a-8d4e-6a1b2c3d4e5f", chromeUA, now},
		{"another User-Agent's", valid, testKeyID, strings.Replace(chromeUA, "Chrome/141.0.0.0", "Chrome/142.0.0.0", 1), now},
		{"truncated", valid[:len(valid)-4], testKeyID, chromeUA, now},
		{"payload only", encoded, testKeyID, chromeUA, now},
		{"oversized", valid + strings.Repeat("A", 40), testKeyID, chromeUA, now},
		{"over 256 characters", valid + "." + strings.Repeat("A", maxClearanceLength), testKeyID, chromeUA, now},
		{"signed, with a byte more than its key id needs", signedWith(key, append(slices.Clone(payload), 0)), testKeyID, chromeUA, now},
		{"signed, cut short", signedWith(key, payload[:clearanceFixedSize-1]), testKeyID, chromeUA, now},
		{"signed, another version", signedWith(key, changed(func(p []byte) { p[0] = 2 })), testKeyID, chromeUA, now},
		{"not base64", "!!!." + signature, testKeyID, chromeUA, now},
		{"empty", "", testKeyID, chromeUA, now},
	}
	for _, test := range tests {
		if _, ok := parseClearance(key, test.text, test.keyID, test.userAgent, test.at); ok {
			t.Errorf("%s: accepted", test.name)
		}
	}
	// So each case above fails for its own reason.
	if _, ok := parseClearance(key, valid, testKeyID, chromeUA, now.Add(time.Hour-time.Second)); !ok {
		t.Fatal("the valid clearance refused")
	}
	if string(key) == testTokenSecret || len(key) != 32 {
		t.Fatal("the clearance key is not derived from the token secret")
	}
}

// slideIssued is a slide_puzzle challenge as New stores it, its hole at x = 150.
func slideIssued(now time.Time) *Challenge {
	challenge := issued(now)
	challenge.Type, challenge.Answer = TypeSlidePuzzle, "150"
	return challenge
}

// slideDrag is a drag of the piece to x as a hand makes it: it speeds up, brakes to aim, wobbles up
// and down, and its events come unevenly.
func slideDrag(x float64) string {
	var points [][3]float64
	t := 0.0
	for i := range 40 {
		u := float64(i) / 39
		points = append(points, [3]float64{t, x * u * u * u * (u*(6*u-15) + 10), 1.5 * math.Sin(7*u)})
		t += 14 + float64(i%4)*3
	}
	data, _ := json.Marshal(map[string]any{"x": x, "t": points})
	return string(data)
}

func TestClearanceIssued(t *testing.T) {
	key := clearanceKey(testTokenSecret)
	slide := func(f *fixture, change func(*Challenge, *Answer)) *Outcome {
		t.Helper()
		challenge, answer := slideIssued(f.now), answered()
		answer.Text = slideDrag(150)
		if change != nil {
			change(challenge, &answer)
		}
		f.challenges.stored[testChallengeID] = challenge
		outcome, err := f.usecase.Check(context.Background(), answer)
		if err != nil {
			t.Fatal(err)
		}
		return outcome
	}
	// A right slide_puzzle answer earns one, for its key and its browser.
	f := newFixture(nil)
	outcome := slide(f, nil)
	if !outcome.Right {
		t.Fatalf("the drag was not right: %+v", outcome.Behavior)
	}
	if _, ok := parseClearance(key, outcome.Clearance, "key-1", chromeUA, f.now.Add(167*time.Hour)); !ok {
		t.Fatalf("clearance %q does not hold for the key and the browser", outcome.Clearance)
	}
	// It lets the browser pass auto_check on the key, with reason cleared.
	answer := autoAnswered()
	answer.Text = autoAnswerWith(outcome.Clearance, nil)
	if got := checkAuto(t, newFixture(nil, func(c *conf.AutoCheck) { c.Mode = "strict" }), visitor, answer); !got.Right ||
		!slices.Equal(got.Behavior.Reasons, []string{"cleared"}) {
		t.Fatalf("strict with the clearance: pass %v, %q", got.Right, got.Behavior.Reasons)
	}

	for name, change := range map[string]func(*Challenge, *Answer){
		"wrong":     func(_ *Challenge, a *Answer) { a.Text = slideDrag(100) },
		"automated": func(_ *Challenge, a *Answer) { a.UserAgent = "python-requests/2.32.3" },
		"from a hosting address": func(c *Challenge, a *Answer) {
			c.ClientIP, a.Client = hostingVisitor.String(), hostingVisitor
		},
		"through a verify that passes no request": func(_ *Challenge, a *Answer) { a.Client, a.Origin, a.UserAgent = netip.Addr{}, "", "" },
	} {
		outcome := slide(newFixture(nil), change)
		if outcome.Clearance != "" {
			t.Errorf("%s: a clearance with a %v answer", name, outcome.Right)
		}
		if name == "from a hosting address" && !outcome.Right {
			t.Errorf("%s: the answer itself still counts", name)
		}
	}
	// Turning the hosting rule off lets a hosting address earn one too.
	open := newFixture(nil, func(c *conf.AutoCheck) { c.BlockHostingAddresses = false })
	if outcome := slide(open, func(c *Challenge, a *Answer) { c.ClientIP, a.Client = hostingVisitor.String(), hostingVisitor }); outcome.Clearance == "" {
		t.Error("no clearance for a hosting address with block_hosting_addresses off")
	}
	// An auto_check pass earns none.
	if got := checkAuto(t, newFixture(nil), visitor, autoAnswered()); !got.Right || got.Clearance != "" {
		t.Errorf("auto_check pass %v with clearance %q", got.Right, got.Clearance)
	}
}

func TestAutoCheckClearance(t *testing.T) {
	key := clearanceKey(testTokenSecret)
	issue := func(keyID, userAgent string, issuedAt time.Time) string {
		text, err := issueClearance(key, keyID, userAgent, issuedAt, 168*time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		return text
	}
	valid := func(f *fixture) string { return issue("key-1", chromeUA, f.now.Add(-time.Hour)) }
	idOf := func(f *fixture, text string) string {
		cleared, ok := parseClearance(key, text, "key-1", chromeUA, f.now)
		if !ok {
			t.Fatal("not a valid clearance")
		}
		return cleared.id
	}
	day := func(f *fixture) string { return quota.Day(f.now, f.usecase.location) }
	tests := []struct {
		name      string
		mode      string
		clearance func(f *fixture) string
		counts    func(f *fixture, id string)
		pass      bool
		reasons   []string
	}{
		{"valid", "strict", valid, nil, true, []string{"cleared"}},
		{"valid, balanced", "balanced", valid, nil, true, []string{"cleared"}},
		{"under both caps", "strict", valid, func(f *fixture, id string) { f.clearances.hour[id], f.clearances.day[id+" "+day(f)] = 5, 23 },
			true, []string{"cleared"}},
		{"forged", "strict", func(f *fixture) string {
			text, _ := issueClearance(clearanceKey("another secret"), "key-1", chromeUA, f.now, time.Hour)
			return text
		}, nil, false, []string{"clearance_invalid", "clearance_required"}},
		{"expired", "strict", func(f *fixture) string { return issue("key-1", chromeUA, f.now.Add(-168*time.Hour)) }, nil,
			false, []string{"clearance_invalid", "clearance_required"}},
		{"another key's", "strict", func(f *fixture) string { return issue("key-2", chromeUA, f.now) }, nil,
			false, []string{"clearance_invalid", "clearance_required"}},
		{"another User-Agent's, after a browser update", "strict", func(f *fixture) string {
			return issue("key-1", strings.Replace(chromeUA, "Chrome/141", "Chrome/140", 1), f.now)
		}, nil, false, []string{"clearance_invalid", "clearance_required"}},
		{"truncated", "strict", func(f *fixture) string { text := valid(f); return text[:len(text)-1] }, nil,
			false, []string{"clearance_invalid", "clearance_required"}},
		{"oversized", "strict", func(f *fixture) string { return valid(f) + strings.Repeat("A", 50) }, nil,
			false, []string{"clearance_invalid", "clearance_required"}},
		{"over its hourly cap", "strict", valid, func(f *fixture, id string) { f.clearances.hour[id] = 6 },
			false, []string{"clearance_cap", "clearance_required"}},
		{"over its daily cap", "strict", valid, func(f *fixture, id string) { f.clearances.day[id+" "+day(f)] = 24 },
			false, []string{"clearance_cap", "clearance_required"}},
		{"Redis down", "strict", valid, func(f *fixture, _ string) { f.clearances.err = errors.New("redis down") },
			false, []string{"clearance_cap", "clearance_required"}},
		// In balanced mode one that counts as none leaves a first visit.
		{"invalid, balanced", "balanced", func(f *fixture) string { return issue("key-2", chromeUA, f.now) }, nil,
			true, []string{"clearance_invalid", "first_visit"}},
		{"over its hourly cap, balanced", "balanced", valid, func(f *fixture, id string) { f.clearances.hour[id] = 6 },
			true, []string{"clearance_cap", "first_visit"}},
		// A hosting address never passes, whatever its clearance.
		{"valid, from a hosting address", "strict", valid, nil, false, []string{"hosting_address"}},
	}
	for _, test := range tests {
		f := newFixture(nil, func(c *conf.AutoCheck) { c.Mode = test.mode })
		text := test.clearance(f)
		var id string
		if test.counts != nil {
			id = idOf(f, text)
			test.counts(f, id)
		}
		hourBefore, dayBefore := f.clearances.hour[id], f.clearances.day[id+" "+day(f)]
		answer := autoAnswered()
		answer.Text = autoAnswerWith(text, nil)
		client := visitor
		if strings.Contains(test.name, "hosting") {
			client = hostingVisitor
		}
		got := checkAuto(t, f, client, answer)
		if got.Right != test.pass || !slices.Equal(got.Behavior.Reasons, test.reasons) {
			t.Errorf("%s: got pass %v, %q; want %v, %q", test.name, got.Right, got.Behavior.Reasons, test.pass, test.reasons)
			continue
		}
		// A pass counts against the clearance that gave it, and only such a pass.
		cleared := slices.Contains(test.reasons, "cleared")
		if cleared && id == "" {
			id = idOf(f, text)
			hourBefore, dayBefore = 0, 0
		}
		counted := int64(0)
		if cleared {
			counted = 1
		}
		if id != "" && f.clearances.err == nil &&
			(f.clearances.hour[id] != hourBefore+counted || f.clearances.day[id+" "+day(f)] != dayBefore+counted) {
			t.Errorf("%s: clearance passes %d and %d, from %d and %d", test.name, f.clearances.hour[id], f.clearances.day[id+" "+day(f)],
				hourBefore, dayBefore)
		}
		if visits := f.history.firstVisits[visitor.String()]; (visits == 1) != slices.Contains(test.reasons, "first_visit") {
			t.Errorf("%s: %d first visits counted", test.name, visits)
		}
	}
}
