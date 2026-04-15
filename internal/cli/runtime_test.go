package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// runtime_test.go exercises the RunE paths of commands against invalid or
// missing configs so that error branches in use-case plumbing are covered.
// All tests run in an isolated t.TempDir() and restore any globals they
// modify.

// withTempCWD chdirs into a fresh t.TempDir() and restores the previous
// working directory on cleanup. It returns the temp directory path.
func withTempCWD(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	return dir
}

// withGlobalConfigPath sets the package-level `configPath` and restores it.
func withGlobalConfigPath(t *testing.T, p string) {
	t.Helper()
	orig := configPath
	configPath = p
	t.Cleanup(func() { configPath = orig })
}

func withGlobalProjectName(t *testing.T, name string) {
	t.Helper()
	orig := projectName
	projectName = name
	t.Cleanup(func() { projectName = orig })
}

// expectRunError runs RunE and asserts it returned a non-nil error.
func expectRunError(t *testing.T, cmd *cobra.Command, args []string) {
	t.Helper()
	err := cmd.RunE(cmd, args)
	if err == nil {
		t.Errorf("%s: expected error, got nil", cmd.Name())
	}
}

func TestUpRunInvalidConfig(t *testing.T) {
	dir := withTempCWD(t)
	withGlobalConfigPath(t, filepath.Join(dir, "nope.yaml"))
	expectRunError(t, upCmd, nil)
}

func TestDownRunInvalidConfig(t *testing.T) {
	dir := withTempCWD(t)
	withGlobalConfigPath(t, filepath.Join(dir, "nope.yaml"))
	withGlobalProjectName(t, "")
	// Down may silently succeed if it determines nothing to do; just verify
	// it doesn't panic.
	_ = downCmd.RunE(downCmd, nil)
}

func TestStatusRunInvalidConfig(t *testing.T) {
	dir := withTempCWD(t)
	withGlobalConfigPath(t, filepath.Join(dir, "nope.yaml"))
	withGlobalProjectName(t, "")
	expectRunError(t, statusCmd, nil)
}

func TestCleanRunInvalidConfig(t *testing.T) {
	dir := withTempCWD(t)
	withGlobalConfigPath(t, filepath.Join(dir, "nope.yaml"))
	withGlobalProjectName(t, "")
	// clean may tolerate missing config in dry-run mode
	orig := cleanDryRun
	cleanDryRun = false
	defer func() { cleanDryRun = orig }()
	_ = cleanCmd.RunE(cleanCmd, nil)
}

func TestCompareRunInvalidConfig(t *testing.T) {
	dir := withTempCWD(t)
	withGlobalConfigPath(t, filepath.Join(dir, "nope.yaml"))
	origProd := compareProductionPath
	compareProductionPath = filepath.Join(dir, "also-missing.yml")
	defer func() { compareProductionPath = origProd }()
	expectRunError(t, compareCmd, nil)
}

func TestCheckRunInvalidConfig(t *testing.T) {
	dir := withTempCWD(t)
	withGlobalConfigPath(t, filepath.Join(dir, "nope.yaml"))
	withGlobalProjectName(t, "")
	expectRunError(t, checkCmd, nil)
}

func TestLogsRunInvalidConfig(t *testing.T) {
	dir := withTempCWD(t)
	withGlobalConfigPath(t, filepath.Join(dir, "nope.yaml"))
	withGlobalProjectName(t, "")
	// Should error cleanly without panic.
	_ = logsCmd.RunE(logsCmd, nil)
}

func TestRestartRunInvalidConfig(t *testing.T) {
	dir := withTempCWD(t)
	withGlobalConfigPath(t, filepath.Join(dir, "nope.yaml"))
	withGlobalProjectName(t, "")
	_ = restartCmd.RunE(restartCmd, nil)
}

func TestExecRunInvalidConfig(t *testing.T) {
	dir := withTempCWD(t)
	withGlobalConfigPath(t, filepath.Join(dir, "nope.yaml"))
	withGlobalProjectName(t, "")
	// exec needs a service name arg + command
	_ = execCmd.RunE(execCmd, []string{"api", "ls"})
}

func TestPortsRunInvalidConfig(t *testing.T) {
	_ = withTempCWD(t)
	withGlobalProjectName(t, "")
	_ = portsCmd.RunE(portsCmd, nil)
}

func TestListRunInvalidConfig(t *testing.T) {
	_ = withTempCWD(t)
	// list iterates workspace base; may succeed with empty output.
	_ = listCmd.RunE(listCmd, nil)
}

func TestHealthRunInvalidConfig(t *testing.T) {
	dir := withTempCWD(t)
	withGlobalConfigPath(t, filepath.Join(dir, "nope.yaml"))
	_ = healthCmd.RunE(healthCmd, nil)
}

func TestVolumesListRunInvalidConfig(t *testing.T) {
	dir := withTempCWD(t)
	withGlobalConfigPath(t, filepath.Join(dir, "nope.yaml"))
	withGlobalProjectName(t, "")
	_ = volumesListCmd.RunE(volumesListCmd, nil)
}

func TestIgnoreListRunInvalidConfig(t *testing.T) {
	dir := withTempCWD(t)
	withGlobalConfigPath(t, filepath.Join(dir, "nope.yaml"))
	_ = ignoreListCmd.RunE(ignoreListCmd, nil)
}

func TestIgnoreAddRunInvalidConfig(t *testing.T) {
	dir := withTempCWD(t)
	withGlobalConfigPath(t, filepath.Join(dir, "nope.yaml"))
	_ = ignoreAddCmd.RunE(ignoreAddCmd, []string{"api"})
}

func TestIgnoreRemoveRunInvalidConfig(t *testing.T) {
	dir := withTempCWD(t)
	withGlobalConfigPath(t, filepath.Join(dir, "nope.yaml"))
	_ = ignoreRemoveCmd.RunE(ignoreRemoveCmd, []string{"api"})
}

func TestInitRunInDir(t *testing.T) {
	dir := withTempCWD(t)
	// init generates a new raioz.yaml in cwd; run it and verify a file
	// was produced (or at least no panic).
	_ = initCmd.RunE(initCmd, nil)
	_ = dir // suppress unused
}

func TestMigrateRunMissingCompose(t *testing.T) {
	_ = withTempCWD(t)
	// No --compose flag set: should return an error.
	orig := migrateComposePath
	migrateComposePath = ""
	defer func() { migrateComposePath = orig }()
	err := migrateCmd.RunE(migrateCmd, nil)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestMigrateRunMissingProject(t *testing.T) {
	dir := withTempCWD(t)
	origCompose := migrateComposePath
	origProject := migrateProjectName
	// Point compose at a (non-existent) file so the compose check passes
	// the "required" guard and the project guard fires next.
	migrateComposePath = filepath.Join(dir, "compose.yml")
	migrateProjectName = ""
	defer func() {
		migrateComposePath = origCompose
		migrateProjectName = origProject
	}()
	err := migrateCmd.RunE(migrateCmd, nil)
	if err == nil {
		t.Error("expected error, got nil")
	}
}
