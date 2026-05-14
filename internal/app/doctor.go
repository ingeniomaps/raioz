package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	goruntime "runtime"
	"strings"

	"raioz/internal/host"
	"raioz/internal/i18n"
	"raioz/internal/output"
	"raioz/internal/runtime"
)

// DoctorCheck represents a single diagnostic check result
type DoctorCheck struct {
	Name    string
	Status  string // "ok", "warning", "error"
	Message string
}

// DoctorUseCase handles the "doctor" use case.
//
// DevBuild is the CLI's IsDevBuild() value, plumbed through at
// construction so the use case can surface a warning without
// importing internal/cli (which would invert the layering). When
// nothing populates it (e.g., test fixtures) the dev-build check
// reports "ok".
type DoctorUseCase struct {
	Out      io.Writer
	DevBuild bool
}

// NewDoctorUseCase creates a new DoctorUseCase. Callers should set
// DevBuild after construction; the legacy zero-arg form keeps existing
// call sites compiling while the migration finishes.
func NewDoctorUseCase() *DoctorUseCase {
	return &DoctorUseCase{Out: os.Stdout}
}

// Execute runs all diagnostic checks
func (uc *DoctorUseCase) Execute(ctx context.Context) error {
	w := uc.Out
	fmt.Fprintf(w, "\n")
	output.PrintSectionHeader(i18n.T("doctor.header"))

	checks := []DoctorCheck{
		uc.checkDocker(ctx),
		uc.checkDockerCompose(ctx),
		uc.checkGit(ctx),
		uc.checkDiskSpace(),
		uc.checkRaiozDir(),
		uc.checkOS(),
		uc.checkCaddy(ctx),
		uc.checkMkcert(ctx),
		uc.checkRuntimes(ctx),
		uc.checkBuildInfo(),
		uc.checkEnvironment(),
	}

	hasError := false
	hasWarning := false

	for _, check := range checks {
		var tag string
		switch check.Status {
		case "ok":
			tag = "\033[32m[ok]\033[0m"
		case "warning":
			tag = "\033[33m[!!]\033[0m"
			hasWarning = true
		case "error":
			tag = "\033[31m[fail]\033[0m"
			hasError = true
		}
		fmt.Fprintf(w, "  %s %-16s %s\n", tag, check.Name, check.Message)
	}

	fmt.Fprintf(w, "\n")

	if hasError {
		output.PrintError(i18n.T("doctor.result_error"))
		return fmt.Errorf("%s", i18n.T("doctor.result_error"))
	}
	if hasWarning {
		output.PrintWarning(i18n.T("doctor.result_warning"))
	} else {
		output.PrintSuccess(i18n.T("doctor.result_ok"))
	}

	return nil
}

func (uc *DoctorUseCase) checkDocker(ctx context.Context) DoctorCheck {
	name := "Docker"

	_, err := exec.LookPath("docker")
	if err != nil {
		return DoctorCheck{Name: name, Status: "error", Message: i18n.T("doctor.docker_not_installed")}
	}

	out, err := exec.CommandContext(ctx, runtime.Binary(), "info", "--format", "{{.ServerVersion}}").Output()
	if err != nil {
		return DoctorCheck{Name: name, Status: "error", Message: i18n.T("doctor.docker_not_running")}
	}

	version := strings.TrimSpace(string(out))
	return DoctorCheck{Name: name, Status: "ok", Message: fmt.Sprintf("v%s", version)}
}

func (uc *DoctorUseCase) checkDockerCompose(ctx context.Context) DoctorCheck {
	name := "Docker Compose"

	out, err := exec.CommandContext(ctx, runtime.Binary(), "compose", "version", "--short").Output()
	if err != nil {
		return DoctorCheck{Name: name, Status: "error", Message: i18n.T("doctor.compose_not_installed")}
	}

	version := strings.TrimSpace(string(out))
	return DoctorCheck{Name: name, Status: "ok", Message: fmt.Sprintf("v%s", version)}
}

func (uc *DoctorUseCase) checkGit(ctx context.Context) DoctorCheck {
	name := "Git"

	out, err := exec.CommandContext(ctx, "git", "--version").Output()
	if err != nil {
		return DoctorCheck{Name: name, Status: "error", Message: i18n.T("doctor.git_not_installed")}
	}

	version := strings.TrimSpace(string(out))
	version = strings.TrimPrefix(version, "git version ")
	return DoctorCheck{Name: name, Status: "ok", Message: fmt.Sprintf("v%s", version)}
}

