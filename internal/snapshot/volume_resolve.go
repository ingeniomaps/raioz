package snapshot

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"raioz/internal/docker"
)

// snapshotNameRE is what a snapshot may be called. The name becomes a
// directory under the snapshot store, so anything that could walk out of
// it (separators, `..`) is refused before a path is ever built.
var snapshotNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func validateName(kind, name string) error {
	if !snapshotNameRE.MatchString(name) || strings.Contains(name, "..") {
		return fmt.Errorf("invalid %s name %q: use letters, digits, '.', '_' and '-'", kind, name)
	}
	return nil
}

// volumeResolver maps a dependency's `volumes:` entry to the Docker volume
// that holds its data. Package var so tests run without a docker daemon.
var volumeResolver = resolveVolume

// resolveVolume returns the Docker volume behind one `volumes:` entry of a
// dependency. ok is false for a bind mount — there is no volume to archive.
func resolveVolume(project, service, spec string) (name string, ok bool, err error) {
	name, named, exists := docker.DepVolume(context.Background(), project, service, spec)
	if !named {
		return "", false, nil
	}
	if !exists {
		return "", true, fmt.Errorf(
			"no Docker volume found for %q of %q — start the project once so it is created", spec, service)
	}
	return name, true, nil
}
