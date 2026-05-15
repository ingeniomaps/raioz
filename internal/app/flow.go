package app

import (
	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
)

// Flow names the high-level pipeline that an inspection command should
// take based on the format the project was loaded from. The split
// exists because the yaml and json paths diverged historically; the
// dual-flow is on the removal ramp documented in ADR-038, and this
// type centralizes the discriminator so each command does not re-
// encode the rule.
type Flow int

const (
	// FlowLegacy targets the .raioz.json pipeline. Removed in v0.8
	// per ADR-038; until then every command falls back to this when
	// the loader did not see a yaml file.
	FlowLegacy Flow = iota
	// FlowYAML targets the raioz.yaml pipeline (also produced by
	// zero-config auto-detect). New commands should branch on this.
	FlowYAML
)

// SelectFlow loads the project description once and returns the flow
// the command should follow. Centralizes the dual-flow discriminator
// so commands stop re-implementing `deps.SchemaVersion == "2.0"`
// checks inline (issue 069). Reads `Deps.SourceFormat` (ADR-039),
// not the legacy magic literal.
//
// Returns the loaded deps and warnings even on error paths so the
// caller can still print the warnings before returning the error.
func SelectFlow(
	loader interfaces.ConfigLoader, configPath string,
) (Flow, *models.Deps, []string, error) {
	deps, warnings, err := loader.LoadDeps(configPath)
	if err != nil {
		return FlowLegacy, nil, warnings, err
	}
	if deps != nil && deps.SourceFormat == config.SourceFormatYAML {
		return FlowYAML, deps, warnings, nil
	}
	return FlowLegacy, deps, warnings, nil
}
