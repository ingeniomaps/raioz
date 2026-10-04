package docker

import (
	"context"
	"os/exec"
	"strings"

	"raioz/internal/naming"
	"raioz/internal/runtime"
)

// Lookup is the docker-package implementation of naming.ContainerLookup.
// It wraps the existing helpers (GetContainerStatusByName,
// ListContainersByLabels) without adding new behavior so the naming
// package can stay free of docker imports.
//
// Construct one with NewLookup() and pass it to naming.ResolveContainer /
// naming.ContainerTarget.
type Lookup struct{}

// NewLookup returns a zero-cost adapter wiring naming.ContainerLookup to
// the docker package's exec-based probes.
func NewLookup() Lookup { return Lookup{} }

// Exists reports whether a container with the given name exists. Mirrors
// GetContainerStatusByName: empty status string from `docker inspect`
// (i.e. the container is unknown) returns false.
func (Lookup) Exists(ctx context.Context, name string) (bool, error) {
	status, err := GetContainerStatusByName(ctx, name)
	if err != nil {
		return false, err
	}
	return status != "", nil
}

// FindByLabels delegates to ListContainersByLabels. Empty match list is
// not an error.
func (Lookup) FindByLabels(
	ctx context.Context, labels map[string]string,
) []string {
	return ListContainersByLabels(ctx, labels)
}

// ServiceContainerIP returns the network address of the container that
// runs a project's service, "" when there is none. A container that
// publishes no host port can only be reached there from the host.
func ServiceContainerIP(ctx context.Context, project, service string) string {
	names := ListContainersByLabels(ctx, map[string]string{
		naming.LabelManaged: "true",
		naming.LabelProject: project,
		naming.LabelService: service,
	})
	if len(names) == 0 {
		return ""
	}
	return ContainerIP(ctx, names[0])
}

// ContainerIP returns the network address of the container with the given
// name, "" when there is no such container or it has no address.
func ContainerIP(ctx context.Context, name string) string {
	out, err := exec.CommandContext(ctx, runtime.Binary(), "inspect", "--format",
		"{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}", name).Output()
	if err != nil {
		return ""
	}
	if fields := strings.Fields(string(out)); len(fields) > 0 {
		return fields[0]
	}
	return ""
}
