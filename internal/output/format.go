package output

import (
	"fmt"
	"strings"
	"time"

	"raioz/internal/i18n"
)

// ANSI color codes
const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	cyan   = "\033[36m"
	white  = "\033[37m"
)

// PrintSuccess prints a success message with green checkmark.
func PrintSuccess(message string) {
	fmt.Printf("  %s[ok]%s %s\n", green, reset, message)
}

// PrintWarning prints a warning message in yellow.
func PrintWarning(message string) {
	fmt.Printf("  %s[!!]%s %s\n", yellow, reset, message)
}

// PrintError prints an error message in red.
func PrintError(message string) {
	fmt.Printf("  %s[error]%s %s\n", red, reset, message)
}

// PrintInfo prints an informational message.
func PrintInfo(message string) {
	fmt.Printf("  %s%s%s\n", dim, message, reset)
}

// FormatDuration formats a duration in human-readable format.
func FormatDuration(d time.Duration) string {
	seconds := int(d.Seconds())
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := seconds / 60
	remainingSeconds := seconds % 60
	if minutes < 60 {
		return fmt.Sprintf("%dm %ds", minutes, remainingSeconds)
	}
	hours := minutes / 60
	remainingMinutes := minutes % 60
	return fmt.Sprintf("%dh %dm", hours, remainingMinutes)
}

// PrintSummary prints the final summary after raioz up completes.
func PrintSummary(services []string, infra []string, duration time.Duration) {
	fmt.Println()
	PrintSectionHeader(i18n.T("output.environment_ready"))

	if len(services) > 0 {
		PrintSubsection(i18n.T("output.services_count", len(services)))
		PrintList(services, 2)
	}

	if len(infra) > 0 {
		PrintSubsection(i18n.T("output.infra_count", len(infra)))
		PrintList(infra, 2)
	}

	fmt.Println()
	fmt.Printf("  %s%s:%s %s\n", dim, i18n.T("output.time_elapsed"), reset, FormatDuration(duration))
	fmt.Println()
}

// PrintServiceCloned prints a message when a service is cloned.
func PrintServiceCloned(serviceName string) {
	PrintSuccess(i18n.T("output.cloned", serviceName))
}

// PrintInfraStarted prints a message when infrastructure is started.
func PrintInfraStarted(infraName string) {
	PrintSuccess(infraName)
}

// PrintWorkspaceCreated is a no-op in the new architecture.
// Workspaces are implicit; no user-facing message needed.
func PrintWorkspaceCreated() {
	// Intentionally silent — workspace creation is an internal detail
}

// PrintProjectStarted prints a success message when project is started.
func PrintProjectStarted(projectName string) {
	PrintSuccess(i18n.T("output.project_started", projectName))
}

// PrintProgress prints a progress step message.
func PrintProgress(message string) {
	fmt.Printf("  %s--%s %s\n", cyan, reset, message)
}

// PrintProgressStep prints a numbered progress step.
func PrintProgressStep(step int, total int, message string) {
	fmt.Printf("  %s[%d/%d]%s %s\n", cyan, step, total, reset, message)
}

// PrintProgressDone prints completion of a progress step.
func PrintProgressDone(message string) {
	fmt.Printf("  %s[ok]%s %s\n", green, reset, message)
}

// PrintProgressError prints failure of a progress step.
func PrintProgressError(message string) {
	fmt.Printf("  %s[fail]%s %s\n", red, reset, message)
}

// PrintSectionHeader prints a section header with a clean separator.
func PrintSectionHeader(title string) {
	fmt.Println()
	line := strings.Repeat("-", 50)
	fmt.Printf("  %s%s%s\n", dim, line, reset)
	fmt.Printf("  %s%s%s\n", bold, strings.ToUpper(title), reset)
	fmt.Printf("  %s%s%s\n", dim, line, reset)
}

// PrintSubsection prints a subsection title.
func PrintSubsection(title string) {
	fmt.Printf("\n  %s%s%s\n", bold, title, reset)
}

// PrintList prints a formatted list.
func PrintList(items []string, indent int) {
	indentStr := strings.Repeat("  ", indent)
	for _, item := range items {
		fmt.Printf("%s- %s\n", indentStr, item)
	}
}

// PrintKeyValue prints a key-value pair.
func PrintKeyValue(key, value string) {
	fmt.Printf("  %s%s:%s %s\n", dim, key, reset, value)
}

// PrintPrompt prints a prompt for user input.
func PrintPrompt(message string) {
	fmt.Print(message)
}
