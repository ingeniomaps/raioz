package orchestrate

import (
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/domain/interfaces"
	"raioz/internal/naming"
	"raioz/internal/runtime"

	"gopkg.in/yaml.v3"
)

// overlayPath is where raioz writes its network+labels overlay for
// compose-based dependencies. Lives in the same per-dep temp dir that
// image-based deps use — keeps the lifecycle symmetrical.
func (r *ImageRunner) overlayPath(svc interfaces.ServiceContext) string {
	dir := filepath.Dir(naming.DepComposePath(svc.ProjectName, svc.Name))
	return filepath.Join(dir, "raioz-overlay.yml")
}

// writeInfraOverlay renders the raioz overlay that layers on top of the
// user's compose fragment(s). Shape:
//
//	services:
//	  <depname>:
//	    networks: [<workspace-net>]
//	    labels: {com.raioz.managed: true, ...}
//	networks:
//	  <workspace-net>: {external: true}
//
// The overlay relies on service name match — the user's compose must
// expose a service whose name matches `dep.Name` in raioz.yaml (the common
// case: `services.postgres:` in postgres.yml pairing with
// `dependencies.postgres:` in raioz.yaml).
func (r *ImageRunner) writeInfraOverlay(svc interfaces.ServiceContext) (string, error) {
	// Shared deps omit com.raioz.project so raioz down of a single project
	// doesn't sweep them — mirrors the same logic in generateCompose.
	labelProject := svc.ProjectName
	if naming.WorkspaceName() != "" ||
		svc.ContainerName != naming.Container(svc.ProjectName, svc.Name) {
		labelProject = ""
	}
	labels := naming.Labels(
		naming.WorkspaceName(), labelProject, svc.Name, naming.KindDependency,
	)

	service := map[string]any{
		"networks": []any{svc.NetworkName, "default"},
		"labels":   labels,
	}
	// Same host-gateway alias every other raioz-owned container gets
	// (ADR-047 § H-1): discovery hands out host.docker.internal, and
	// without the mapping it does not resolve on Linux.
	if runtime.Supports(runtime.HostGatewayAlias) {
		service["extra_hosts"] = []string{"host.docker.internal:host-gateway"}
	}

	// A `resources:` block on the dependency replaces whatever limit its
	// compose file sets.
	applyResourceLimits(service, svc.Resources)

	overlay := map[string]any{
		"services": map[string]any{
			svc.Name: service,
		},
		"networks": map[string]any{
			svc.NetworkName: map[string]any{
				"external": true,
			},
		},
	}

	path := r.overlayPath(svc)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("mkdir overlay dir: %w", err)
	}
	data, err := yaml.Marshal(overlay)
	if err != nil {
		return "", fmt.Errorf("marshal overlay: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", fmt.Errorf("write overlay %q: %w", path, err)
	}
	return path, nil
}
