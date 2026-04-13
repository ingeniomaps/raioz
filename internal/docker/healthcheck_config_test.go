package docker

import (
	"testing"

	"raioz/internal/config"
)

func TestHealthcheckToMap(t *testing.T) {
	tests := []struct {
		name     string
		hc       *config.HealthcheckConfig
		wantNil  bool
		wantKeys []string
		wantVals map[string]any
	}{
		{
			name:    "nil config",
			hc:      nil,
			wantNil: true,
		},
		{
			name: "disabled",
			hc: &config.HealthcheckConfig{
				Disable: true,
			},
			wantKeys: []string{"disable"},
			wantVals: map[string]any{"disable": true},
		},
		{
			name: "disabled overrides other fields",
			hc: &config.HealthcheckConfig{
				Disable:  true,
				Interval: "30s",
				Test:     []string{"CMD", "true"},
			},
			wantKeys: []string{"disable"},
		},
		{
			name: "full config",
			hc: &config.HealthcheckConfig{
				Test:          []string{"CMD", "curl", "-f", "http://localhost/"},
				Interval:      "30s",
				Timeout:       "10s",
				Retries:       5,
				StartPeriod:   "40s",
				StartInterval: "5s",
			},
			wantKeys: []string{
				"test", "interval", "timeout", "retries", "start_period", "start_interval",
			},
		},
		{
			name: "partial config",
			hc: &config.HealthcheckConfig{
				Test:     []string{"CMD-SHELL", "echo ok"},
				Interval: "10s",
			},
			wantKeys: []string{"test", "interval"},
		},
		{
			name:    "empty config",
			hc:      &config.HealthcheckConfig{},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HealthcheckToMap(tt.hc)
			if tt.wantNil {
				if got != nil {
					t.Errorf("HealthcheckToMap() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("HealthcheckToMap() = nil, want non-nil")
			}
			for _, k := range tt.wantKeys {
				if _, ok := got[k]; !ok {
					t.Errorf("HealthcheckToMap() missing key %q", k)
				}
			}
			for k, v := range tt.wantVals {
				if got[k] != v {
					t.Errorf("HealthcheckToMap()[%q] = %v, want %v", k, got[k], v)
				}
			}
			// Ensure no extra unexpected keys when wantKeys provided.
			if len(tt.wantKeys) > 0 && len(got) != len(tt.wantKeys) {
				t.Errorf("HealthcheckToMap() keys = %d, want %d; got %v",
					len(got), len(tt.wantKeys), got)
			}
		})
	}
}
