package output

import (
	"fmt"
	"strings"
	"time"
)

// PrintSuccess prints a success message with checkmark
func PrintSuccess(message string) {
	fmt.Printf("✔ %s\n", message)
}

// PrintWarning prints a warning message with warning emoji
func PrintWarning(message string) {
	fmt.Printf("⚠️  %s\n", message)
}

// PrintError prints an error message with error emoji
func PrintError(message string) {
	fmt.Printf("🔴 %s\n", message)
}

// PrintInfo prints an info message
func PrintInfo(message string) {
	fmt.Printf("ℹ️  %s\n", message)
}

// FormatDuration formats a duration in human-readable format
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

// PrintSummary prints a summary of services started
func PrintSummary(services []string, infra []string, duration time.Duration) {
	fmt.Println()
	PrintSectionHeader("Environment Ready")

	if len(services) > 0 {
		PrintSubsection(fmt.Sprintf("Services (%d)", len(services)))
		PrintList(services, 1)
	}

	if len(infra) > 0 {
		PrintSubsection(fmt.Sprintf("Infrastructure (%d)", len(infra)))
		PrintList(infra, 1)
	}

	fmt.Println()
	PrintKeyValue("Time elapsed", FormatDuration(duration))
	fmt.Println()
}

// PrintServiceCloned prints a message when a service is cloned
func PrintServiceCloned(serviceName string) {
	PrintSuccess(fmt.Sprintf("%s clonado", serviceName))
}

// PrintServiceUsingImage prints a message when a service uses an image
func PrintServiceUsingImage(serviceName string) {
	PrintSuccess(fmt.Sprintf("%s usando imagen", serviceName))
}

// PrintInfraStarted prints a message when infrastructure is started
func PrintInfraStarted(infraName string) {
	PrintSuccess(fmt.Sprintf("%s levantado", infraName))
}

// PrintWorkspaceCreated prints a message when workspace is created
func PrintWorkspaceCreated() {
	PrintSuccess("Workspace creado")
}

// PrintGeneratingCompose prints a message when generating compose
func PrintGeneratingCompose() {
	PrintSuccess("generating docker-compose.generated.yml")
}

// PrintStartingServices prints a message when starting services
func PrintStartingServices() {
	PrintSuccess("starting services...")
}

// PrintProjectStarted prints a success message when project is started
func PrintProjectStarted(projectName string) {
	fmt.Printf("✔ Project '%s' started successfully\n", projectName)
}

// FormatConfigChanges formats configuration changes for display
func FormatConfigChanges(changes []string) string {
	if len(changes) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, change := range changes {
		sb.WriteString(fmt.Sprintf("  %s\n", change))
	}
	return sb.String()
}

// PrintProgress prints a progress message for long-running operations
func PrintProgress(message string) {
	fmt.Printf("⏳ %s...\n", message)
}

// PrintProgressStep prints a progress step message (for multi-step operations)
func PrintProgressStep(step int, total int, message string) {
	fmt.Printf("⏳ [%d/%d] %s...\n", step, total, message)
}

// PrintProgressDone prints a completion message for a progress operation
func PrintProgressDone(message string) {
	fmt.Printf("✔ %s\n", message)
}

// PrintProgressError prints an error message for a failed progress operation
func PrintProgressError(message string) {
	fmt.Printf("✗ %s\n", message)
}

// PrintSectionHeader prints a section header with visual separator
func PrintSectionHeader(title string) {
	fmt.Println()
	fmt.Printf("━━━ %s ━━━\n", strings.ToUpper(title))
	fmt.Println()
}

// PrintSubsection prints a subsection header
func PrintSubsection(title string) {
	fmt.Printf("\n▸ %s\n", title)
}

// PrintList prints a formatted list with bullets
func PrintList(items []string, indent int) {
	indentStr := strings.Repeat("  ", indent)
	for _, item := range items {
		fmt.Printf("%s• %s\n", indentStr, item)
	}
}

// PrintKeyValue prints a key-value pair in a formatted way
func PrintKeyValue(key, value string) {
	fmt.Printf("  %s: %s\n", key, value)
}

// PrintTableHeader prints a table header with separator
func PrintTableHeader(headers ...string) {
	// Print headers
	for i, header := range headers {
		if i > 0 {
			fmt.Print("\t")
		}
		fmt.Print(header)
	}
	fmt.Println()

	// Print separator
	separator := strings.Repeat("─", 60)
	fmt.Println(separator)
}

// PrintTableRow prints a table row
func PrintTableRow(values ...string) {
	for i, value := range values {
		if i > 0 {
			fmt.Print("\t")
		}
		fmt.Print(value)
	}
	fmt.Println()
}

// PrintEmptyState prints a message when there's no data to show
func PrintEmptyState(message string) {
	fmt.Printf("  (no %s)\n", message)
}
