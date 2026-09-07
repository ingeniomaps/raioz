package docker

import (
	"context"
	"os"
	"sort"
	"strings"
)

func defaultOsEnviron() []string { return os.Environ() }

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

// composeExtraEnvKey carries ad-hoc env vars that should be exported to the
// docker compose subprocess. Separate from env-files so callers can inject
// computed values (e.g. `PROJECT_PREFIX=<workspace>`) without writing a
// scratch .env. Interpolation in the user's compose sees these.
type composeExtraEnvKey struct{}

// WithComposeExtraEnv returns a context carrying additional `KEY=value`
// pairs to export to docker compose. Merged with COMPOSE_PROJECT_NAME and
// whatever was already in os.Environ().
func WithComposeExtraEnv(ctx context.Context, env map[string]string) context.Context {
	if len(env) == 0 {
		return ctx
	}
	return context.WithValue(ctx, composeExtraEnvKey{}, env)
}

// composeExtraEnvFromContext returns a flat `KEY=value` slice for appending
// to cmd.Env. Stable-sorted so order is deterministic (helps tests).
func composeExtraEnvFromContext(ctx context.Context) []string {
	v, ok := ctx.Value(composeExtraEnvKey{}).(map[string]string)
	if !ok || len(v) == 0 {
		return nil
	}
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+v[k])
	}
	return out
}

// composeEnvFilesKey is the context key used to pass .env file paths through
// to docker compose invocations. When set, each file is prepended to the
// `docker compose` argv as `--env-file <path>` so ${VAR} interpolation in
// user-supplied compose fragments resolves correctly without having to
// shell-export the values beforehand.
type composeEnvFilesKey struct{}

// WithComposeEnvFiles returns a context carrying a list of .env files that
// should be passed to docker compose via --env-file flags. Typically populated
// for dependencies declared with `compose:` (user-supplied fragments) whose
// env:, publish:, etc. reference $POSTGRES_* / $REDIS_* etc.
func WithComposeEnvFiles(ctx context.Context, files []string) context.Context {
	if len(files) == 0 {
		return ctx
	}
	return context.WithValue(ctx, composeEnvFilesKey{}, append([]string(nil), files...))
}

// ComposeEnvFilesFromContext returns the env-file list stashed in ctx or nil.
// Exported because callers outside this package (runner.go, compose runners)
// need to splice `--env-file <path>` into their docker compose argv.
func ComposeEnvFilesFromContext(ctx context.Context) []string {
	if v, ok := ctx.Value(composeEnvFilesKey{}).([]string); ok {
		return v
	}
	return nil
}

// ComposeEnvFileArgs renders the env-file list carried by ctx as
// `--env-file <path1> --env-file <path2> ...`. Empty when no files are set.
func ComposeEnvFileArgs(ctx context.Context) []string {
	files := ComposeEnvFilesFromContext(ctx)
	args := make([]string, 0, len(files)*2)
	for _, f := range files {
		args = append(args, "--env-file", f)
	}
	return args
}

// composeCommandEnv builds the environment that the docker compose subprocess
// should run under. Base is os.Environ(); we append COMPOSE_PROJECT_NAME (if
// set) and any explicit KEY=value pairs from WithComposeExtraEnv. Used by
// every docker compose invocation in runner.go so the scoping contract is
// centralized in one place.
func composeCommandEnv(ctx context.Context) []string {
	env := osEnviron()
	if proj := composeProjectEnvFromContext(ctx); proj != "" {
		env = append(env, proj)
	}
	env = append(env, composeExtraEnvFromContext(ctx)...)
	return env
}

// osEnviron is a package-scoped indirection to os.Environ() so tests that
// want deterministic env can stub it. Production is just os.Environ.
var osEnviron = defaultOsEnviron

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
