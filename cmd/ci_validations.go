package cmd

import (
	"context"
	"fmt"

	"raioz/internal/config"
	"raioz/internal/docker"
	"raioz/internal/git"
	"raioz/internal/workspace"
)

// validateCIPorts validates ports for CI
func validateCIPorts(
	deps *config.Deps,
	ws *workspace.Workspace,
	workspaceName string,
	result *CIResult,
) error {
	baseDir := workspace.GetBaseDirFromWorkspace(ws)
	conflicts, err := docker.ValidatePorts(deps, baseDir, workspaceName)
	if err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "ports",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(result.Errors, fmt.Sprintf("Port validation failed: %v", err))
		return err
	}

	if len(conflicts) > 0 {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "ports",
			Status:  "failed",
			Message: fmt.Sprintf("Port conflicts detected: %d", len(conflicts)),
		})
		for _, conflict := range conflicts {
			result.Errors = append(
				result.Errors,
				fmt.Sprintf(
					"Port %s in use by %s/%s",
					conflict.Port, conflict.Project, conflict.Service,
				),
			)
		}
		return fmt.Errorf("port conflicts detected")
	}

	result.Validations = append(result.Validations, ValidationResult{
		Check:  "ports",
		Status: "passed",
	})
	return nil
}

// validateCIImages validates images for CI
func validateCIImages(deps *config.Deps, result *CIResult) error {
	if !ciSkipPull {
		if err := docker.ValidateAllImages(deps); err != nil {
			result.Validations = append(result.Validations, ValidationResult{
				Check:   "images",
				Status:  "failed",
				Message: err.Error(),
			})
			result.Errors = append(result.Errors, fmt.Sprintf("Image validation failed: %v", err))
			return err
		}

		result.Validations = append(result.Validations, ValidationResult{
			Check:  "images",
			Status: "passed",
		})
	} else {
		result.Validations = append(result.Validations, ValidationResult{
			Check:  "images",
			Status: "skipped",
			Message: "Image pull skipped (--skip-pull)",
		})
	}
	return nil
}

// ensureCINetwork ensures network for CI
func ensureCINetwork(deps *config.Deps, result *CIResult) error {
	ctx := context.Background()
	networkConfig := docker.NetworkConfig{
		Name:   deps.Network.GetName(),
		Subnet: deps.Network.GetSubnet(),
	}
	if err := docker.EnsureNetworkWithConfigAndContext(ctx, networkConfig, false); err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "network",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(result.Errors, fmt.Sprintf("Network setup failed: %v", err))
		return err
	}

	result.Validations = append(result.Validations, ValidationResult{
		Check:  "network",
		Status: "passed",
	})
	return nil
}

// ensureCIVolumes ensures volumes for CI
func ensureCIVolumes(deps *config.Deps, result *CIResult) error {
	var allVolumes []string
	for _, svc := range deps.Services {
		allVolumes = append(allVolumes, svc.Docker.Volumes...)
	}
	for _, infra := range deps.Infra {
		allVolumes = append(allVolumes, infra.Volumes...)
	}

	namedVolumes, err := docker.ExtractNamedVolumes(allVolumes)
	if err != nil {
		result.Validations = append(result.Validations, ValidationResult{
			Check:   "volumes",
			Status:  "failed",
			Message: err.Error(),
		})
		result.Errors = append(result.Errors, fmt.Sprintf("Volume extraction failed: %v", err))
		return err
	}

	ctx := context.Background()
	for _, volName := range namedVolumes {
		if err := docker.EnsureVolumeWithContext(ctx, volName); err != nil {
			result.Validations = append(result.Validations, ValidationResult{
				Check:   "volumes",
				Status:  "failed",
				Message: fmt.Sprintf("Volume %s: %v", volName, err),
			})
			result.Errors = append(result.Errors, fmt.Sprintf("Volume setup failed for %s: %v", volName, err))
			return err
		}
	}

	if len(namedVolumes) > 0 {
		result.Validations = append(result.Validations, ValidationResult{
			Check:  "volumes",
			Status: "passed",
			Message: fmt.Sprintf("Ensured %d named volumes", len(namedVolumes)),
		})
	}
	return nil
}

// resolveCIGit resolves git repositories for CI
func resolveCIGit(deps *config.Deps, ws *workspace.Workspace, result *CIResult) error {
	if !ciSkipBuild {
		for name, svc := range deps.Services {
			if svc.Source.Kind == "git" {
				if err := git.EnsureRepoWithForce(svc.Source, ws.ServicesDir, ciForceReclone); err != nil {
					result.Validations = append(result.Validations, ValidationResult{
						Check:   "git",
						Status:  "failed",
						Message: fmt.Sprintf("Service %s: %v", name, err),
					})
					result.Errors = append(result.Errors, fmt.Sprintf("Git setup failed for %s: %v", name, err))
					return err
				}
			}
		}

		result.Validations = append(result.Validations, ValidationResult{
			Check:  "git",
			Status: "passed",
		})
	} else {
		result.Validations = append(result.Validations, ValidationResult{
			Check:  "git",
			Status: "skipped",
			Message: "Git operations skipped (--skip-build)",
		})
	}
	return nil
}
