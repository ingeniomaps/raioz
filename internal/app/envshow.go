package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"raioz/internal/config"
	"raioz/internal/detect"
	"raioz/internal/discovery"
	"raioz/internal/domain/interfaces"
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
	_ context.Context,
	opts EnvShowOptions,
) ([]EnvEntry, error) {
	deps, warnings, err := uc.deps.ConfigLoader.LoadDeps(opts.ConfigPath)
	for _, w := range warnings {
		output.PrintWarning(w)
	}
	if err != nil {
		return nil, err
	}

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

	// 1. Env file variables
	entries = append(entries, resolveFileVars(
		uc.deps, deps, opts.ServiceName, svc, projectDir,
	)...)

	// 2. Discovery variables (YAML/orchestrated mode)
	if deps.SchemaVersion == "2.0" {
		entries = append(entries, resolveDiscoveryVars(
			deps, opts.ServiceName, projectDir,
		)...)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})

	return entries, nil
}

func resolveFileVars(
	appDeps *Dependencies,
	deps *config.Deps,
	serviceName string,
	svc config.Service,
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

func resolveDiscoveryVars(
	deps *config.Deps,
	serviceName string,
	projectDir string,
) []EnvEntry {
	detections := make(map[string]detect.DetectResult)
	for name, svc := range deps.Services {
		path := filepath.Join(projectDir, svc.Source.Path)
		detections[name] = detect.Detect(path)
	}
	for name, entry := range deps.Infra {
		if entry.Inline != nil && entry.Inline.Image != "" {
			detections[name] = detect.DetectResult{
				Runtime: detect.RuntimeImage,
			}
		}
	}

	endpoints := make(map[string]interfaces.ServiceEndpoint)
	for name, det := range detections {
		ep := interfaces.ServiceEndpoint{
			Name:    name,
			Runtime: det.Runtime,
			Port:    det.Port,
		}
		if det.IsDocker() {
			ep.Host = naming.Container(deps.Project.Name, name)
		} else {
			ep.Host = "localhost"
		}
		if s, ok := deps.Services[name]; ok &&
			s.Docker != nil && len(s.Docker.Ports) > 0 {
			ep.Port = parseFirstPort(s.Docker.Ports[0])
		}
		if e, ok := deps.Infra[name]; ok &&
			e.Inline != nil && len(e.Inline.Ports) > 0 {
			ep.Port = parseFirstPort(e.Inline.Ports[0])
		}
		endpoints[name] = ep
	}

	svcDet := detections[serviceName]
	mgr := discovery.NewManager()
	vars := mgr.GenerateEnvVars(
		serviceName, svcDet.Runtime, endpoints, deps.Proxy,
	)

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

func joinServiceNames(deps *config.Deps) string {
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
