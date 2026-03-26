package cmd

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
	SilenceUsage: true, // Don't show usage/help on execution errors (only show on invalid command usage)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// Format error with context if it's a RaiozError
		fmt.Print(errors.FormatError(err))
		os.Exit(1)
	}
}

func init() {
	// Initialize i18n before anything else (detects system locale)
	i18n.Init("")

	// Initialize logging from environment or use defaults
	logging.InitFromEnv()

	// Add global flags
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "Set log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolVar(&logJSON, "log-json", false, "Output logs in JSON format")
	rootCmd.PersistentFlags().StringVar(&langFlag, "lang", "", i18n.T("flag.lang"))

	// Hook to update logging and language when flags are parsed
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if logLevel != "" {
			logging.SetLevel(logging.ParseLogLevel(logLevel))
		}
		if logJSON {
			logging.SetJSONFormat(true)
		}
		if langFlag != "" {
			i18n.SetLang(langFlag)
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
	rootCmd.AddCommand(overrideCmd)
	rootCmd.AddCommand(workspaceCmd)
	rootCmd.AddCommand(ignoreCmd)
	rootCmd.AddCommand(linkCmd)
	rootCmd.AddCommand(langCmd)
}
