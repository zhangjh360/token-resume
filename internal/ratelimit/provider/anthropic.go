package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"tokenresume/internal/config"
)

type AnthropicProvider struct {
	cfg    config.RateLimitConfig
	client *http.Client
}

type anthropicPayload struct {
	RemainingTokens int    `json:"remaining_tokens"`
	ResetAt         string `json:"reset_at"`
	IsLimited       bool   `json:"is_limited"`
	LimitPer5H      int    `json:"limit_per_5h"`
}

func New(cfg config.RateLimitConfig) Provider {
	switch strings.ToLower(cfg.Provider) {
	case "anthropic":
		return &AnthropicProvider{
			cfg: cfg,
			client: &http.Client{
				Timeout: 10 * time.Second,
			},
		}
	default:
		return &NoopProvider{}
	}
}

func (p *AnthropicProvider) SupportsStreaming() bool {
	return true
}

func (p *AnthropicProvider) Check(ctx context.Context) (*Response, error) {
	if p.cfg.ProxyEndpoint == "" {
		return &Response{
			RemainingTokens: p.cfg.Fallback.LimitPer5H,
			ResetAt:         time.Now().Add(time.Duration(p.cfg.Fallback.ResetWindowMinutes) * time.Minute),
			IsLimited:       false,
			LimitPer5H:      p.cfg.Fallback.LimitPer5H,
		}, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.cfg.ProxyEndpoint, nil)
	if err != nil {
		return nil, err
	}
	if p.cfg.APIKey != "" {
		req.Header.Set("x-api-key", p.cfg.APIKey)
	}
	for k, v := range p.cfg.Headers {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		if resp.StatusCode == http.StatusTooManyRequests {
			return readRateLimitResponse(resp, true)
		}
		return nil, fmt.Errorf("rate limit endpoint status: %d", resp.StatusCode)
	}

	return readRateLimitResponse(resp, false)
}

func readRateLimitResponse(resp *http.Response, forceLimited bool) (*Response, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var payload anthropicPayload
	if err := json.Unmarshal(body, &payload); err == nil {
		resetAt, _ := time.Parse(time.RFC3339, payload.ResetAt)
		isLimited := payload.IsLimited || forceLimited || payload.RemainingTokens <= 0
		return &Response{
			RemainingTokens: payload.RemainingTokens,
			ResetAt:         resetAt,
			IsLimited:       isLimited,
			LimitPer5H:      payload.LimitPer5H,
		}, nil
	}

	remaining := parseIntHeader(resp.Header.Get("x-ratelimit-remaining-tokens"))
	limit := parseIntHeader(resp.Header.Get("x-ratelimit-limit-tokens"))
	resetAt := parseTimeHeader(resp.Header.Get("x-ratelimit-reset-timestamp"))
	isLimited := forceLimited || remaining <= 0

	return &Response{
		RemainingTokens: remaining,
		ResetAt:         resetAt,
		IsLimited:       isLimited,
		LimitPer5H:      limit,
	}, nil
}

func parseIntHeader(v string) int {
	i, _ := strconv.Atoi(strings.TrimSpace(v))
	return i
}

func parseTimeHeader(v string) time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}
	}
	if unix, err := strconv.ParseInt(v, 10, 64); err == nil {
		return time.Unix(unix, 0)
	}
	t, _ := time.Parse(time.RFC3339, v)
	return t
}
