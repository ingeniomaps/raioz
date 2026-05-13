// Package snapshotcase wraps the snapshot port (issue 034 / ADR-014)
// in a use-case-shaped struct so the CLI follows the same wiring as
// every other command (cli → app → port).
//
// Tests in this package stub the SnapshotManager directly; no Docker
// is required to exercise the lifecycle paths.
package snapshotcase

import (
	"context"
	"fmt"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
)

// Dependencies is the small subset of app.Dependencies a snapshot use
// case needs. Kept narrow so tests don't have to wire the whole
// dependency graph.
type Dependencies struct {
	ConfigLoader    interfaces.ConfigLoader
	SnapshotManager interfaces.SnapshotManager
}

// CreateOptions controls Create. Name is the snapshot tag; the volume
// list is resolved from the project's raioz.yaml.
type CreateOptions struct {
	ConfigPath string
	Name       string
}

// CreateResult mirrors the metadata returned by the manager. The CLI
// formats it; the use case never prints.
type CreateResult struct {
	Snapshot *interfaces.Snapshot
	// NoVolumes is true when the project declared no infra volumes.
	// Callers should report "nothing to snapshot" rather than treating
	// it as an error.
	NoVolumes bool
}

// CreateUseCase saves every infra volume declared in raioz.yaml.
type CreateUseCase struct {
	Deps *Dependencies
}

func (uc *CreateUseCase) Execute(ctx context.Context, opts CreateOptions) (*CreateResult, error) {
	deps, _, err := uc.Deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	volumes := collectInfraVolumes(deps)
	if len(volumes) == 0 {
		return &CreateResult{NoVolumes: true}, nil
	}
	snap, err := uc.Deps.SnapshotManager.Create(ctx, deps.Project.Name, opts.Name, volumes)
	if err != nil {
		return nil, err
	}
	return &CreateResult{Snapshot: snap}, nil
}

// RestoreOptions controls Restore.
type RestoreOptions struct {
	ConfigPath string
	Name       string
}

// RestoreUseCase replays a snapshot over the project's named volumes.
type RestoreUseCase struct {
	Deps *Dependencies
}

func (uc *RestoreUseCase) Execute(ctx context.Context, opts RestoreOptions) error {
	deps, _, err := uc.Deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	return uc.Deps.SnapshotManager.Restore(ctx, deps.Project.Name, opts.Name)
}

// ListOptions controls List.
type ListOptions struct {
	ConfigPath string
}

// ListUseCase returns every snapshot the manager knows about for the
// current project.
type ListUseCase struct {
	Deps *Dependencies
}

func (uc *ListUseCase) Execute(ctx context.Context, opts ListOptions) ([]interfaces.Snapshot, error) {
	deps, _, err := uc.Deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return uc.Deps.SnapshotManager.List(ctx, deps.Project.Name)
}

// DeleteOptions controls Delete.
type DeleteOptions struct {
	ConfigPath string
	Name       string
}

// DeleteUseCase removes a snapshot from disk.
type DeleteUseCase struct {
	Deps *Dependencies
}

func (uc *DeleteUseCase) Execute(ctx context.Context, opts DeleteOptions) error {
	deps, _, err := uc.Deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	return uc.Deps.SnapshotManager.Delete(ctx, deps.Project.Name, opts.Name)
}

// collectInfraVolumes walks `deps.Infra` and returns a (volume → service)
// map, the same shape the manager expects. Volumes declared on
// services (rather than infra) are excluded — they're typically bind
// mounts that don't need archiving.
func collectInfraVolumes(deps *models.Deps) map[string]string {
	if deps == nil {
		return nil
	}
	out := make(map[string]string)
	for svcName, entry := range deps.Infra {
		if entry.Inline == nil {
			continue
		}
		for _, vol := range entry.Inline.Volumes {
			out[vol] = svcName
		}
	}
	return out
}
