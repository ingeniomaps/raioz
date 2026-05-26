package upcase

import (
	"path/filepath"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
)

// applyServiceEnv passes a service's own `env:` to the runner: inline vars →
// EnvVars (-e), files → EnvFilePaths (--env-file). Mirrors the deps path in
// processOrchestration. Without it a service's `env:` is silently dropped
// (deps already get it). Relative file paths are anchored to the raioz.yaml
// dir, not the raioz process cwd, since the runner's `docker run` inherits
// the latter.
func applyServiceEnv(svcCtx *interfaces.ServiceContext, env *models.EnvValue, projectDir string) {
	if env == nil {
		return
	}
	for k, v := range env.GetVariables() {
		svcCtx.EnvVars[k] = v
	}
	for _, f := range env.GetFilePaths() {
		if f == "" {
			continue
		}
		if !filepath.IsAbs(f) {
			f = filepath.Join(projectDir, f)
		}
		svcCtx.EnvFilePaths = append(svcCtx.EnvFilePaths, f)
	}
}
