package app

import (
	"context"
	"os"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/state"
)

// up records every service as "stopped"; list has to answer with what is
// running now — a live host PID or a running container, either one.
func TestRefreshLiveStatus(t *testing.T) {
	projectDir := t.TempDir()
	if err := state.SaveLocalState(projectDir, &models.LocalState{
		Project:  "bencha",
		HostPIDs: map[string]int{"web": os.Getpid(), "dead": 999999991},
	}); err != nil {
		t.Fatal(err)
	}
	stubContainerLookup(t, &projectLabelLookup{byService: map[string][]string{
		"api": {"raioz-bencha-api"},
	}})
	prev := dockerStateProbe
	dockerStateProbe = func(_ context.Context, name string) (ContainerState, bool) {
		return ContainerState{Status: statusRunning}, name == "raioz-bencha-api"
	}
	t.Cleanup(func() { dockerStateProbe = prev })

	gs := &models.GlobalState{
		ActiveProjects: []string{"bencha"},
		Projects: map[string]models.ProjectState{"bencha": {
			Name: "bencha", Path: projectDir,
			Services: []models.ServiceState{
				{Name: "web", Status: "stopped"},
				{Name: "api", Status: "stopped"},
				{Name: "dead", Status: "running"},
				{Name: "never", Status: "stopped"},
			},
		}},
	}

	NewListUseCase(&Dependencies{}).refreshLiveStatus(context.Background(), gs)

	want := map[string]string{
		"web": statusRunning, "api": statusRunning, "dead": statusStopped, "never": statusStopped,
	}
	for _, svc := range gs.Projects["bencha"].Services {
		if svc.Status != want[svc.Name] {
			t.Errorf("%s = %q, want %q", svc.Name, svc.Status, want[svc.Name])
		}
	}
}
