package orchestrate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raioz/internal/domain/models"
	"raioz/internal/mocks"
	"raioz/internal/naming"
)

// A compose service's `resources:` caps each container of its stack
// through the overlay, never by touching the user's file.
func TestComposeRunner_Overlay_Resources(t *testing.T) {
	mock := &mocks.MockDockerRunner{
		GetAvailableServicesWithContextFunc: func(context.Context, string) ([]string, error) {
			return []string{"web", "db"}, nil
		},
	}
	r := &ComposeRunner{docker: mock}

	svc := makeComposeSvc(t)
	path, err := r.createNetworkOverlay(svc)
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); strings.Contains(string(data), "mem_limit") {
		t.Errorf("no block declared, no limit expected:\n%s", data)
	}

	svc.Resources = &models.Resources{Memory: "64m", CPUs: 0.5}
	if path, err = r.createNetworkOverlay(svc); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	body := string(data)
	for _, want := range []string{"mem_limit: 64m", "memswap_limit: 64m", "cpus: 0.5"} {
		if strings.Count(body, want) != 2 {
			t.Errorf("want %q on both services of the stack:\n%s", want, body)
		}
	}
}

// A compose dependency that declares `resources:` gets them in the raioz
// overlay, which replaces whatever its own file sets.
func TestImageRunner_InfraOverlay_Resources(t *testing.T) {
	userCompose := filepath.Join(t.TempDir(), "kv.yml")
	if err := os.WriteFile(userCompose, []byte("services:\n  kv:\n    image: redis\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := makeImageSvc()
	svc.ProjectName = "overlayres-" + t.Name()
	svc.ContainerName = naming.Container(svc.ProjectName, svc.Name)
	svc.ExternalComposeFiles = []string{userCompose}
	svc.Resources = &models.Resources{Memory: "128m"}
	t.Cleanup(func() { os.RemoveAll(naming.TempDir(svc.ProjectName)) })

	r := &ImageRunner{}
	path, err := r.writeInfraOverlay(svc)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	body := string(data)
	if !strings.Contains(body, "mem_limit: 128m") || !strings.Contains(body, "memswap_limit: 128m") {
		t.Errorf("limits missing from the overlay:\n%s", body)
	}
	if strings.Contains(body, "cpus") {
		t.Errorf("no cpu cap was declared:\n%s", body)
	}
}
