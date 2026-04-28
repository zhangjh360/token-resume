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
