package interfaces

import "raioz/internal/domain/models"

// PortValidator covers port-conflict checks: enumerate the host ports a
// project would bind and report conflicts with other projects or
// external processes.
//
// ADR-012: one of six segregated interfaces composed by DockerRunner.
type PortValidator interface {
	// ValidatePorts inspects the project's declared ports against
	// already-bound ports on this host and returns any conflicts with
	// proposed alternatives. Returning an empty slice + nil error
	// means every port is free.
	ValidatePorts(
		deps *models.Deps, baseDir, projectName string,
	) ([]PortConflict, error)

	// GetAllActivePorts returns the host ports currently bound by any
	// raioz-managed project under baseDir. Used by the port allocator
	// to seed its "in-use" set without re-running every project's
	// ValidatePorts.
	GetAllActivePorts(baseDir string) ([]PortInfo, error)
}
