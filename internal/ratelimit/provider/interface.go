package provider

import (
	"context"
	"time"
)

type Response struct {
	RemainingTokens int
	ResetAt         time.Time
	IsLimited       bool
	LimitPer5H      int
}

type Provider interface {
	Check(ctx context.Context) (*Response, error)
	SupportsStreaming() bool
}
