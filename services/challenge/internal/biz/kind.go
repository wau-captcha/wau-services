package biz

import (
	"strings"
	"time"
)

// challengeKind is how a challenge type makes its media together with the answer it stores, and
// how it checks an answer against that stored answer.
type challengeKind struct {
	generate func() (answer string, media Media, err error)
	// check reports whether answer solves a challenge issued sinceIssue ago that stored expected.
	check func(answer, expected string, sinceIssue time.Duration) bool
}

var challengeKinds = map[string]challengeKind{
	TypeImageDistort: textKind(alphanumericChars, 5, renderImageDistort),
	TypeMotionNoise:  textKind(distinctChars, 4, renderMotionNoise),
	TypeSlidePuzzle:  {generate: generateSlidePuzzle, check: checkSlidePuzzle},
}

// textKind is a type whose answer is a random code drawn into the media and typed back,
// in any case.
func textKind(chars string, length int, render func(code string) (Media, error)) challengeKind {
	return challengeKind{
		generate: func() (string, Media, error) {
			code := newCode(chars, length)
			media, err := render(code)
			return code, media, err
		},
		check: func(answer, expected string, _ time.Duration) bool {
			return strings.EqualFold(strings.TrimSpace(answer), expected)
		},
	}
}
