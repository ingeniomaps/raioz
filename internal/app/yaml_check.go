package app

import (
	"fmt"

	"raioz/internal/app/upcase"
	"raioz/internal/config"
	"raioz/internal/domain/models"
	"raioz/internal/output"
)

func CheckYAML(proj *YAMLProject) error {
	fmt.Println()
	output.PrintSectionHeader("Config check: " + proj.ProjectName)

	issues := 0

	// Check service paths exist (honoring yaml `command:`/`compose:` overrides).
	for name, svc := range proj.Deps.Services {
		result := config.ResolveServiceDetection(svc, svc.Source.Path)
		if result.Runtime == models.RuntimeUnknown {
			if svc.Source.Path != "" {
				output.PrintWarning(fmt.Sprintf("%s: no runtime detected at %s", name, svc.Source.Path))
			} else {
				output.PrintWarning(fmt.Sprintf("%s: no runtime declared (command/compose/path)", name))
			}
			issues++
		} else {
			output.PrintSuccess(fmt.Sprintf("%s: %s", name, result.Runtime))
		}
	}

	// Check dependency images
	for name, entry := range proj.Deps.Infra {
		if entry.Inline != nil && entry.Inline.Image != "" {
			output.PrintSuccess(fmt.Sprintf("%s: %s", name, entry.Inline.Image))
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
				output.PrintError(fmt.Sprintf("%s depends on '%s' which is not defined", name, dep))
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
		output.PrintSuccess("All checks passed")
		return nil
	}
	// Issues found: return a sentinel error so the CLI wrapper (cli/check.go)
	// can skip the misleading "Configuration is valid" banner, surface a
	// non-zero exit code, and avoid the "no state found" hint that implies
	// everything is fine. The actual issue list has already been printed
	// above — the error here is just the signal.
	output.PrintWarning(fmt.Sprintf("%d issue(s) found", issues))
	return fmt.Errorf("%d check issue(s) found", issues)
}
