package orchestrate

import (
	"os"

	"raioz/internal/domain/interfaces"
	"raioz/internal/env"
	"raioz/internal/i18n"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// hostProcessEnv builds the environment of a host service: the inherited
// one, then the files declared in `env:`, then the vars raioz computes
// (discovery, PORT, inline `env:` values). Later entries win, the same
// order DockerfileRunner gets from `--env-file` followed by `-e`.
//
// A declared file that is missing or unparsable is skipped with a warning
// rather than failing the start: the service may not need it to boot, and
// the docker runners are the ones that cannot run without it.
func hostProcessEnv(svc interfaces.ServiceContext) []string {
	out := os.Environ()
	for _, f := range svc.EnvFilePaths {
		vars, err := env.LoadFiles([]string{f})
		if err != nil {
			logging.Warn("Skipping env file for host service",
				"service", svc.Name, "file", f, "error", err.Error())
			output.PrintWarning(i18n.T("up.host_env_file_skipped", svc.Name, f))
			continue
		}
		for k, v := range vars {
			out = append(out, k+"="+v)
		}
	}
	for k, v := range svc.EnvVars {
		out = append(out, k+"="+v)
	}
	return out
}
