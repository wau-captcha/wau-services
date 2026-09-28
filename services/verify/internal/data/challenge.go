package data

import (
	"context"

	challengev1 "verify/api/challenge/v1"
	"verify/internal/biz"
)

type challengeClient struct {
	client challengev1.ChallengeServiceClient
}

// NewChallengeClient adapts the challenge gRPC client to biz.ChallengeClient.
func NewChallengeClient(client challengev1.ChallengeServiceClient) biz.ChallengeClient {
	return &challengeClient{client: client}
}

func (c *challengeClient) CheckAnswer(ctx context.Context, challengeID, answer string) (*biz.Solution, error) {
	reply, err := c.client.CheckAnswer(ctx, &challengev1.CheckAnswerRequest{ChallengeId: challengeID, Answer: answer})
	if err != nil {
		return nil, err
	}
	return &biz.Solution{
		Success:       reply.GetSuccess(),
		APIKeyID:      reply.GetApiKeyId(),
		ChallengeType: reply.GetChallengeType(),
		UserID:        reply.GetUserId(),
	}, nil
}
