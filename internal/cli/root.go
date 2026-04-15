package cli

import (
	"fmt"
	"os"

	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/logging"

	"github.com/spf13/cobra"
)

var (
	logLevel string
	logJSON  bool
	langFlag string
)

var rootCmd = &cobra.Command{
	Use:          "raioz",
	Short:        "Raioz local microservices orchestrator",
	SilenceUsage: true, // Don't show usage/help on execution errors
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// Format error with context if it's a RaiozError
		fmt.Print(errors.FormatError(err))
		os.Exit(1)
	}
}

func init() {
	// Detect --lang flag early from os.Args so i18n is initialized
	// with the right language before Cobra renders help text.
	langOverride := detectLangFlag()

	// Initialize i18n before anything else
	i18n.Init(langOverride)

	// Initialize logging from environment or use defaults
	logging.InitFromEnv()

	// Add global flags
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "Set log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVar(&logJSON, "log-json", false, "Output logs in JSON format")
	rootCmd.PersistentFlags().StringVar(&langFlag, "lang", "", "Override display language (en, es)")

	// Hook to update logging and language when flags are parsed
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if logLevel != "" {
			logging.SetLevel(logging.ParseLogLevel(logLevel))
		}
		if logJSON {
			logging.SetJSONFormat(true)
		}
		if langFlag != "" {
			if err := i18n.SetLang(langFlag); err != nil {
				logging.Warn("Failed to set language, falling back to default",
					"lang", langFlag, "error", err)
			}
		}
	}

	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(portsCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(ciCmd)
	rootCmd.AddCommand(compareCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(ignoreCmd)
	rootCmd.AddCommand(langCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(volumesCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(proxyCmd)
	rootCmd.AddCommand(devCmd)
	rootCmd.AddCommand(graphCmd)
	rootCmd.AddCommand(snapshotCmd)
	rootCmd.AddCommand(tunnelCmd)
	rootCmd.AddCommand(dashboardCmd)
	rootCmd.AddCommand(envCmd)
	rootCmd.AddCommand(cloneCmd)
	rootCmd.AddCommand(hostsCmd)
}

// detectLangFlag scans os.Args for --lang <value> or --lang=<value> before
// Cobra parses flags, so i18n can be initialized with the right language.
func detectLangFlag() string {
	for i, arg := range os.Args {
		if arg == "--lang" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
		if len(arg) > 7 && arg[:7] == "--lang=" {
			return arg[7:]
		}
	}
	return ""
}
