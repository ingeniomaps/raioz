package app

import (
	"context"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/naming"
)

// labelLookup answers FindByLabels only for the workspace-scoped dep filter
// the infra overlay stamps when a workspace is set (no project label).
type labelLookup struct {
	existing map[string]bool
	byWSDep  map[string]string // dep name → container name
}

func (l *labelLookup) Exists(_ context.Context, name string) (bool, error) {
	return l.existing[name], nil
}

func (l *labelLookup) FindByLabels(_ context.Context, labels map[string]string) []string {
	if _, scopedToProject := labels[naming.LabelProject]; scopedToProject {
		return nil
	}
	if labels[naming.LabelKind] != naming.KindDependency || labels[naming.LabelWorkspace] == "" {
		return nil
	}
	if name, ok := l.byWSDep[labels[naming.LabelService]]; ok {
		return []string{name}
	}
	return nil
}

func stubContainerLookup(t *testing.T, l naming.ContainerLookup) {
	t.Helper()
	prev := containerLookup
	containerLookup = func() naming.ContainerLookup { return l }
	t.Cleanup(func() { containerLookup = prev })
}

func TestYAMLProject_ContainerState_WorkspaceComposeDep(t *testing.T) {
	naming.SetPrefix("dropi")
	t.Cleanup(func() { naming.SetPrefix("") })

	// rabbitmq is a `compose:` dep whose compose fixes container_name:
	// rabbitmq — the name breaks the <workspace>-<dep> convention.
	stubContainerLookup(t, &labelLookup{byWSDep: map[string]string{"rabbitmq": "rabbitmq"}})

	var probed string
	prev := dockerStateProbe
	dockerStateProbe = func(_ context.Context, name string) (ContainerState, bool) {
		probed = name
		return ContainerState{Status: statusRunning}, true
	}
	t.Cleanup(func() { dockerStateProbe = prev })

	p := &YAMLProject{
		ProjectName: "dropi",
		Deps: &models.Deps{
			Infra: map[string]models.InfraEntry{
				"rabbitmq": {Inline: &models.Infra{Compose: []string{"./rabbitmq.compose.yml"}}},
			},
			Services: map[string]models.Service{},
		},
	}

	tests := []struct {
		name       string
		target     string
		wantStatus string
		wantProbed string
	}{
		{"dep resolves through workspace labels", "rabbitmq", statusRunning, "rabbitmq"},
		// A service is never widened to the workspace: another project's
		// dep of the same name must not answer for it.
		{"non-dep keeps the project-scoped lookup", "other", statusStopped, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			probed = ""
			got := p.ContainerState(context.Background(), tc.target)
			if got.Status != tc.wantStatus {
				t.Errorf("status = %q, want %q", got.Status, tc.wantStatus)
			}
			if probed != tc.wantProbed {
				t.Errorf("probed %q, want %q", probed, tc.wantProbed)
			}
		})
	}
}

func TestYAMLProject_LiveContainerName_ServiceNotWidened(t *testing.T) {
	naming.SetPrefix("dropi")
	t.Cleanup(func() { naming.SetPrefix("") })
	stubContainerLookup(t, &labelLookup{byWSDep: map[string]string{"api": "other-project-api"}})

	p := &YAMLProject{
		ProjectName: "dropi",
		Deps: &models.Deps{
			Infra:    map[string]models.InfraEntry{},
			Services: map[string]models.Service{"api": {}},
		},
	}
	if got := p.liveContainerName(context.Background(), "api"); got != "" {
		t.Errorf("service resolved to %q through the workspace dep lookup, want \"\"", got)
	}
}
