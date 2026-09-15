package service

import (
	"context"

	"challenge/internal/biz"

	v1 "common/proto/challenge/v1"
)

// ChallengeService implements the challenge API.
type ChallengeService struct {
	v1.UnimplementedChallengeServiceServer

	uc *biz.ChallengeUsecase
}

// NewChallengeService creates a ChallengeService.
func NewChallengeService(uc *biz.ChallengeUsecase) *ChallengeService {
	return &ChallengeService{uc: uc}
}

// NewChallenge issues a challenge for a site key.
func (s *ChallengeService) NewChallenge(ctx context.Context, req *v1.NewChallengeRequest) (*v1.Challenge, error) {
	c, err := s.uc.New(ctx, req.GetSiteKey(), req.GetChallengeType())
	if err != nil {
		return nil, err
	}
	return &v1.Challenge{
		ChallengeId:   c.ID,
		ChallengeType: c.Type,
		Image:         c.Media.Data,
		MimeType:      c.Media.MIMEType,
		Width:         int32(c.Media.Width),
		Height:        int32(c.Media.Height),
		ExpiresIn:     int32(c.TTL.Seconds()),
	}, nil
}

// CheckAnswer consumes a challenge and reports whether the answer is correct.
func (s *ChallengeService) CheckAnswer(ctx context.Context, req *v1.CheckAnswerRequest) (*v1.CheckAnswerReply, error) {
	c, ok, err := s.uc.Check(ctx, req.GetChallengeId(), req.GetAnswer())
	if err != nil {
		return nil, err
	}
	return &v1.CheckAnswerReply{Success: ok, ApiKeyId: c.APIKeyID, ChallengeType: c.Type}, nil
}
