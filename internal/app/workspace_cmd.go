package app

import (
	"fmt"

	"raioz/internal/audit"
	"raioz/internal/i18n"
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
		output.PrintInfo(i18n.T("output.workspace_created_name", workspaceName))
		_ = ws
	} else {
		output.PrintInfo(i18n.T("output.workspace_already_exists", workspaceName))
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
			output.PrintInfo(i18n.T("output.workspace_loaded_root", workspaceName))
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
			output.PrintWarning(i18n.T("output.failed_log_audit", err))
		}
	}

	output.PrintSuccess(i18n.T("output.workspace_active_set", workspaceName))
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
		fmt.Println(i18n.T("output.workspace_no_workspaces"))
		fmt.Println(i18n.T("output.workspace_create_hint"))
		return nil
	}

	// Print workspaces
	fmt.Println(i18n.T("output.workspace_available"))
	for _, ws := range workspaces {
		marker := " "
		if ws == activeWorkspace {
			marker = "*"
		}
		fmt.Printf("  %s %s", marker, ws)
		if ws == activeWorkspace {
			fmt.Printf(" %s", i18n.T("output.workspace_active_label"))
		}
		fmt.Println()
	}

	if activeWorkspace == "" {
		fmt.Println()
		fmt.Println(i18n.T("output.workspace_no_active"))
		fmt.Println(i18n.T("output.workspace_set_hint"))
	}

	return nil
}
