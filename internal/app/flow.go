package app

// TODO(ADR-038): inline this when the legacy JSON loader is removed in
// v1.0. The Flow enum + SelectFlow helper exist purely to differentiate
// the two loader implementations during the deprecation ramp. Tracked
// in scripts/dual-flow-baseline.txt and docs/RATCHETS.md.

import (
	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
)

// Flow discriminates yaml vs legacy-json pipelines for inspection
// commands. The legacy branch goes away with ADR-038.
type Flow int

const (
	FlowLegacy Flow = iota
	FlowYAML
)

// SelectFlow is the single source of truth for the yaml/legacy
// branch. Reads Deps.SourceFormat (ADR-039), not the legacy "2.0"
// literal. Warnings are returned even on error so the caller can
// still surface them.
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
