package ratelimit

import (
	"context"
	"time"

	"tokenresume/internal/config"
	"tokenresume/internal/ratelimit/provider"
)

type RateLimitStatus struct {
	RemainingTokens int
	ResetAt         time.Time
	IsLimited       bool
	LimitPer5Hours  int
}

type Detector struct {
	client   provider.Provider
	fallback config.RateLimitFallback
}

func NewDetector(client provider.Provider, fallback config.RateLimitFallback) *Detector {
	return &Detector{client: client, fallback: fallback}
}

func (d *Detector) Check(ctx context.Context) (*RateLimitStatus, error) {
	resp, err := d.client.Check(ctx)
	if err != nil {
		return nil, err
	}
	limit := d.fallback.LimitPer5H
	if resp.LimitPer5H > 0 {
		limit = resp.LimitPer5H
	}
	resetAt := resp.ResetAt
	if resetAt.IsZero() {
		resetAt = time.Now().Add(time.Duration(d.fallback.ResetWindowMinutes) * time.Minute)
	}
	return &RateLimitStatus{
		RemainingTokens: resp.RemainingTokens,
		ResetAt:         resetAt,
		IsLimited:       resp.IsLimited,
		LimitPer5Hours:  limit,
	}, nil
}

func (d *Detector) WaitForReset(ctx context.Context, resetAt time.Time, safetyMarginSeconds int) error {
	target := resetAt.Add(time.Duration(safetyMarginSeconds) * time.Second)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if time.Now().After(target) {
				return nil
			}
			time.Sleep(30 * time.Second)
		}
	}
}
