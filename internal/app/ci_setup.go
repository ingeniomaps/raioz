package app

import (
	"context"
	"fmt"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
)

// validateCIPorts validates ports for CI
func (uc *CIUseCase) validateCIPorts(
	deps *config.Deps,
	ws *interfaces.Workspace,
	workspaceName string,
	result *CIResult,
) error {
	baseDir := uc.deps.Workspace.GetBaseDirFromWorkspace(ws)
	conflicts, err := uc.deps.DockerRunner.ValidatePorts(deps, baseDir, workspaceName)
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
func (uc *CIUseCase) validateCIImages(deps *config.Deps, opts CIOptions, result *CIResult) error {
	if !opts.SkipPull {
		if err := uc.deps.DockerRunner.ValidateAllImages(deps); err != nil {
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
			Check:   "images",
			Status:  "skipped",
			Message: "Image pull skipped (--skip-pull)",
		})
	}
	return nil
}

// ensureCINetwork ensures network for CI
func (uc *CIUseCase) ensureCINetwork(deps *config.Deps, result *CIResult) error {
	ctx := context.Background()
	networkName := deps.Network.GetName()
	subnet := deps.Network.GetSubnet()
	if err := uc.deps.DockerRunner.EnsureNetworkWithConfigAndContext(ctx, networkName, subnet, false); err != nil {
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
func (uc *CIUseCase) ensureCIVolumes(deps *config.Deps, result *CIResult) error {
	var allVolumes []string
	for _, svc := range deps.Services {
		if svc.Docker != nil {
			allVolumes = append(allVolumes, svc.Docker.Volumes...)
		}
	}
	for _, entry := range deps.Infra {
		if entry.Inline != nil {
			allVolumes = append(allVolumes, entry.Inline.Volumes...)
		}
	}

	namedVolumes, err := uc.deps.DockerRunner.ExtractNamedVolumes(allVolumes)
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
		if err := uc.deps.DockerRunner.EnsureVolumeWithContext(ctx, volName); err != nil {
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
			Check:   "volumes",
			Status:  "passed",
			Message: fmt.Sprintf("Ensured %d named volumes", len(namedVolumes)),
		})
	}
	return nil
}

// resolveCIGit resolves git repositories for CI
func (uc *CIUseCase) resolveCIGit(
	deps *config.Deps,
	ws *interfaces.Workspace,
	opts CIOptions,
	result *CIResult,
) error {
	if !opts.SkipBuild {
		for name, svc := range deps.Services {
			if svc.Source.Kind == "git" {
				if err := uc.deps.GitRepository.EnsureRepoWithForce(svc.Source, ws.ServicesDir, opts.ForceReclone); err != nil {
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
			Check:   "git",
			Status:  "skipped",
			Message: "Git operations skipped (--skip-build)",
		})
	}
	return nil
}
