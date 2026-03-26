package app

import (
	"fmt"

	"raioz/internal/audit"
	"raioz/internal/output"
	"raioz/internal/root"
)

// WorkspaceUseCase handles workspace operations
type WorkspaceUseCase struct {
	deps *Dependencies
}

// NewWorkspaceUseCase creates a new WorkspaceUseCase
func NewWorkspaceUseCase(deps *Dependencies) *WorkspaceUseCase {
	return &WorkspaceUseCase{deps: deps}
}

// Use sets the active workspace
func (uc *WorkspaceUseCase) Use(workspaceName string) error {
	if workspaceName == "" {
		return fmt.Errorf("workspace name cannot be empty")
	}

	// Check if workspace exists
	exists, err := uc.deps.Workspace.WorkspaceExists(workspaceName)
	if err != nil {
		return fmt.Errorf("failed to check workspace existence: %w", err)
	}

	if !exists {
		// Create workspace by resolving it (this creates the directory structure)
		ws, err := uc.deps.Workspace.Resolve(workspaceName)
		if err != nil {
			return fmt.Errorf("failed to create workspace: %w", err)
		}
		output.PrintInfo(fmt.Sprintf("Created workspace: %s", workspaceName))
		_ = ws
	} else {
		output.PrintInfo(fmt.Sprintf("Workspace %s already exists", workspaceName))
	}

	// Load raioz.root.json if it exists (to validate it)
	ws, err := uc.deps.Workspace.Resolve(workspaceName)
	if err != nil {
		return fmt.Errorf("failed to resolve workspace: %w", err)
	}

	if root.Exists(ws) {
		rootConfig, err := root.Load(ws)
		if err != nil {
			return fmt.Errorf("failed to load raioz.root.json: %w", err)
		}
		if rootConfig != nil {
			output.PrintInfo(fmt.Sprintf("Loaded raioz.root.json for workspace %s", workspaceName))
		}
	}

	// Get old workspace before setting new one
	oldWorkspace, _ := uc.deps.Workspace.GetActiveWorkspace()

	// Set as active workspace
	if err := uc.deps.Workspace.SetActiveWorkspace(workspaceName); err != nil {
		return fmt.Errorf("failed to set active workspace: %w", err)
	}

	// Log audit event if workspace changed
	if oldWorkspace != workspaceName {
		if err := audit.LogWorkspaceChanged(oldWorkspace, workspaceName); err != nil {
			output.PrintWarning(fmt.Sprintf("Failed to log audit event: %v", err))
		}
	}

	output.PrintSuccess(fmt.Sprintf("Active workspace set to: %s", workspaceName))
	return nil
}

// List lists all available workspaces
func (uc *WorkspaceUseCase) List() error {
	// Get active workspace
	activeWorkspace, err := uc.deps.Workspace.GetActiveWorkspace()
	if err != nil {
		return fmt.Errorf("failed to get active workspace: %w", err)
	}

	// List all workspaces
	workspaces, err := uc.deps.Workspace.ListWorkspaces()
	if err != nil {
		return fmt.Errorf("failed to list workspaces: %w", err)
	}

	if len(workspaces) == 0 {
		fmt.Println("No workspaces found.")
		fmt.Println("Create a workspace by running: raioz workspace use <name>")
		return nil
	}

	// Print workspaces
	fmt.Println("Available workspaces:")
	for _, ws := range workspaces {
		marker := " "
		if ws == activeWorkspace {
			marker = "*"
		}
		fmt.Printf("  %s %s", marker, ws)
		if ws == activeWorkspace {
			fmt.Print(" (active)")
		}
		fmt.Println()
	}

	if activeWorkspace == "" {
		fmt.Println("\nNo active workspace set.")
		fmt.Println("Set an active workspace by running: raioz workspace use <name>")
	}

	return nil
}
