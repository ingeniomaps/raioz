package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/naming"
)

// projectLabelLookup answers the project-scoped label filter a service's
// containers carry (managed + project + service).
type projectLabelLookup struct {
	byService map[string][]string
}

func (l *projectLabelLookup) Exists(context.Context, string) (bool, error) { return false, nil }

func (l *projectLabelLookup) FindByLabels(_ context.Context, labels map[string]string) []string {
	if labels[naming.LabelProject] == "" {
		return nil
	}
	return l.byService[labels[naming.LabelService]]
}

// dockerServiceProject is a project whose `site` service runs as a compose
// stack with its own container_name — the canonical name does not exist.
func dockerServiceProject(t *testing.T) *YAMLProject {
	t.Helper()
	siteDir := writeServiceDir(t, map[string]string{
		"docker-compose.yml": "services:\n  site:\n    image: nginx:alpine\n    container_name: rzb-site-custom\n",
	})
	stubContainerLookup(t, &projectLabelLookup{byService: map[string][]string{
		"site": {"rzb-site-custom"},
	}})
	prev := dockerStateProbe
	dockerStateProbe = func(_ context.Context, name string) (ContainerState, bool) {
		if name == "rzb-site-custom" {
			return ContainerState{Status: statusRunning}, true
		}
		return ContainerState{}, false
	}
	t.Cleanup(func() { dockerStateProbe = prev })

	return &YAMLProject{
		ProjectName: "rzb1",
		ConfigPath:  filepath.Join(t.TempDir(), "raioz.yaml"),
		Deps: &models.Deps{
			Project: models.Project{Name: "rzb1"},
			Services: map[string]models.Service{
				"site": {Source: models.SourceConfig{Kind: "local", Path: siteDir}},
			},
			Infra: map[string]models.InfraEntry{},
		},
	}
}

// status only looked at host PIDs, so a container service was always
// reported stopped.
func TestStatusYAML_DockerServiceReadsContainerState(t *testing.T) {
	initI18nForTest(t)
	proj := dockerServiceProject(t)

	out := captureStdout(t, func() {
		if err := NewStatusUseCase(&Dependencies{}).StatusYAML(context.Background(), proj, nil); err != nil {
			t.Fatalf("StatusYAML: %v", err)
		}
	})
	if !strings.Contains(out, "compose    running") {
		t.Errorf("status does not report the compose service running:\n%s", out)
	}
}

func TestServiceVerdict_DockerService(t *testing.T) {
	initI18nForTest(t)
	proj := dockerServiceProject(t)

	v := NewHealthUseCase(&Dependencies{}).serviceVerdict(
		context.Background(), proj, "site", proj.Deps.Services["site"], nil)
	if !v.healthy || v.status != statusRunning {
		t.Errorf("verdict = %+v, want a running, healthy service", v)
	}
}

func TestLiveContainerNames(t *testing.T) {
	proj := dockerServiceProject(t)
	stubContainerLookup(t, &projectLabelLookup{byService: map[string][]string{
		"site": {"stack-web-1", "stack-worker-1"},
	}})

	tests := []struct {
		name string
		svc  string
		want []string
	}{
		{"every container of a compose service", "site", []string{"stack-web-1", "stack-worker-1"}},
		{"nothing running", "ghost", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := proj.liveContainerNames(context.Background(), tc.svc)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// A Dockerfile service that publishes no host port answers its health
// endpoint on the container's address; probing loopback alone called a
// healthy service unhealthy.
func TestServiceVerdict_DockerServiceEndpointOnContainerAddress(t *testing.T) {
	initI18nForTest(t)
	proj := dockerServiceProject(t)
	svc := proj.Deps.Services["site"]
	svc.Port, svc.HealthEndpoint = 3000, "/health"

	prevIP := serviceContainerIP
	serviceContainerIP = func(context.Context, string, string) string { return "10.213.0.8" }
	t.Cleanup(func() { serviceContainerIP = prevIP })

	var probed []string
	prevProbe := healthEndpointProbe
	healthEndpointProbe = func(_ context.Context, url string) bool {
		probed = append(probed, url)
		return strings.Contains(url, "10.213.0.8")
	}
	t.Cleanup(func() { healthEndpointProbe = prevProbe })

	v := NewHealthUseCase(&Dependencies{}).serviceVerdict(context.Background(), proj, "site", svc, nil)
	if !v.healthy {
		t.Errorf("verdict = %+v, want healthy (probed %v)", v, probed)
	}
	if v.detail != "http://10.213.0.8:3000/health" {
		t.Errorf("detail = %q, want the address that answered", v.detail)
	}
}

// --json prints the same facts as the table, as JSON and nothing else.
func TestStatusJSON(t *testing.T) {
	initI18nForTest(t)
	proj := dockerServiceProject(t)

	out := captureStdout(t, func() {
		if err := NewStatusUseCase(&Dependencies{}).statusJSON(context.Background(), proj, nil); err != nil {
			t.Fatalf("statusJSON: %v", err)
		}
	})

	var report statusReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	if report.Project != "rzb1" || len(report.Services) != 1 {
		t.Fatalf("report = %+v", report)
	}
	if svc := report.Services[0]; svc.Name != "site" || svc.Runtime != "compose" || svc.Status != statusRunning {
		t.Errorf("service = %+v, want site/compose/running", svc)
	}
}

// A dependency that is another raioz project is up when that project is.
func TestSiblingDependencyStatus(t *testing.T) {
	projectDir := t.TempDir()
	sibling := filepath.Join(filepath.Dir(projectDir), "rzb2")

	prev := recordedProjects
	recordedProjects = func() []models.ProjectState {
		return []models.ProjectState{{Name: "rzb2", Path: sibling}}
	}
	t.Cleanup(func() { recordedProjects = prev })

	rel, err := filepath.Rel(projectDir, sibling)
	if err != nil {
		t.Fatal(err)
	}
	if got := siblingDependencyStatus("sib", rel, projectDir); got.Status != statusRunning {
		t.Errorf("active sibling reported %q", got.Status)
	}
	if got := siblingDependencyStatus("sib", "../absent", projectDir); got.Status != statusStopped {
		t.Errorf("inactive sibling reported %q", got.Status)
	}
}
