package app

import (
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/i18n"
	"raioz/internal/link"
	"raioz/internal/output"
)

// LinkUseCase handles link operations for services
type LinkUseCase struct {
	deps *Dependencies
}

// NewLinkUseCase creates a new LinkUseCase
func NewLinkUseCase(deps *Dependencies) *LinkUseCase {
	return &LinkUseCase{deps: deps}
}

// Add creates a symlink from workspace to external path
func (uc *LinkUseCase) Add(serviceName string, externalPath string, configPath string) error {
	// Resolve external path to absolute
	absExternalPath, err := filepath.Abs(externalPath)
	if err != nil {
		return fmt.Errorf("failed to resolve external path: %w", err)
	}

	// Load config to get project name
	deps, _, err := uc.deps.ConfigLoader.LoadDeps(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Resolve workspace
	ws, err := uc.deps.Workspace.Resolve(deps.Project.Name)
	if err != nil {
		return fmt.Errorf("failed to resolve workspace: %w", err)
	}

	// Check if service exists in config
	svc, exists := deps.Services[serviceName]
	if !exists {
		return fmt.Errorf("service '%s' not found in configuration", serviceName)
	}

	// Get service path in workspace
	servicePath := uc.deps.Workspace.GetServicePath(ws, serviceName, svc)

	// Check if service path already exists as a directory (not symlink)
	if info, err := os.Stat(servicePath); err == nil {
		if info.IsDir() {
			// Check if it's a symlink
			isLinked, target, err := link.IsLinked(servicePath)
			if err != nil {
				return fmt.Errorf("failed to check if service is linked: %w", err)
			}
			if !isLinked {
				return fmt.Errorf(
					"service path already exists as a directory: %s\n"+
						"To create a symlink, you must first remove or move the existing directory",
					servicePath,
				)
			}
			// Already linked, check if it points to the same target
			absTarget, err := filepath.Abs(target)
			if err != nil {
				return fmt.Errorf("failed to resolve existing target: %w", err)
			}
			if absTarget == absExternalPath {
				output.PrintInfo(i18n.T("output.link_already_linked", serviceName, absExternalPath))
				return nil
			}
			return fmt.Errorf(
				"service '%s' is already linked to: %s\n"+
					"Use 'raioz link remove %s' first to unlink it",
				serviceName, target, serviceName,
			)
		}
	}

	// Create symlink
	if err := link.CreateLink(servicePath, absExternalPath); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	output.PrintSuccess(i18n.T("output.link_created", serviceName, absExternalPath))
	output.PrintInfo(i18n.T("output.link_service_path", servicePath))

	return nil
}

// Remove removes a service symlink
func (uc *LinkUseCase) Remove(serviceName string, configPath string) error {
	// Load config to get project name
	deps, _, err := uc.deps.ConfigLoader.LoadDeps(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Resolve workspace
	ws, err := uc.deps.Workspace.Resolve(deps.Project.Name)
	if err != nil {
		return fmt.Errorf("failed to resolve workspace: %w", err)
	}

	// Check if service exists in config
	svc, exists := deps.Services[serviceName]
	if !exists {
		return fmt.Errorf("service '%s' not found in configuration", serviceName)
	}

	// Get service path in workspace
	servicePath := uc.deps.Workspace.GetServicePath(ws, serviceName, svc)

	// Check if service is linked
	isLinked, target, err := link.IsLinked(servicePath)
	if err != nil {
		return fmt.Errorf("failed to check if service is linked: %w", err)
	}

	if !isLinked {
		output.PrintInfo(i18n.T("output.link_not_linked", serviceName))
		return nil
	}

	// Remove symlink
	if err := link.RemoveLink(servicePath); err != nil {
		return fmt.Errorf("failed to remove symlink: %w", err)
	}

	output.PrintSuccess(i18n.T("output.link_removed", serviceName, target))
	output.PrintInfo(i18n.T("output.link_external_not_deleted"))

	return nil
}

// List lists all linked services
func (uc *LinkUseCase) List(configPath string) error {
	// Load config to get project name
	deps, _, err := uc.deps.ConfigLoader.LoadDeps(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Resolve workspace
	ws, err := uc.deps.Workspace.Resolve(deps.Project.Name)
	if err != nil {
		return fmt.Errorf("failed to resolve workspace: %w", err)
	}

	var linkedServices []struct {
		name   string
		target string
	}

	// Check each service
	for name, svc := range deps.Services {
		servicePath := uc.deps.Workspace.GetServicePath(ws, name, svc)
		isLinked, target, err := link.IsLinked(servicePath)
		if err != nil {
			// Skip on error (service might not exist yet)
			continue
		}
		if isLinked {
			linkedServices = append(linkedServices, struct {
				name   string
				target string
			}{name, target})
		}
	}

	if len(linkedServices) == 0 {
		fmt.Println(i18n.T("output.link_empty_list"))
		return nil
	}

	fmt.Println(i18n.T("output.link_list_header"))
	for _, linked := range linkedServices {
		fmt.Printf("  %s -> %s\n", linked.name, linked.target)
	}

	return nil
}