func (uc *DoctorUseCase) checkDiskSpace() DoctorCheck {
	name := i18n.T("doctor.disk_space")
	free := getFreeDiskSpaceGB()

	if free < 1 {
		return DoctorCheck{Name: name, Status: "error", Message: i18n.T("doctor.disk_critical", free)}
	}
	if free < 5 {
		return DoctorCheck{Name: name, Status: "warning", Message: i18n.T("doctor.disk_low", free)}
	}
	return DoctorCheck{Name: name, Status: "ok", Message: fmt.Sprintf("%.1f GB", free)}
}

// checkBuildInfo surfaces "this is a dev build" as a doctor warning.
// Pairs with the once-per-process stderr notice in
// internal/cli/version.go::MaybePrintDevBuildWarning (ADR-021); the
// CLI side warns at startup, the doctor side puts the same signal in
// the diagnostics report.
func (uc *DoctorUseCase) checkBuildInfo() DoctorCheck {
	if uc.DevBuild {
		return DoctorCheck{
			Name:    "Build info",
			Status:  "warning",
			Message: "DEV BUILD — rebuild with `make build` for reproducible bug reports",
		}
	}
	return DoctorCheck{
		Name:    "Build info",
		Status:  "ok",
		Message: "release build with version metadata",
	}
}

func (uc *DoctorUseCase) checkRaiozDir() DoctorCheck {
	name := i18n.T("doctor.raioz_dir")
	home, err := os.UserHomeDir()
	if err != nil {
		return DoctorCheck{Name: name, Status: "warning", Message: i18n.T("doctor.home_not_found")}
	}

	raiozDir := home + "/.raioz"
	if _, err := os.Stat(raiozDir); os.IsNotExist(err) {
		return DoctorCheck{Name: name, Status: "warning", Message: i18n.T("doctor.raioz_dir_missing")}
	}

	return DoctorCheck{Name: name, Status: "ok", Message: raiozDir}
}

// checkEnvironment surfaces the resolution state of duration-typed env
// vars raioz reads. A typo like `RAIOZ_LAUNCHER_TIMEOUT=60` (missing
// "s") used to fall back to the default silently; the doctor now flags
// malformed values loudly and lists overrides quietly. See ADR-035.
//
// Status:
//   - error    → at least one value is malformed (typo'd unit, etc.)
//   - warning  → none today; reserved for "deprecated env var still set"
//   - ok       → all values either default or valid override
func (uc *DoctorUseCase) checkEnvironment() DoctorCheck {
	name := "Environment"
	statuses := host.KnownDurationEnvs()

	var malformed, overrides []string
	for _, s := range statuses {
		if s.Malformed {
			malformed = append(malformed,
				fmt.Sprintf("%s=%q (using default %s)", s.Name, s.Raw, s.Default))
			continue
		}
		if s.Raw != "" {
			overrides = append(overrides,
				fmt.Sprintf("%s=%s", s.Name, s.Resolved))
		}
	}

	if len(malformed) > 0 {
		return DoctorCheck{
			Name:   name,
			Status: "error",
			Message: strings.Join(malformed, ", ") +
				" — expected Go duration like 60s, 2m, 1h" +
				" (see docs/CONFIG_REFERENCE.md#environment-variables-read-by-raioz)",
		}
	}
	if len(overrides) > 0 {
		return DoctorCheck{
			Name:    name,
			Status:  "ok",
			Message: fmt.Sprintf("%d override(s): %s", len(overrides), strings.Join(overrides, ", ")),
		}
	}
	return DoctorCheck{
		Name:    name,
		Status:  "ok",
		Message: fmt.Sprintf("no overrides (%d duration var(s) at default)", len(statuses)),
	}
}

func (uc *DoctorUseCase) checkOS() DoctorCheck {
	name := i18n.T("doctor.system")
	return DoctorCheck{
		Name:    name,
		Status:  "ok",
		Message: fmt.Sprintf("%s/%s", goruntime.GOOS, goruntime.GOARCH),
	}
}
