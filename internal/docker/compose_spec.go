package docker

import (
	"context"
	"strings"
)

// composeProjectNameKey is the context key used to inject an explicit docker
// compose project name into up/down/status/logs invocations. When set, the
// docker layer exports COMPOSE_PROJECT_NAME to the spawned subprocess so
// containers are scoped to that project instead of defaulting to the basename
// of the compose file directory. This prevents `--remove-orphans` from
// accidentally sweeping containers from unrelated projects that share a dir.
type composeProjectNameKey struct{}

// WithComposeProjectName returns a context carrying an explicit docker compose
// project name. Pass the returned ctx to docker.UpWithContext / DownWithContext
// / GetServicesStatusWithContext / ViewLogsWithContext to force the scope.
func WithComposeProjectName(ctx context.Context, name string) context.Context {
	if name == "" {
		return ctx
	}
	return context.WithValue(ctx, composeProjectNameKey{}, name)
}

// composeProjectEnvFromContext returns `COMPOSE_PROJECT_NAME=<name>` if the
// context carries one, or an empty string otherwise. Append the result to
// cmd.Env to scope the docker compose invocation.
func composeProjectEnvFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(composeProjectNameKey{}).(string); ok && v != "" {
		return "COMPOSE_PROJECT_NAME=" + v
	}
	return ""
}

// composePathSeparator is the internal separator raioz uses to pass multiple
// compose file paths through a single `composePath string` parameter without
// breaking the existing DockerRunner interface. Colon is not a valid filesystem
// character on Linux and is not rejected by ValidateComposePath per-piece, so
// it is safe as a sentinel.
const composePathSeparator = ":"

// SplitComposePaths splits a composePath parameter on the internal separator
// and returns the individual compose file paths. A single-file path returns a
// one-element slice. Empty segments are dropped.
func SplitComposePaths(composePath string) []string {
	parts := strings.Split(composePath, composePathSeparator)
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// JoinComposePaths combines multiple compose file paths into a single
// composePath parameter using the internal separator.
func JoinComposePaths(paths []string) string {
	return strings.Join(paths, composePathSeparator)
}

// ComposeFileArgs returns `-f file1 -f file2 ...` for a composePath spec.
// Pass the result directly where docker compose expects `-f` flags.
func ComposeFileArgs(composePath string) []string {
	parts := SplitComposePaths(composePath)
	args := make([]string, 0, len(parts)*2)
	for _, p := range parts {
		args = append(args, "-f", p)
	}
	return args
}

// PrimaryComposeFile returns the first compose file in a multi-file spec.
// Used for operations that only need one file (e.g. os.Stat existence checks,
// docker compose config queries that don't need the overlay).
func PrimaryComposeFile(composePath string) string {
	parts := SplitComposePaths(composePath)
	if len(parts) == 0 {
		return composePath
	}
	return parts[0]
}
