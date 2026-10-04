package app

import (
	"fmt"
	"os"

	"raioz/internal/app/upcase"
	"raioz/internal/config"
	"raioz/internal/domain/models"
	"raioz/internal/i18n"
	"raioz/internal/output"
)

func CheckYAML(proj *YAMLProject) error {
	fmt.Println()
	output.PrintSectionHeader(i18n.T("output.check_section_header", proj.ProjectName))

	issues := 0

	// What the loader noticed is shown. An ignored field also fails the
	// check — it is a typo the user asked check to find; the rest
	// (unpinned image, legacy `ports:`) is advice.
	unknown := make(map[string]bool)
	for _, w := range config.UnknownFields(proj.ConfigPath) {
		unknown[w] = true
	}
	for _, warning := range proj.Warnings {
		output.PrintWarning(warning)
		if unknown[warning] {
			issues++
		}
	}

	// Check service paths exist (honoring yaml `command:`/`compose:` overrides).
	for name, svc := range proj.Deps.Services {
		result := config.ResolveServiceDetection(svc, svc.Source.Path)
		if result.Runtime == models.RuntimeUnknown && awaitingClone(svc) {
			// Nothing to detect yet: `up` clones the repository first.
			output.PrintInfo(i18n.T("check.git_not_cloned", name, svc.Source.Repo))
			continue
		}
		if result.Runtime == models.RuntimeUnknown {
			if svc.Source.Path != "" {
				output.PrintWarning(i18n.T("check.no_runtime_at", name, svc.Source.Path))
			} else {
				output.PrintWarning(i18n.T("check.no_runtime_declared", name))
			}
			issues++
		} else {
			output.PrintSuccess(fmt.Sprintf("%s: %s", name, result.Runtime))
		}
	}

	// Check dependency images
	for _, name := range sortedKeysInfra(proj.Deps.Infra) {
		entry := proj.Deps.Infra[name]
		switch {
		case entry.Inline == nil:
		case entry.Inline.Image != "":
			output.PrintSuccess(fmt.Sprintf("%s: %s", name, entry.Inline.Image))
		case len(entry.Inline.Compose) > 0:
			// A compose dependency names no image of its own; leaving it
			// out made the report look like the dependency was missing.
			output.PrintSuccess(fmt.Sprintf("%s: %s", name, models.RuntimeCompose))
		}
	}

	// Check dependsOn references
	known := make(map[string]bool)
	for name := range proj.Deps.Services {
		known[name] = true
	}
	for name := range proj.Deps.Infra {
		known[name] = true
	}
	for name, svc := range proj.Deps.Services {
		for _, dep := range svc.GetDependsOn() {
			if !known[dep] {
				output.PrintError(i18n.T("check.unknown_dependency", name, dep))
				issues++
			}
		}
	}

	// Proxy requirements (mkcert presence, certs on disk). Matches what
	// `raioz up` enforces so the user never gets a green check followed by
	// a red up on the same machine.
	if err := upcase.CheckProxyRequirements(proj.Deps); err != nil {
		output.PrintError(err.Error())
		issues++
	}

	// Port allocation + host-bind probing. This runs the same allocator the
	// up flow uses: explicit conflicts fail loud, implicit/auto conflicts
	// bump deterministically, external binders (other projects, random
	// containers, local processes) are surfaced as errors pointing at the
	// offending service or dep.
	//
	// `raioz check` runs this read-only — nothing is actually bound, just
	// a transient net.Listen() per candidate port to probe availability.
	detections := upcase.BuildDetectionMap(proj.Deps)
	if _, err := upcase.AllocateHostPorts(proj.Deps, detections); err != nil {
		output.PrintError(err.Error())
		issues++
	}

	fmt.Println()
	if issues == 0 {
		output.PrintSuccess(i18n.T("output.checks_passed"))
		return nil
	}
	// Issues found: return a sentinel error so the CLI wrapper (cli/check.go)
	// can skip the misleading "Configuration is valid" banner, surface a
	// non-zero exit code, and avoid the "no state found" hint that implies
	// everything is fine. The actual issue list has already been printed
	// above — the error here is just the signal.
	output.PrintWarning(i18n.T("check.issues_found", issues))
	return fmt.Errorf("%d check issue(s) found", issues)
}

// awaitingClone reports whether a git service has no checkout on disk yet.
// Its runtime cannot be detected until `up` clones it, which is the normal
// state of a project that was just handed a raioz.yaml.
func awaitingClone(svc models.Service) bool {
	if svc.Source.Kind != "git" || svc.Source.Path == "" {
		return false
	}
	entries, err := os.ReadDir(svc.Source.Path)
	return err != nil || len(entries) == 0
}
