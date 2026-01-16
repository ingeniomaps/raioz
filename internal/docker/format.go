package docker

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"
)

// ansiEscapeRegex matches ANSI escape sequences
var ansiEscapeRegex = regexp.MustCompile(`\033\[[0-9;]*m`)

// stripANSI removes ANSI escape sequences from a string
func stripANSI(s string) string {
	return ansiEscapeRegex.ReplaceAllString(s, "")
}

// padString pads a string to a specific width, accounting for ANSI codes
func padString(s string, width int) string {
	stripped := stripANSI(s)
	actualWidth := len(stripped)
	if actualWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-actualWidth)
}

// Color codes for terminal output
const (
	ColorReset  = "\033[0m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorRed    = "\033[31m"
	ColorBlue   = "\033[34m"
	ColorGray   = "\033[90m"
)

// ColorizeStatus returns colored status string
func ColorizeStatus(status string) string {
	switch status {
	case "running":
		return ColorGreen + status + ColorReset
	case "stopped":
		return ColorRed + status + ColorReset
	default:
		return status
	}
}

// ColorizeHealth returns colored health string
func ColorizeHealth(health string) string {
	switch health {
	case "healthy":
		return ColorGreen + health + ColorReset
	case "unhealthy":
		return ColorRed + health + ColorReset
	case "starting":
		return ColorYellow + health + ColorReset
	case "none":
		return ColorGray + "n/a" + ColorReset
	default:
		return health
	}
}

// FormatStatusTable formats service info as a table
func FormatStatusTable(services map[string]*ServiceInfo, jsonOutput bool) error {
	if jsonOutput {
		// JSON output will be handled by cmd/status.go
		return nil
	}

	// Use tabwriter with left alignment and proper padding
	// minwidth=0, tabwidth=8, padding=2, padchar=' ', flags=0 (left align)
	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	defer w.Flush()

	// Header with better formatting - pad each column to ensure alignment
	// Calculate max widths for each column to ensure proper alignment
	// These widths account for ANSI color codes by using fixed widths
	// The padding function strips ANSI codes before calculating width
	maxWidths := map[string]int{
		"NAME":    18,
		"STATUS":  14,
		"HEALTH":  12,
		"UPTIME":  10,
		"CPU":     10,
		"MEMORY":  22,
		"VERSION": 14,
		"UPDATED": 12,
	}

	header := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t",
		padString("NAME", maxWidths["NAME"]),
		padString("STATUS", maxWidths["STATUS"]),
		padString("HEALTH", maxWidths["HEALTH"]),
		padString("UPTIME", maxWidths["UPTIME"]),
		padString("CPU", maxWidths["CPU"]),
		padString("MEMORY", maxWidths["MEMORY"]),
		padString("VERSION", maxWidths["VERSION"]),
		padString("UPDATED", maxWidths["UPDATED"]),
	)
	fmt.Fprintln(w, header)

	// Separator line - pad each separator to match header widths
	separator := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t",
		strings.Repeat("─", maxWidths["NAME"]),
		strings.Repeat("─", maxWidths["STATUS"]),
		strings.Repeat("─", maxWidths["HEALTH"]),
		strings.Repeat("─", maxWidths["UPTIME"]),
		strings.Repeat("─", maxWidths["CPU"]),
		strings.Repeat("─", maxWidths["MEMORY"]),
		strings.Repeat("─", maxWidths["VERSION"]),
		strings.Repeat("─", maxWidths["UPDATED"]),
	)
	fmt.Fprintln(w, separator)

	// Rows
	for _, info := range services {
		status := ColorizeStatus(info.Status)
		health := ColorizeHealth(info.Health)

		// Default values
		uptime := "-"
		if info.Uptime != "" {
			uptime = info.Uptime
		}

		cpu := "-"
		if info.CPU != "" {
			cpu = info.CPU
		}

		memory := "-"
		if info.Memory != "" {
			memory = info.Memory
		}

		version := "-"
		if info.Version != "" {
			version = info.Version
		}

		updated := "-"
		if info.LastUpdated != "" {
			// Format commit date to shorter format if it's a git date
			updated = formatDate(info.LastUpdated)
		}

		// Add link indicator if service is linked
		nameDisplay := info.Name
		if info.Linked {
			nameDisplay = fmt.Sprintf("%s (→ %s)", info.Name, info.LinkTarget)
		}

		// Pad each column to ensure proper alignment with ANSI codes
		fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t\n",
			padString(nameDisplay, maxWidths["NAME"]),
			padString(status, maxWidths["STATUS"]),
			padString(health, maxWidths["HEALTH"]),
			padString(uptime, maxWidths["UPTIME"]),
			padString(cpu, maxWidths["CPU"]),
			padString(memory, maxWidths["MEMORY"]),
			padString(version, maxWidths["VERSION"]),
			padString(updated, maxWidths["UPDATED"]),
		)
	}

	return nil
}

// formatDate formats a date string to a shorter format
func formatDate(dateStr string) string {
	// Try parsing as git commit date format (2006-01-02 15:04:05 -0700)
	gitLayout := "2006-01-02 15:04:05 -0700"
	if t, err := time.Parse(gitLayout, dateStr); err == nil {
		return t.Format("2006-01-02 15:04")
	}

	// Try parsing as container start date format
	containerLayout := "2006-01-02 15:04:05"
	if t, err := time.Parse(containerLayout, dateStr); err == nil {
		return t.Format("2006-01-02 15:04")
	}

	// If parsing fails, return first 16 chars
	if len(dateStr) > 16 {
		return dateStr[:16]
	}
	return dateStr
}
