package cli

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/spf13/cobra"
)

// Version information (set at build time with -ldflags)
var (
	Version       = "dev"     // Set with -ldflags "-X 'raioz/internal/cli.Version=...'"
	Commit        = "unknown" // Set with -ldflags "-X 'raioz/internal/cli.Commit=...'"
	BuildDate     = "unknown" // Set with -ldflags "-X 'raioz/internal/cli.BuildDate=...'"
	SchemaVersion = "1.0"     // Supported schema version
)

// IsDevBuild reports whether this binary was built without the
// `-ldflags="-X ..."` metadata stamps. Exported so doctor and any
// future report can surface it.
func IsDevBuild() bool { return Version == "dev" }

// devBuildWarningOnce ensures the startup warning prints exactly once
// per process. PersistentPreRun fires on every subcommand, so without
// the once-gate the warning would repeat for compound CLI usage
// (e.g., `raioz proxy status`).
var devBuildWarningOnce sync.Once

// MaybePrintDevBuildWarning writes a one-time stderr warning when the
// binary lacks injected version metadata. ADR-021 documents the
// trade-off. Tests can call it directly; production calls it from
// rootCmd.PersistentPreRun.
func MaybePrintDevBuildWarning() {
	if !IsDevBuild() {
		return
	}
	devBuildWarningOnce.Do(func() {
		fmt.Fprintln(os.Stderr,
			"warning: this raioz binary was built without version "+
				"metadata. Bug reports against a dev build can't be "+
				"traced to a commit. Rebuild with `make build` or "+
				"`go install -ldflags=...` — see CONTRIBUTING.md.")
	})
}

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
