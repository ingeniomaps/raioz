package upcase

import (
	"context"

	"raioz/internal/config"
	"raioz/internal/domain/interfaces"
	"raioz/internal/env"
	"raioz/internal/logging"
	workspacepkg "raioz/internal/workspace"
)

// generateEnvFilesFromTemplates generates .env files from templates for all services
func (uc *UseCase) generateEnvFilesFromTemplates(ctx context.Context, deps *config.Deps, ws *interfaces.Workspace) error {
	// Convert interfaces.Workspace to concrete workspace.Workspace
	wsConcrete := (*workspacepkg.Workspace)(ws)

	// Process all services
	for name, svc := range deps.Services {
		// Skip disabled services
		if svc.Enabled != nil && !*svc.Enabled {
			continue
		}

		// Only process git services (they have a cloned directory)
		if svc.Source.Kind != "git" {
			continue
		}

		// Get service path
		servicePath := workspacepkg.GetServicePath(wsConcrete, name, svc)

		// Generate .env from template
		if err := env.GenerateEnvFromTemplate(wsConcrete, deps, name, servicePath, svc); err != nil {
			logging.WarnWithContext(ctx, "Failed to generate .env from template", "service", name, "error", err.Error())
			// Continue with other services
		} else {
			logging.InfoWithContext(ctx, "Generated .env from template", "service", name, "path", servicePath)
		}
	}

	return nil
}
