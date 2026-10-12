package biz

// SlideTolerancePx is how far from the hole the left edge of a slide_puzzle piece may end and
// the answer still be right; the drag itself must also move like a hand.
const SlideTolerancePx = slideAimSlackPx

// Sample makes one challenge of challengeType with the generator /challenge/new uses, for
// cmd/samples: the stored answer and the media a client gets. Nothing is saved. auto_check has no
// media to sample.
func Sample(challengeType string) (answer string, media Media, err error) {
	kind, ok := challengeKinds[challengeType]
	if !ok || challengeType == TypeAutoCheck {
		return "", Media{}, ErrChallengeTypeUnsupported
	}
	return kind.generate()
}
