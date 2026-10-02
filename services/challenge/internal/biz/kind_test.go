package biz

import (
	"context"
	"strconv"
	"testing"
	"time"

	"challenge/internal/conf"

	"google.golang.org/protobuf/types/known/durationpb"
)

func TestTextKindsIgnoreCase(t *testing.T) {
	for _, kindType := range []string{TypeImageDistort, TypeMotionNoise} {
		kind := challengeKinds[kindType]
		for _, test := range []struct {
			answer, expected string
			right            bool
		}{
			{"aB3dE", "Ab3De", true},
			{"  ab3de\n", "AB3DE", true},
			{"AB3DE", "AB3DE", true},
			{"AB3DF", "AB3DE", false},
			{"AB3D", "AB3DE", false},
			{"", "AB3DE", false},
		} {
			// Text answers do not depend on how fast they came back.
			if got := kind.check(test.answer, test.expected, 0); got != test.right {
				t.Errorf("%s: %q against %q got %v, want %v", kindType, test.answer, test.expected, got, test.right)
			}
		}
		answer, media, err := kind.generate()
		if err != nil {
			t.Fatal(err)
		}
		if media.Piece != nil || !kind.check(answer, answer, 0) {
			t.Errorf("%s: generated a piece, or its own answer %q does not check", kindType, answer)
		}
	}
}

type takeOnce map[string]*Challenge

func (r takeOnce) Save(_ context.Context, c *Challenge) error { r[c.ID] = c; return nil }

func (r takeOnce) Take(_ context.Context, id string) (*Challenge, error) {
	challenge, ok := r[id]
	if !ok {
		return nil, ErrChallengeNotFound
	}
	delete(r, id)
	return challenge, nil
}

func TestCheck(t *testing.T) {
	now := time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	trail := humanTrail(11, testTarget, 900)
	drag := encode(trail, lastX(trail))
	tests := []struct {
		name      string
		challenge Challenge
		answer    string
		right     bool
	}{
		{"image_distort, other case", Challenge{Type: TypeImageDistort, Answer: "Xy7Qz", IssuedAt: now}, "xY7qZ", true},
		{"motion_noise, other case", Challenge{Type: TypeMotionNoise, Answer: "K7RB", IssuedAt: now}, " k7rb ", true},
		{"motion_noise, wrong", Challenge{Type: TypeMotionNoise, Answer: "K7RB", IssuedAt: now}, "K7RV", false},
		{"slide_puzzle, human drag", Challenge{Type: TypeSlidePuzzle, Answer: strconv.Itoa(testTarget), IssuedAt: now.Add(-3 * time.Second)}, drag, true},
		{"slide_puzzle, checked 200 ms after issue", Challenge{Type: TypeSlidePuzzle, Answer: strconv.Itoa(testTarget), IssuedAt: now.Add(-200 * time.Millisecond)}, drag, false},
		// Stored by the previous version during a rollout: counts as issued long ago.
		{"slide_puzzle, no issue time", Challenge{Type: TypeSlidePuzzle, Answer: strconv.Itoa(testTarget)}, drag, true},
		{"slide_puzzle, text answer", Challenge{Type: TypeSlidePuzzle, Answer: strconv.Itoa(testTarget), IssuedAt: now.Add(-3 * time.Second)}, "150", false},
		{"unknown type", Challenge{Type: "puzzle_v9", Answer: "AB3DE", IssuedAt: now}, "AB3DE", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			challenges := takeOnce{}
			usecase, err := NewChallengeUsecase(nil, challenges, nil, nil,
				&conf.Challenge{Ttl: durationpb.New(2 * time.Minute)}, &conf.Quota{TimeZone: "Asia/Ho_Chi_Minh"})
			if err != nil {
				t.Fatal(err)
			}
			usecase.now = func() time.Time { return now }
			stored := test.challenge
			stored.ID = "c1"
			_ = challenges.Save(context.Background(), &stored)

			_, right, err := usecase.Check(context.Background(), "c1", test.answer)
			if err != nil || right != test.right {
				t.Fatalf("got right=%v err=%v, want right=%v", right, err, test.right)
			}
			// Each challenge is answered once, right or wrong.
			if _, _, err := usecase.Check(context.Background(), "c1", test.answer); err != ErrChallengeNotFound {
				t.Fatalf("second check: got %v, want ErrChallengeNotFound", err)
			}
		})
	}
}
