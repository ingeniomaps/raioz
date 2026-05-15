package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"raioz/internal/domain/models"
	"raioz/internal/i18n"
	"raioz/internal/output"
)

// warnedJSONDeprecation makes the .raioz.json deprecation banner fire
// exactly once per process even when LoadDeps is called repeatedly
// (e.g. dependency_assist scanning sub-projects). See ADR-038.
var warnedJSONDeprecation sync.Once

// ResetJSONDeprecationWarningForTest clears the dedup so a test can
// verify the warning fires on the first hit without depending on
// test ordering. Test-only.
func ResetJSONDeprecationWarningForTest() {
	warnedJSONDeprecation = sync.Once{}
}

// Type aliases keep `config.Foo` callers compiling while the canonical
// definitions live in internal/domain/models (see ADR-009).
type (
	Deps                = models.Deps
	Project             = models.Project
	ProjectCommands     = models.ProjectCommands
	EnvironmentCommands = models.EnvironmentCommands
	EnvConfig           = models.EnvConfig
	Service             = models.Service
	ServiceCommands     = models.ServiceCommands
	SourceConfig        = models.SourceConfig
	DockerConfig        = models.DockerConfig
	SourceFormat        = models.SourceFormat
)

// Re-export the SourceFormat constants so callers can read
// `config.SourceFormatYAML` without reaching into domain/models.
const (
	SourceFormatLegacyJSON = models.SourceFormatLegacyJSON
	SourceFormatYAML       = models.SourceFormatYAML
)

// LegacyProject mirrors the old config shape where network lived inside
// project. Used solely by LoadDeps to migrate to the current layout.
type LegacyProject struct {
	Name     string           `json:"name"`
	Network  NetworkConfig    `json:"network,omitempty"`
	Commands *ProjectCommands `json:"commands,omitempty"`
	Env      *EnvValue        `json:"env,omitempty"`
}

func LoadDeps(path string) (*Deps, []string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read config %q: %w", path, err)
	}

	warnedJSONDeprecation.Do(func() {
		output.PrintWarning(i18n.T("warning.json_format_deprecated"))
	})

	warnings, err := CheckDeprecatedFields(data)
	if err != nil {
		warnings = []string{}
	}

	var legacyStruct struct {
		Project            LegacyProject         `json:"project"`
		Network            NetworkConfig         `json:"network,omitempty"`
		SchemaVersion      string                `json:"schemaVersion"`
		Workspace          string                `json:"workspace,omitempty"`
		Profiles           []string              `json:"profiles,omitempty"`
		Services           map[string]Service    `json:"services"`
		Infra              map[string]InfraEntry `json:"infra"`
		Env                EnvConfig             `json:"env"`
		ProjectComposePath string                `json:"projectComposePath,omitempty"`
	}
	if err := json.Unmarshal(data, &legacyStruct); err != nil {
		return nil, nil, fmt.Errorf("parse legacy config %q: %w", path, err)
	}
	if legacyStruct.Infra == nil {
		legacyStruct.Infra = make(map[string]InfraEntry)
	}

	deps := Deps{
		SchemaVersion: legacyStruct.SchemaVersion,
		SourceFormat:  SourceFormatLegacyJSON,
		Workspace:     legacyStruct.Workspace,
		Project: Project{
			Name:     legacyStruct.Project.Name,
			Commands: legacyStruct.Project.Commands,
			Env:      legacyStruct.Project.Env,
		},
		Profiles:           legacyStruct.Profiles,
		Services:           legacyStruct.Services,
		Infra:              legacyStruct.Infra,
		Env:                legacyStruct.Env,
		ProjectComposePath: legacyStruct.ProjectComposePath,
	}

	if legacyStruct.Network.Name != "" {
		deps.Network = legacyStruct.Network
	} else if legacyStruct.Project.Network.Name != "" {
		deps.Network = legacyStruct.Project.Network
	}

	return &deps, warnings, nil
}

// LoadDepsLegacy is a compatibility wrapper that ignores warnings.
// Deprecated: Use LoadDeps instead to get deprecation warnings.
func LoadDepsLegacy(path string) (*Deps, error) {
	deps, _, err := LoadDeps(path)
	return deps, err
}

// FilterByProfile filters services and infra by the given profile.
// Services/infra with no profiles are always included; otherwise only those matching the profile are included.
func FilterByProfile(deps *Deps, profile string) *Deps {
	filtered := &Deps{
		SchemaVersion:      deps.SchemaVersion,
		SourceFormat:       deps.SourceFormat,
		Workspace:          deps.Workspace,
		Network:            deps.Network,
		Project:            deps.Project,
		Profiles:           deps.Profiles,
		Services:           make(map[string]Service),
		Infra:              make(map[string]InfraEntry),
		Env:                deps.Env,
		ProjectComposePath: deps.ProjectComposePath,
		Proxy:              deps.Proxy,
		ProxyConfig:        deps.ProxyConfig,
		PreHook:            deps.PreHook,
		PreUpHook:          deps.PreUpHook,
		PostHook:           deps.PostHook,
	}

	for name, svc := range deps.Services {
		if svc.Enabled != nil && !*svc.Enabled {
			continue
		}
		if len(svc.Profiles) == 0 {
			filtered.Services[name] = svc
		} else {
			for _, p := range svc.Profiles {
				if p == profile {
					filtered.Services[name] = svc
					break
				}
			}
		}
	}

	for name, entry := range deps.Infra {
		profs := entry.Profiles()
		if len(profs) == 0 {
			filtered.Infra[name] = entry
		} else {
			for _, p := range profs {
				if p == profile {
					filtered.Infra[name] = entry
					break
				}
			}
		}
	}

	return filtered
}

// FilterByProfiles filters services and infra by a list of profiles.
// Items with no profiles are always included; otherwise included if any profile matches.
func FilterByProfiles(deps *Deps, profiles []string) *Deps {
	if len(profiles) == 0 {
		return deps
	}
	allowed := make(map[string]bool)
	for _, p := range profiles {
		allowed[p] = true
	}
	filtered := &Deps{
		SchemaVersion:      deps.SchemaVersion,
		SourceFormat:       deps.SourceFormat,
		Workspace:          deps.Workspace,
		Network:            deps.Network,
		Project:            deps.Project,
		Profiles:           deps.Profiles,
		Services:           make(map[string]Service),
		Infra:              make(map[string]InfraEntry),
		Env:                deps.Env,
		ProjectComposePath: deps.ProjectComposePath,
		Proxy:              deps.Proxy,
		ProxyConfig:        deps.ProxyConfig,
		PreHook:            deps.PreHook,
		PreUpHook:          deps.PreUpHook,
		PostHook:           deps.PostHook,
	}
	for name, svc := range deps.Services {
		if svc.Enabled != nil && !*svc.Enabled {
			continue
		}
		if len(svc.Profiles) == 0 {
			filtered.Services[name] = svc
		} else {
			for _, p := range svc.Profiles {
				if allowed[p] {
					filtered.Services[name] = svc
					break
				}
			}
		}
	}
	for name, entry := range deps.Infra {
		profs := entry.Profiles()
		if len(profs) == 0 {
			filtered.Infra[name] = entry
		} else {
			for _, p := range profs {
				if allowed[p] {
					filtered.Infra[name] = entry
					break
				}
			}
		}
	}
	return filtered
}
