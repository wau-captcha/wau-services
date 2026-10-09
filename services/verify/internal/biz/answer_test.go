package biz

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestBoundAnswer(t *testing.T) {
	tests := []struct {
		name  string
		field func(*Answer) *string
		sent  string
		want  int
	}{
		{"a site key is cut one past its limit", func(a *Answer) *string { return &a.SiteKey }, strings.Repeat("k", 500), 65},
		{"a site key within the limit is kept", func(a *Answer) *string { return &a.SiteKey }, "pk_" + strings.Repeat("a", 48), 51},
		{"a nonce is cut one past its 20 digits", func(a *Answer) *string { return &a.PowNonce }, strings.Repeat("9", 100), 21},
		{"a User-Agent keeps its first KiB", func(a *Answer) *string { return &a.UserAgent }, strings.Repeat("u", 1<<20), 1024},
		{"Sec-Fetch-Site", func(a *Answer) *string { return &a.SecFetchSite }, strings.Repeat("s", 4096), 64},
		{"Accept-Language", func(a *Answer) *string { return &a.AcceptLanguage }, strings.Repeat("l", 4096), 64},
		// "é" is two bytes: cut at 1024 splits the last one, which is dropped.
		{"a character cut in two is dropped", func(a *Answer) *string { return &a.UserAgent }, "u" + strings.Repeat("é", 600), 1023},
		{"bytes that are not UTF-8 are dropped", func(a *Answer) *string { return &a.UserAgent }, "Mozilla/5.0 \xff\xfe(X11)", 17},
	}
	for _, test := range tests {
		var answer Answer
		*test.field(&answer) = test.sent
		bounded := boundAnswer(answer)
		got := *test.field(&bounded)
		if len(got) != test.want || !utf8.ValidString(got) {
			t.Errorf("%s: %d bytes, valid UTF-8 %v; want %d", test.name, len(got), utf8.ValidString(got), test.want)
		}
	}
	behavior := &Behavior{Pointer: make([]PointerSample, 500)}
	if bounded := boundAnswer(Answer{Behavior: behavior}); len(bounded.Behavior.Pointer) != 201 || len(behavior.Pointer) != 500 {
		t.Errorf("behaviour: %d samples passed on, %d left in the request", len(bounded.Behavior.Pointer), len(behavior.Pointer))
	}
}
