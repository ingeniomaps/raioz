package cli

import (
	"fmt"
	"os"

	"raioz/internal/config"

	"github.com/spf13/cobra"
)

var yamlLintFile string

var yamlLintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Report the raioz version each used field requires",
	Long: "Walks the raioz.yaml in the current directory (or --file) and " +
		"reports, for each populated field, the raioz version that " +
		"introduced it. Fields without a since marker (rare; tracked by " +
		"check-since) are skipped silently.",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := yamlLintFile
		if path == "" {
			path = "raioz.yaml"
		}
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		findings, cfg, err := config.LintConfigPath(path)
		if err != nil {
			return err
		}
		emitLintReport(cmd, path, cfg, findings)
		return nil
	},
}

func init() {
	yamlCmd.AddCommand(yamlLintCmd)
	yamlLintCmd.Flags().StringVar(&yamlLintFile, "file", "",
		"Path to raioz.yaml (default: ./raioz.yaml)")
}

// emitLintReport prints one line per finding, prefixed with severity, so
// the output is human-skimmable and grep-friendly. The "no version
// declared" warning is collapsed into a single banner instead of being
// repeated per field — otherwise a 30-field config produces 30 copies of
// the same suggestion.
func emitLintReport(
	cmd *cobra.Command, path string, cfg *config.RaiozConfig, findings []config.LintFinding,
) {
	out := cmd.OutOrStdout()
	if cfg == nil {
		return
	}
	declared := cfg.Version
	fmt.Fprintf(out, "raioz yaml lint: %s\n", path)
	if declared == "" {
		fmt.Fprintf(out, "  declared version: (none) — "+
			"add `version: %q` to your raioz.yaml to lock the schema\n",
			config.CurrentSchemaVersion)
	} else {
		fmt.Fprintf(out, "  declared version: %s\n", declared)
	}
	fmt.Fprintf(out, "  fields in use:    %d\n\n", len(findings))

	for _, f := range findings {
		switch f.Severity {
		case "warn":
			fmt.Fprintf(out, "  [warn] %s (since %s)\n", f.Path, f.Since)
		case "info":
			fmt.Fprintf(out, "  [info] %s\n", f.Message)
		default:
			fmt.Fprintf(out, "  [ok]   %s (since %s)\n", f.Path, f.Since)
		}
	}
}
