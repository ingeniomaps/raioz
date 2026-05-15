package app

import (
	"errors"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/mocks"
)

func TestSelectFlow(t *testing.T) {
	type loadResult struct {
		deps     *models.Deps
		warnings []string
		err      error
	}

	yamlDeps := &models.Deps{SourceFormat: models.SourceFormatYAML}
	jsonDeps := &models.Deps{SourceFormat: models.SourceFormatLegacyJSON}
	unstamped := &models.Deps{} // never happens in production but pin defensive default
	loadErr := errors.New("boom")

	tests := []struct {
		name     string
		load     loadResult
		wantFlow Flow
		wantErr  bool
	}{
		{"yaml deps", loadResult{deps: yamlDeps}, FlowYAML, false},
		{"legacy json deps", loadResult{deps: jsonDeps}, FlowLegacy, false},
		{"unstamped deps default to legacy", loadResult{deps: unstamped}, FlowLegacy, false},
		{"nil deps default to legacy", loadResult{deps: nil}, FlowLegacy, false},
		{
			"loader error propagates with FlowLegacy",
			loadResult{deps: nil, warnings: []string{"w"}, err: loadErr},
			FlowLegacy, true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := &mocks.MockConfigLoader{
				LoadDepsFunc: func(_ string) (*models.Deps, []string, error) {
					return tt.load.deps, tt.load.warnings, tt.load.err
				},
			}
			gotFlow, gotDeps, gotWarnings, gotErr := SelectFlow(loader, "ignored")
			if gotFlow != tt.wantFlow {
				t.Errorf("flow = %v, want %v", gotFlow, tt.wantFlow)
			}
			if (gotErr != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", gotErr, tt.wantErr)
			}
			if !tt.wantErr && gotDeps != tt.load.deps {
				t.Errorf("deps mismatch")
			}
			if len(gotWarnings) != len(tt.load.warnings) {
				t.Errorf("warnings len = %d, want %d", len(gotWarnings), len(tt.load.warnings))
			}
		})
	}
}
