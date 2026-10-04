package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"raioz/internal/app/upcase"
	"raioz/internal/discovery"
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/env"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/naming"
	"raioz/internal/output"
)

// EnvShowOptions contains options for the env show use case.
type EnvShowOptions struct {
	ConfigPath  string
	ServiceName string
}

// EnvShowUseCase displays environment variables for a service.
type EnvShowUseCase struct {
	deps *Dependencies
}

// NewEnvShowUseCase creates a new EnvShowUseCase.
func NewEnvShowUseCase(deps *Dependencies) *EnvShowUseCase {
	return &EnvShowUseCase{deps: deps}
}

// EnvEntry represents a single environment variable with its source.
type EnvEntry struct {
	Key    string
	Value  string
	Source string // "file" or "discovery"
}

// Execute computes and returns env vars for the given service.
func (uc *EnvShowUseCase) Execute(
	ctx context.Context,
	opts EnvShowOptions,
) ([]EnvEntry, error) {
	deps, warnings, err := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	for _, w := range warnings {
		output.PrintWarning(w)
	}
	if err != nil {
		return nil, err
	}
	// Container names carry the workspace prefix; without it every host
	// shown here is one that `up` never creates.
	naming.SetPrefix(deps.Workspace)

	svc, ok := deps.Services[opts.ServiceName]
	if !ok {
		if _, isInfra := deps.Infra[opts.ServiceName]; isInfra {
			return nil, errors.New(
				errors.ErrCodeInvalidConfig,
				i18n.T("env.is_dependency", opts.ServiceName),
			).WithSuggestion(
				i18n.T("env.is_dependency_suggestion"),
			)
		}
		return nil, errors.New(
			errors.ErrCodeInvalidConfig,
			i18n.T("env.service_not_found", opts.ServiceName),
		).WithSuggestion(i18n.T(
			"env.service_not_found_suggestion",
			joinServiceNames(deps),
		))
	}

	projectDir, _ := filepath.Abs(filepath.Dir(opts.ConfigPath))
	var entries []EnvEntry

	// 1. Computed variables (YAML/orchestrated mode)
	computed := map[string]bool{}
	if deps.SourceFormat == models.SourceFormatYAML {
		dm := uc.deps.DiscoveryManager
		if dm == nil {
			dm = discovery.NewManager()
		}
		for _, e := range resolveDiscoveryVars(ctx, dm, deps, projectDir, opts.ServiceName) {
			computed[e.Key] = true
			entries = append(entries, e)
		}
	}

	// 2. Env file variables. A computed var of the same name wins in the
	// process, so the file's value is not listed.
	for _, e := range resolveFileVars(uc.deps, deps, opts.ServiceName, svc, projectDir) {
		if !computed[e.Key] {
			entries = append(entries, e)
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})

	return entries, nil
}

func resolveFileVars(
	appDeps *Dependencies,
	deps *models.Deps,
	serviceName string,
	svc models.Service,
	projectDir string,
) []EnvEntry {
	ws, err := appDeps.Workspace.Resolve(deps.GetWorkspaceName())
	if err != nil {
		return nil
	}

	servicePath := filepath.Join(projectDir, svc.Source.Path)
	envPath, err := env.ResolveEnvFileForService(
		ws, deps, serviceName, svc.Env, projectDir, servicePath,
	)
	if err != nil || envPath == "" {
		return nil
	}

	vars, err := env.LoadFiles([]string{envPath})
	if err != nil {
		return nil
	}

	entries := make([]EnvEntry, 0, len(vars))
	for k, v := range vars {
		entries = append(entries, EnvEntry{
			Key: k, Value: v, Source: "file",
		})
	}
	return entries
}

// resolveDiscoveryVars returns the vars raioz computes for the service —
// the same recompute restart and the watcher relaunch with, so what this
// prints is what the process receives.
func resolveDiscoveryVars(
	ctx context.Context,
	dm interfaces.DiscoveryManager,
	deps *models.Deps,
	projectDir string,
	serviceName string,
) []EnvEntry {
	vars := upcase.ComputedServiceEnv(ctx, dm, containerLookup(), deps, projectDir, serviceName)

	entries := make([]EnvEntry, 0, len(vars))
	for k, v := range vars {
		entries = append(entries, EnvEntry{
			Key: k, Value: v, Source: "discovery",
		})
	}
	return entries
}

func parseFirstPort(portStr string) int {
	var port int
	// Sscanf errors surface as port == 0, which is the "unknown port"
	// sentinel the caller already expects.
	_, _ = fmt.Sscanf(portStr, "%d", &port)
	return port
}

func joinServiceNames(deps *models.Deps) string {
	names := make([]string, 0, len(deps.Services))
	for name := range deps.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	result := ""
	for i, name := range names {
		if i > 0 {
			result += ", "
		}
		result += name
	}
	return result
}
