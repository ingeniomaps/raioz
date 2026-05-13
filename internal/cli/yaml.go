package cli

import (
	"github.com/spf13/cobra"
)

// yamlCmd groups subcommands that inspect a raioz.yaml file. Today it
// only hosts `lint`; the parent exists to leave room for `format`,
// `explain`, etc. without polluting the top-level help.
var yamlCmd = &cobra.Command{
	Use:   "yaml",
	Short: "Inspect or manipulate raioz.yaml",
	Long: "Subcommands for raioz.yaml — currently only `lint`, which " +
		"checks which schema fields your file uses against the raioz " +
		"version that introduced each one.",
}
