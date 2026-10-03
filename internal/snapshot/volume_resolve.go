package snapshot

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"raioz/internal/docker"
	"raioz/internal/naming"
	"raioz/internal/runtime"
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
//
// The entry is `name:/path`, and neither half is the volume's real name:
// compose prefixes it with the dep's compose project. The running
// container knows; when the project is down the known naming schemes are
// tried in order, newest first.
func resolveVolume(project, service, spec string) (name string, ok bool, err error) {
	src, dest, _ := strings.Cut(spec, ":")
	if dest, _, _ = strings.Cut(dest, ":"); src == "" {
		return "", false, nil
	}
	if strings.HasPrefix(src, "/") || strings.HasPrefix(src, ".") || strings.HasPrefix(src, "~") {
		return "", false, nil
	}

	ctx := context.Background()
	if container, _ := naming.ResolveDepContainer(ctx, docker.NewLookup(), project, service, ""); container != "" {
		if mounted := mountedVolume(ctx, container, dest); mounted != "" {
			return mounted, true, nil
		}
	}

	scoped := project + "_" + src
	candidates := []string{
		naming.DepComposeProjectNameFor(project, service, naming.WorkspaceName() != "") + "_" + scoped,
		naming.DepComposeProjectNameFor(project, service, false) + "_" + scoped,
		scoped,
		src,
	}
	for _, candidate := range candidates {
		if volumeExists(ctx, candidate) {
			return candidate, true, nil
		}
	}
	return "", true, fmt.Errorf(
		"no Docker volume found for %q of %q — start the project once so it is created", src, service)
}

// mountedVolume returns the named volume a container mounts at dest.
func mountedVolume(ctx context.Context, container, dest string) string {
	out, err := exec.CommandContext(ctx, runtime.Binary(), "inspect", "--format",
		`{{range .Mounts}}{{if eq .Type "volume"}}{{.Name}}={{.Destination}} {{end}}{{end}}`,
		container).Output()
	if err != nil {
		return ""
	}
	for _, pair := range strings.Fields(string(out)) {
		if name, at, found := strings.Cut(pair, "="); found && at == dest {
			return name
		}
	}
	return ""
}

func volumeExists(ctx context.Context, name string) bool {
	return exec.CommandContext(ctx, runtime.Binary(), "volume", "inspect", name).Run() == nil
}
