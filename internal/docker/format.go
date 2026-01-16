package docker

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"
)

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

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.AlignRight)
	defer w.Flush()

	// Header with better formatting
	fmt.Fprintln(
		w,
		"NAME\tSTATUS\tHEALTH\tUPTIME\tCPU\tMEMORY\tVERSION\tUPDATED",
	)
	// Separator line
	separator := "────\t──────\t──────\t──────\t───\t──────\t───────\t───────"
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

		fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			nameDisplay,
			status,
			health,
			uptime,
			cpu,
			memory,
			version,
			updated,
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
