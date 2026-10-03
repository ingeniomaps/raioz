package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"raioz/internal/domain/models"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/naming"
	"raioz/internal/output"
	"raioz/internal/state"
)

// ListOptions holds options for the list use case
type ListOptions struct {
	JSONOutput bool
	Filter     string
	Status     string
}

// ListUseCase handles listing active projects
type ListUseCase struct {
	deps *Dependencies
}

// NewListUseCase creates a new ListUseCase
func NewListUseCase(deps *Dependencies) *ListUseCase {
	return &ListUseCase{deps: deps}
}

// Execute runs the list use case
func (uc *ListUseCase) Execute(opts ListOptions) error {
	globalState, err := uc.deps.StateManager.LoadGlobalState()
	if err != nil {
		return errors.New(errors.ErrCodeStateLoadError, i18n.T("error.list_load_state")).WithError(err)
	}

	// The stored status is whatever up wrote; answer with what is true now.
	uc.refreshLiveStatus(context.Background(), globalState)

	// Apply filters
	filteredState := uc.applyFilters(globalState, opts)

	if opts.JSONOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(filteredState); err != nil {
			return errors.New(errors.ErrCodeInternalError, i18n.T("error.list_encode_json")).WithError(err)
		}
		return nil
	}

	// Table output
	if len(filteredState.ActiveProjects) == 0 {
		if opts.Filter != "" || opts.Status != "" {
			output.PrintInfo(i18n.T("output.no_projects_match_filters"))
		} else {
			output.PrintInfo(i18n.T("output.no_active_projects"))
		}
		return nil
	}

	output.PrintSectionHeader(i18n.T("output.active_projects_header"))

	for i, projectName := range filteredState.ActiveProjects {
		projectState, exists := filteredState.Projects[projectName]
		if !exists {
			continue
		}

		if i > 0 {
			fmt.Println()
		}

		output.PrintSubsection(projectState.Name)
		output.PrintKeyValue(i18n.T("output.label_workspace"), projectState.Workspace)
		output.PrintKeyValue(i18n.T("output.label_last_execution"), formatTime(projectState.LastExecution))
		output.PrintKeyValue(i18n.T("output.label_active_services"), fmt.Sprintf("%d", len(projectState.Services)))

		// Show service summary
		if len(projectState.Services) > 0 {
			runningCount := 0
			for _, svc := range projectState.Services {
				if svc.Status == "running" {
					runningCount++
				}
			}
			output.PrintKeyValue(i18n.T("output.label_running"), fmt.Sprintf("%d/%d", runningCount, len(projectState.Services)))

			if len(projectState.Services) <= 5 {
				serviceNames := make([]string, 0, len(projectState.Services))
				for _, svc := range projectState.Services {
					statusIndicator := "●"
					if svc.Status == "running" {
						statusIndicator = "✓"
					}
					serviceNames = append(serviceNames, fmt.Sprintf("%s %s", statusIndicator, svc.Name))
				}
				output.PrintKeyValue(i18n.T("output.label_services"), strings.Join(serviceNames, ", "))
			}
		}
	}

	return nil
}

// refreshLiveStatus overwrites each service's recorded status with its
// live one. A service is running when its recorded host PID is alive or a
// container carrying its labels is — the two ways raioz launches things.
func (uc *ListUseCase) refreshLiveStatus(ctx context.Context, globalState *models.GlobalState) {
	for name, project := range globalState.Projects {
		hostPIDs := uc.projectHostPIDs(project)
		for i := range project.Services {
			svc := &project.Services[i]
			svc.Status = statusStopped
			if pid := hostPIDs[svc.Name]; pid > 0 && processAlive(pid) {
				svc.Status = statusRunning
				continue
			}
			for _, container := range containerLookup().FindByLabels(ctx, map[string]string{
				naming.LabelManaged: "true",
				naming.LabelProject: project.Name,
				naming.LabelService: svc.Name,
			}) {
				if st, ok := dockerStateProbe(ctx, container); ok && st.Status == statusRunning {
					svc.Status = statusRunning
					break
				}
			}
		}
		globalState.Projects[name] = project
	}
}

// projectHostPIDs reads the host PIDs a project recorded at up: from its
// own directory when the state knows it, else from the workspace root.
func (uc *ListUseCase) projectHostPIDs(project models.ProjectState) map[string]int {
	if project.Path != "" {
		if ls, err := state.LoadLocalState(project.Path); err == nil && ls != nil {
			return ls.HostPIDs
		}
	}
	return uc.loadHostPIDs(project.Workspace)
}

// applyFilters applies name and status filters to the global state
func (uc *ListUseCase) applyFilters(globalState *models.GlobalState, opts ListOptions) *models.GlobalState {
	if opts.Filter == "" && opts.Status == "" {
		return globalState
	}

	filtered := &models.GlobalState{
		ActiveProjects: []string{},
		Projects:       make(map[string]models.ProjectState),
	}

	for _, projectName := range globalState.ActiveProjects {
		projectState, exists := globalState.Projects[projectName]
		if !exists {
			continue
		}

		// Apply name filter
		if opts.Filter != "" && !strings.Contains(strings.ToLower(projectName), strings.ToLower(opts.Filter)) {
			continue
		}

		// Apply status filter
		if opts.Status != "" {
			hasMatchingStatus := false
			for _, svc := range projectState.Services {
				if strings.EqualFold(svc.Status, opts.Status) {
					hasMatchingStatus = true
					break
				}
			}
			if !hasMatchingStatus {
				continue
			}
		}

		filtered.ActiveProjects = append(filtered.ActiveProjects, projectName)
		filtered.Projects[projectName] = projectState
	}

	return filtered
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return i18n.T("time.never")
	}
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return i18n.T("time.just_now")
	}
	if diff < time.Hour {
		minutes := int(diff.Minutes())
		return i18n.T("time.minutes_ago", minutes)
	}
	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		return i18n.T("time.hours_ago", hours)
	}
	if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		return i18n.T("time.days_ago", days)
	}

	return t.Format("2006-01-02 15:04:05")
}

// loadHostPIDs loads host process PIDs for a project.
// Reads the workspace state to find projectRoot, then loads PIDs from there.
func (uc *ListUseCase) loadHostPIDs(workspace string) map[string]int {
	if workspace == "" {
		return nil
	}

	ws, err := uc.deps.Workspace.Resolve(workspace)
	if err != nil {
		return nil
	}

	// ADR-011 Phase 2: the legacy snapshot used to record ProjectRoot so
	// list could find LocalState for YAML-mode projects living outside
	// the workspace dir. With the snapshot gone, fall back to the
	// workspace-root LocalState only — host PIDs show up only when the
	// user runs from within the project (which writes LocalState there)
	// AND the workspace LocalState also carries the PIDs.
	wsRoot := uc.deps.Workspace.GetRoot(ws)
	ls, lsErr := state.LoadLocalState(wsRoot)
	if lsErr == nil && ls != nil && len(ls.HostPIDs) > 0 {
		return ls.HostPIDs
	}

	return nil
}

// processAlive checks if a PID is running.
func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
