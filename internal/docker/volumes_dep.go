package docker

import (
	"context"
	"os/exec"
	"strings"

	"raioz/internal/naming"
	"raioz/internal/runtime"
)

// DepVolume resolves one `volumes:` entry of a dependency to the Docker
// volume that holds its data.
//
// named is false for a bind mount — there is no volume. exists tells
// whether the returned volume is there; when it is not, name is the one
// the next `up` will create.
//
// Neither half of `name:/path` is the volume's real name: compose prefixes
// it with the dependency's compose project. The running container knows;
// with the project down, the naming schemes raioz has used are tried,
// current first.
func DepVolume(ctx context.Context, project, dep, spec string) (name string, named, exists bool) {
	src, dest, _ := strings.Cut(spec, ":")
	dest, _, _ = strings.Cut(dest, ":")
	if src == "" || strings.HasPrefix(src, "/") || strings.HasPrefix(src, ".") || strings.HasPrefix(src, "~") {
		return "", false, false
	}

	if container, _ := naming.ResolveDepContainer(ctx, NewLookup(), project, dep, ""); container != "" {
		if mounted := mountedVolume(ctx, container, dest); mounted != "" {
			return mounted, true, true
		}
	}

	scoped := project + "_" + src
	candidates := []string{
		naming.DepComposeProjectNameFor(project, dep, naming.WorkspaceName() != "") + "_" + scoped,
		naming.DepComposeProjectNameFor(project, dep, false) + "_" + scoped,
		scoped,
		src,
	}
	for _, candidate := range candidates {
		if volumeExists(ctx, candidate) {
			return candidate, true, true
		}
	}
	return candidates[0], true, false
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
