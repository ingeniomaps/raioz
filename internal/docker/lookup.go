package docker

import "context"

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
