package provider

import (
	"context"
	"time"
)

type NoopProvider struct{}

func (p *NoopProvider) Check(ctx context.Context) (*Response, error) {
	return &Response{
		RemainingTokens: 1,
		ResetAt:         time.Now().Add(5 * time.Minute),
		IsLimited:       false,
		LimitPer5H:      0,
	}, nil
}

func (p *NoopProvider) SupportsStreaming() bool {
	return false
}
