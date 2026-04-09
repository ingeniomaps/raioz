package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"raioz/internal/i18n"
	"raioz/internal/output"
)

// DoctorCheck represents a single diagnostic check result
type DoctorCheck struct {
	Name    string
	Status  string // "ok", "warning", "error"
	Message string
}

// DoctorUseCase handles the "doctor" use case
type DoctorUseCase struct {
	Out io.Writer
}

// NewDoctorUseCase creates a new DoctorUseCase
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

	out, err := exec.CommandContext(ctx, "docker", "info", "--format", "{{.ServerVersion}}").Output()
	if err != nil {
		return DoctorCheck{Name: name, Status: "error", Message: i18n.T("doctor.docker_not_running")}
	}

	version := strings.TrimSpace(string(out))
	return DoctorCheck{Name: name, Status: "ok", Message: fmt.Sprintf("v%s", version)}
}

func (uc *DoctorUseCase) checkDockerCompose(ctx context.Context) DoctorCheck {
	name := "Docker Compose"

	out, err := exec.CommandContext(ctx, "docker", "compose", "version", "--short").Output()
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

func (uc *DoctorUseCase) checkOS() DoctorCheck {
	name := i18n.T("doctor.system")
	return DoctorCheck{
		Name:    name,
		Status:  "ok",
		Message: fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}
