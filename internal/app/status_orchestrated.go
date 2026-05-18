package app

import (
	"context"
	"fmt"
	"path/filepath"

	"raioz/internal/detect"
	"raioz/internal/domain/models"
	"raioz/internal/i18n"
	"raioz/internal/naming"
	"raioz/internal/output"
	"raioz/internal/state"
)

// showOrchestratedStatus displays status for projects using the new raioz.yaml format.
// Shows runtime type and proxy URLs alongside standard Docker status info.
func (uc *StatusUseCase) showOrchestratedStatus(ctx context.Context, opts StatusOptions) error {
	cfgDeps, _, err := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	if err != nil {
		return err
	}

	projectDir, _ := filepath.Abs(filepath.Dir(opts.ConfigPath))
	localState, _ := state.LoadLocalState(projectDir)

	fmt.Printf("\n  %s", cfgDeps.Project.Name)
	if cfgDeps.Workspace != "" {
		fmt.Printf(" (%s)", cfgDeps.Workspace)
	}
	fmt.Println()
	fmt.Println()

	// Services
	if len(cfgDeps.Services) > 0 {
		output.PrintSectionHeader(i18n.T("output.section_services"))
		for name, svc := range cfgDeps.Services {
			detection := detect.Detect(svc.Source.Path)
			statusStr := uc.queryServiceStatus(ctx, name, cfgDeps)

			line := fmt.Sprintf("  %-20s %-12s %-10s", name, detection.Runtime, statusStr)

			// Show proxy URL if available
			if cfgDeps.Proxy && uc.deps.ProxyManager != nil {
				url := uc.deps.ProxyManager.GetURL(name)
				if url != "" {
					line += "  " + url
				}
			}

			fmt.Println(line)
		}
		fmt.Println()
	}

	// Dependencies
	if len(cfgDeps.Infra) > 0 {
		output.PrintSectionHeader(i18n.T("output.section_dependencies"))
		for name, entry := range cfgDeps.Infra {
			imageRef := infraImageRef(entry)
			statusStr := uc.queryDepStatus(ctx, name, entry, cfgDeps)

			label := imageRef
			// Show dev override if active
			if localState != nil && localState.IsDevOverridden(name) {
				override, _ := localState.GetDevOverride(name)
				label = fmt.Sprintf("LOCAL: %s (was: %s)", override.LocalPath, imageRef)
			}

			line := fmt.Sprintf("  %-20s %-12s %-10s  %s", name, "image", statusStr, label)
			fmt.Println(line)
		}
		fmt.Println()
	}

	// Proxy status
	if cfgDeps.Proxy && uc.deps.ProxyManager != nil {
		running, _ := uc.deps.ProxyManager.Status(ctx)
		if running {
			output.PrintInfo(i18n.T("output.proxy_running_caddy"))
		} else {
			output.PrintWarning(i18n.T("output.proxy_state_stopped"))
		}
	}

	return nil
}

// queryServiceStatus checks the state of a service container by name.
// Services are always per-project, so naming.Container is correct here. For
// dependencies (which may be workspace-shared or have a name override) use
// queryDepStatus instead.
func (uc *StatusUseCase) queryServiceStatus(ctx context.Context, name string, deps *models.Deps) string {
	return uc.queryStatusByContainer(ctx, naming.Container(deps.Project.Name, name))
}

// queryDepStatus resolves the actual container name for a dependency,
// honoring workspace-shared naming and user-supplied `name:` overrides.
//
// When the canonical name lookup misses (typical for deps brought up via
// `compose:` where the user's compose file dictates a custom container_name),
// we fall back to a label search by com.raioz.project + com.raioz.service —
// the labels are stamped on every raioz-managed container, so this finds the
// actual container regardless of its name. See.
func (uc *StatusUseCase) queryDepStatus(
	ctx context.Context, name string, entry models.InfraEntry, deps *models.Deps,
) string {
	var nameOverride string
	if entry.Inline != nil {
		nameOverride = entry.Inline.Name
	}
	canonical := naming.DepContainer(deps.Project.Name, name, nameOverride)
	status := uc.queryStatusByContainer(ctx, canonical)
	if status != "stopped" {
		return status
	}

	// Fallback: locate the container by its raioz labels.
	if actual := uc.deps.DockerRunner.FindManagedContainerByService(
		ctx, deps.Project.Name, name,
	); actual != "" && actual != canonical {
		return uc.queryStatusByContainer(ctx, actual)
	}
	return status
}

func (uc *StatusUseCase) queryStatusByContainer(ctx context.Context, containerName string) string {
	status, err := uc.deps.DockerRunner.GetContainerStatusByName(ctx, containerName)
	if err != nil {
		return "unknown"
	}
	if status == "" {
		return "stopped"
	}
	return status
}

// infraImageRef returns the image:tag reference for an infra entry.
func infraImageRef(entry models.InfraEntry) string {
	if entry.Inline != nil {
		ref := entry.Inline.Image
		if entry.Inline.Tag != "" {
			ref += ":" + entry.Inline.Tag
		}
		return ref
	}
	return ""
}
