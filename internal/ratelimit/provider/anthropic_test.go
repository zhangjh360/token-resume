package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"tokenresume/internal/config"
)

func TestAnthropicProviderCheck429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"remaining_tokens":0,"is_limited":true,"reset_at":"2026-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	p := &AnthropicProvider{
		cfg: config.RateLimitConfig{
			Provider:      "anthropic",
			ProxyEndpoint: server.URL,
		},
		client: server.Client(),
	}

	resp, err := p.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsLimited {
		t.Fatalf("expected limited=true")
	}
}

func TestAnthropicProviderDirectModeWithAuthToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/anthropic/v1/rate_limit" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("unexpected authorization header: %s", got)
		}
		w.Header().Set("x-ratelimit-remaining-tokens", "42")
		w.Header().Set("x-ratelimit-limit-tokens", "100")
		w.Header().Set("x-ratelimit-reset-timestamp", "1893456000")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := &AnthropicProvider{
		cfg: config.RateLimitConfig{
			Provider:     "anthropic",
			BaseURL:      server.URL + "/api/anthropic",
			AuthToken:    "test-token",
			EndpointPath: "/v1/rate_limit",
		},
		client: server.Client(),
	}

	resp, err := p.Check(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.RemainingTokens != 42 {
		t.Fatalf("unexpected remaining tokens: %d", resp.RemainingTokens)
	}
	if resp.IsLimited {
		t.Fatalf("expected limited=false")
	}
}
