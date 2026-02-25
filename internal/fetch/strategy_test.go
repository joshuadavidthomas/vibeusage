package fetch

import (
	"context"
	"testing"
)

type OAuthStrategy struct{}

func (s *OAuthStrategy) IsAvailable() bool                              { return false }
func (s *OAuthStrategy) Fetch(ctx context.Context) (FetchResult, error) { return FetchResult{}, nil }

type APIKeyStrategy struct{}

func (s *APIKeyStrategy) IsAvailable() bool                              { return false }
func (s *APIKeyStrategy) Fetch(ctx context.Context) (FetchResult, error) { return FetchResult{}, nil }

type DeviceFlowStrategy struct{}

func (s *DeviceFlowStrategy) IsAvailable() bool { return false }
func (s *DeviceFlowStrategy) Fetch(ctx context.Context) (FetchResult, error) {
	return FetchResult{}, nil
}

type WebStrategy struct{}

func (s *WebStrategy) IsAvailable() bool                              { return false }
func (s *WebStrategy) Fetch(ctx context.Context) (FetchResult, error) { return FetchResult{}, nil }

func TestStrategyName(t *testing.T) {
	tests := []struct {
		name     string
		strategy Strategy
		want     string
	}{
		{"OAuthStrategy", &OAuthStrategy{}, "oauth"},
		{"APIKeyStrategy", &APIKeyStrategy{}, "apikey"},
		{"DeviceFlowStrategy", &DeviceFlowStrategy{}, "deviceflow"},
		{"WebStrategy", &WebStrategy{}, "web"},
		{"mockStrategy", &mockStrategy{}, "mock"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StrategyName(tt.strategy)
			if got != tt.want {
				t.Errorf("StrategyName() = %q, want %q", got, tt.want)
			}
		})
	}
}
