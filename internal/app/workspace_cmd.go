package app

import (
	"fmt"
	"io"
	"os"

	"raioz/internal/audit"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/root"
)

// WorkspaceUseCase handles workspace operations
type WorkspaceUseCase struct {
	deps *Dependencies
	Out  io.Writer
}

// NewWorkspaceUseCase creates a new WorkspaceUseCase
func NewWorkspaceUseCase(deps *Dependencies) *WorkspaceUseCase {
	return &WorkspaceUseCase{deps: deps, Out: os.Stdout}
}

// Current shows the current active workspace
func (uc *WorkspaceUseCase) Current() error {
	w := uc.Out
	active, err := uc.deps.Workspace.GetActiveWorkspace()
	if err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.workspace_get_active")).WithError(err)
	}

	if active == "" {
		fmt.Fprintln(w, i18n.T("output.workspace_no_active"))
		fmt.Fprintln(w, i18n.T("output.workspace_set_hint"))
		return nil
	}

	fmt.Fprintln(w, i18n.T("output.workspace_current", active))
	return nil
}

// Use sets the active workspace
func (uc *WorkspaceUseCase) Use(workspaceName string) error {
	if workspaceName == "" {
		return errors.New(errors.ErrCodeInvalidField, i18n.T("error.workspace_name_empty"))
	}

	w := uc.Out

	exists, err := uc.deps.Workspace.WorkspaceExists(workspaceName)
	if err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.workspace_check_exists")).WithError(err)
	}

	ws, err := uc.deps.Workspace.Resolve(workspaceName)
	if err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.workspace_create")).WithError(err)
	}

	if !exists {
		fmt.Fprintf(w, "ℹ️  %s\n", i18n.T("output.workspace_created_name", workspaceName))
	} else {
		fmt.Fprintf(w, "ℹ️  %s\n", i18n.T("output.workspace_already_exists", workspaceName))
	}

	if root.Exists(ws) {
		rootConfig, err := root.Load(ws)
		if err != nil {
			return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.workspace_load_root")).WithError(err)
		}
		if rootConfig != nil {
			fmt.Fprintf(w, "ℹ️  %s\n", i18n.T("output.workspace_loaded_root", workspaceName))
		}
	}

	oldWorkspace, _ := uc.deps.Workspace.GetActiveWorkspace()

	if err := uc.deps.Workspace.SetActiveWorkspace(workspaceName); err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.workspace_set_active")).WithError(err)
	}

	if oldWorkspace != workspaceName {
		if err := audit.LogWorkspaceChanged(oldWorkspace, workspaceName); err != nil {
			fmt.Fprintf(w, "⚠️  %s\n", i18n.T("output.failed_log_audit", err))
		}
	}

	fmt.Fprintf(w, "✔ %s\n", i18n.T("output.workspace_active_set", workspaceName))
	return nil
}

// Delete removes a workspace
func (uc *WorkspaceUseCase) Delete(workspaceName string) error {
	if workspaceName == "" {
		return errors.New(errors.ErrCodeInvalidField, i18n.T("error.workspace_name_empty"))
	}

	w := uc.Out

	exists, err := uc.deps.Workspace.WorkspaceExists(workspaceName)
	if err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.workspace_check_exists")).WithError(err)
	}

	if !exists {
		fmt.Fprintln(w, i18n.T("output.workspace_not_found", workspaceName))
		return nil
	}

	if err := uc.deps.Workspace.DeleteWorkspace(workspaceName); err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.workspace_delete")).WithError(err)
	}

	// Deactivate if it was the active workspace
	active, _ := uc.deps.Workspace.GetActiveWorkspace()
	if active == workspaceName {
		uc.deps.Workspace.SetActiveWorkspace("")
		fmt.Fprintln(w, i18n.T("output.workspace_deactivated", workspaceName))
	}

	fmt.Fprintf(w, "✔ %s\n", i18n.T("output.workspace_deleted", workspaceName))
	return nil
}

// List lists all available workspaces
func (uc *WorkspaceUseCase) List() error {
	w := uc.Out

	activeWorkspace, err := uc.deps.Workspace.GetActiveWorkspace()
	if err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.workspace_get_active")).WithError(err)
	}

	workspaces, err := uc.deps.Workspace.ListWorkspaces()
	if err != nil {
		return errors.New(errors.ErrCodeWorkspaceError, i18n.T("error.workspace_list")).WithError(err)
	}

	if len(workspaces) == 0 {
		fmt.Fprintln(w, i18n.T("output.workspace_no_workspaces"))
		fmt.Fprintln(w, i18n.T("output.workspace_create_hint"))
		return nil
	}

	fmt.Fprintln(w, i18n.T("output.workspace_available"))
	for _, ws := range workspaces {
		marker := " "
		if ws == activeWorkspace {
			marker = "*"
		}
		fmt.Fprintf(w, "  %s %s", marker, ws)
		if ws == activeWorkspace {
			fmt.Fprintf(w, " %s", i18n.T("output.workspace_active_label"))
		}
		fmt.Fprintln(w)
	}

	if activeWorkspace == "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, i18n.T("output.workspace_no_active"))
		fmt.Fprintln(w, i18n.T("output.workspace_set_hint"))
	}

	return nil
}
