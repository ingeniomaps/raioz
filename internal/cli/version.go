package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// Version information (set at build time with -ldflags)
var (
	Version       = "dev"     // Set with -ldflags "-X 'raioz/internal/cli.Version=...'"
	Commit        = "unknown" // Set with -ldflags "-X 'raioz/internal/cli.Commit=...'"
	BuildDate     = "unknown" // Set with -ldflags "-X 'raioz/internal/cli.BuildDate=...'"
	SchemaVersion = "1.0"     // Supported schema version
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  "Show version information for raioz.",
	RunE: func(cmd *cobra.Command, args []string) error {
		printVersion(cmd.OutOrStdout())
		return nil
	},
}

func printVersion(w io.Writer) {
	fmt.Fprintf(w, "raioz version %s\n", Version)
	fmt.Fprintf(w, "Schema version: %s\n", SchemaVersion)

	if Commit != "unknown" && Commit != "" {
		fmt.Fprintf(w, "Commit: %s\n", Commit)
	}

	if BuildDate != "unknown" && BuildDate != "" {
		fmt.Fprintf(w, "Build date: %s\n", BuildDate)
	}

	if Version == "dev" {
		fmt.Fprintln(w, "\n(Development build)")
	}
}
