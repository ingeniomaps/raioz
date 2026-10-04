package app

import (
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/config"
	"raioz/internal/i18n"
	"raioz/internal/output"
)

// metaConfigFile is the config each meta sub-project is expected to carry.
const metaConfigFile = "raioz.yaml"

// CheckMeta validates a meta-orchestrator config: the meta file itself has
// already parsed, so what is left is every sub-project it names. One that
// `up` would start must be on disk with a config that loads; one that `up`
// clones, proxies or skips is reported as such. Nothing is started.
func CheckMeta(cfg *config.MetaConfig) error {
	fmt.Println()
	output.PrintSectionHeader(i18n.T("check.meta.header", len(cfg.Projects)))

	issues := 0
	projects := cfg.Projects
	if cfg.Router != nil {
		projects = append([]config.MetaProject{*cfg.Router}, projects...)
	}
	for _, p := range projects {
		if problem := checkMetaProject(p); problem != "" {
			output.PrintError(fmt.Sprintf("%s: %s", p.Name, problem))
			issues++
		}
	}

	fmt.Println()
	if issues == 0 {
		output.PrintSuccess(i18n.T("output.checks_passed"))
		return nil
	}
	output.PrintWarning(i18n.T("check.issues_found", issues))
	return &ExitCodeError{Code: 1, Err: fmt.Errorf("%d check issue(s) found", issues)}
}

// checkMetaProject reports one sub-project and returns what is wrong with
// it, or "" when it is fine.
func checkMetaProject(p config.MetaProject) string {
	switch p.Mode {
	case config.MetaModeClone:
		output.PrintInfo(i18n.T("check.meta.will_clone", p.Name, p.Git))
		return ""
	case config.MetaModeRemote:
		output.PrintInfo(i18n.T("check.meta.remote", p.Name, p.Remote))
		return ""
	case config.MetaModeSkip:
		output.PrintInfo(i18n.T("check.meta.optional_absent", p.Name))
		return ""
	}

	if _, err := os.Stat(p.Path); err != nil {
		return i18n.T("check.meta.path_missing", p.Path)
	}
	path := filepath.Join(p.Path, metaConfigFile)
	if sub, isMeta, err := config.LoadMetaConfig(path); isMeta {
		if err != nil {
			return err.Error()
		}
		output.PrintSuccess(i18n.T("check.meta.nested", p.Name, len(sub.Projects)))
		return ""
	}
	sub, err := config.LoadYAML(path)
	if err != nil {
		return err.Error()
	}
	output.PrintSuccess(i18n.T("check.meta.project_ok", p.Name, sub.Project, len(sub.Services), len(sub.Deps)))
	return ""
}
