package app

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/mocks"
)

func TestPrintConflictingPortsTable(t *testing.T) {
	tests := []struct {
		name      string
		conflicts []docker.PortConflict
		wantRows  []string
		wantOmit  []string
	}{
		{
			name: "single conflict with alternative",
			conflicts: []docker.PortConflict{
				{Port: "9001", Project: "hypixo", Service: "keycloak", Alternative: "9011"},
			},
			wantRows: []string{"9001", "hypixo", "keycloak", "9011"},
		},
		{
			name: "missing alternative renders dash",
			conflicts: []docker.PortConflict{
				{Port: "5432", Project: "alpha", Service: "postgres"},
			},
			wantRows: []string{"5432", "alpha", "postgres", "-"},
		},
		{
			name: "multiple conflicts preserve input order",
			conflicts: []docker.PortConflict{
				{Port: "9001", Project: "hypixo", Service: "keycloak", Alternative: "9011"},
				{Port: "5540", Project: "hypixo", Service: "redisinsight"},
				{Port: "8025", Project: "gouduet", Service: "mailhog", Alternative: "8125"},
			},
			wantRows: []string{
				"9001", "hypixo", "keycloak", "9011",
				"5540", "redisinsight", "-",
				"8025", "gouduet", "mailhog", "8125",
			},
		},
		{
			name:      "empty input still prints header",
			conflicts: nil,
			wantRows:  []string{"PORT", "PROJECT", "SERVICE", "ALTERNATIVE"},
			wantOmit:  []string{"hypixo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printConflictingPortsTable(&buf, tt.conflicts)
			got := buf.String()
			for _, want := range tt.wantRows {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q\n--- got ---\n%s", want, got)
				}
			}
			for _, omit := range tt.wantOmit {
				if strings.Contains(got, omit) {
					t.Errorf("output unexpectedly contains %q\n--- got ---\n%s", omit, got)
				}
			}
		})
	}
}

// When ConfigLoader can't resolve a raioz.yaml (e.g. user invoked the flag
// from a directory without one) we should warn instead of crashing.
func TestListConflictingPorts_NoConfig(t *testing.T) {
	deps := &Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(string) (*config.Deps, []string, error) {
				return nil, nil, nil
			},
		},
		Workspace:    &mocks.MockWorkspaceManager{},
		StateManager: &mocks.MockStateManager{},
		DockerRunner: &mocks.MockDockerRunner{},
		Validator:    &mocks.MockValidator{},
	}

	uc := NewPortsUseCase(deps)
	if err := uc.Execute(context.Background(), PortsOptions{Conflicting: true}); err != nil {
		t.Fatalf("expected nil error when config is missing, got %v", err)
	}
}
