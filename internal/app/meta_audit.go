package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"raioz/internal/audit"
	"raioz/internal/config"
)

// metaAuditTarget picks the identifier that lifecycle audit events should
// carry as the "project" field for a meta run. Workspace name wins when
// declared; falls back to the meta basedir so audit log greps still find
// the run.
func metaAuditTarget(cfg *config.MetaConfig) string {
	if cfg.Workspace != "" {
		return cfg.Workspace
	}
	if cfg.BaseDir != "" {
		return cfg.BaseDir
	}
	return "meta"
}

// logMetaLifecycleComplete records the end of a meta_up/meta_down/meta_status
// run. status mirrors lifecycle convention: "success" when no sub failed
// hard, "failure" otherwise. Errors from the audit subsystem are swallowed
// to keep the meta path tolerant of a missing state dir.
func logMetaLifecycleComplete(
	ctx context.Context, operation string,
	cfg *config.MetaConfig, results MetaSummaryList, start time.Time,
) {
	status := "success"
	var summaryErr error
	if results.HasFailures() {
		status = "failure"
		summaryErr = fmt.Errorf("one or more sub-projects failed")
	}
	_ = audit.LogLifecycleComplete(
		ctx, operation, metaAuditTarget(cfg), cfg.Workspace,
		status, time.Since(start), summaryErr,
	)
}

// auditMetaTargets runs ADR-036 hygiene gates against every project
// in cfg (router + projects[]) before spawn. First failure aborts the
// scan and surfaces the path that tripped so the operator can fix
// the upstream yaml.
func auditMetaTargets(cfg *config.MetaConfig) error {
	if cfg.Router != nil {
		if err := auditMetaProject(*cfg.Router); err != nil {
			return err
		}
	}
	for _, p := range cfg.Projects {
		if err := auditMetaProject(p); err != nil {
			return err
		}
	}
	return nil
}

// auditMetaProject delegates a single meta project to AuditYAMLStrict.
// Skips entries whose Path doesn't exist or doesn't contain a
// raioz.yaml — those are pre-existing load errors and are surfaced by
// the regular spawn path with their normal message.
func auditMetaProject(p config.MetaProject) error {
	yamlPath := metaProjectYAMLPath(p)
	if yamlPath == "" {
		return nil
	}
	return config.AuditYAMLStrict(yamlPath)
}

// metaProjectYAMLPath picks the yaml file inside a meta sub-project
// directory. raioz convention is raioz.yaml; raioz.yml is accepted as
// a fallback. Returns "" when neither exists.
func metaProjectYAMLPath(p config.MetaProject) string {
	for _, candidate := range []string{"raioz.yaml", "raioz.yml"} {
		full := filepath.Join(p.Path, candidate)
		if _, err := os.Stat(full); err == nil {
			return full
		}
	}
	return ""
}

// metaSubFailureDetails packs the audit Details for a per-sub failure
// recorded under the meta best-effort path (down/status always; up only
// for optional subs). Keeping the shape consistent with lifecycleDetails
// lets downstream readers reuse the same filters.
func metaSubFailureDetails(
	subCmd string, p config.MetaProject, err error, optional bool,
) map[string]any {
	d := map[string]any{
		"operation":   "meta_sub_" + subCmd,
		"phase":       "complete",
		"status":      "failure",
		"sub_project": p.Name,
		"sub_path":    p.Path,
		"best_effort": true,
		"optional":    optional,
	}
	if err != nil {
		d["error"] = err.Error()
	}
	return d
}
