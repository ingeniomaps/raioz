package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version information (set at build time with -ldflags)
var (
	Version   = "dev"      // Set with -ldflags "-X 'raioz/cmd.Version=...'"
	Commit    = "unknown"  // Set with -ldflags "-X 'raioz/cmd.Commit=...'"
	BuildDate = "unknown"  // Set with -ldflags "-X 'raioz/cmd.BuildDate=...'"
	SchemaVersion = "1.0"  // Supported schema version
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long: `Show version information for raioz including:
- Binary version
- Git commit (if available)
- Build date
- Supported schema version`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("raioz version %s\n", Version)
		fmt.Printf("Schema version: %s\n", SchemaVersion)

		if Commit != "unknown" && Commit != "" {
			fmt.Printf("Commit: %s\n", Commit)
		}

		if BuildDate != "unknown" && BuildDate != "" {
			fmt.Printf("Build date: %s\n", BuildDate)
		}

		// Show if it's a development build
		if Version == "dev" {
			fmt.Println("\n(Development build)")
		}

		return nil
	},
}
