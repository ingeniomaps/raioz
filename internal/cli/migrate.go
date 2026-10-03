package cli

import (
	"fmt"
	"os"

	"raioz/internal/config"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/output"
	"raioz/internal/production"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	migrateComposePath string
	migrateOutputPath  string
	migrateProjectName string
	migrateNetworkName string
	migrateForce       bool
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Convert a Docker Compose file to raioz.yaml",
	Long:  "Convert a production Docker Compose file to raioz.yaml.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if migrateComposePath == "" {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				"Compose path required",
			).WithSuggestion(
				"Please specify the path to the docker-compose.yml file using --compose flag",
			)
		}

		if migrateProjectName == "" {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				"Project name required",
			).WithSuggestion(
				"Please specify the project name using --project flag",
			)
		}

		// Load production configuration
		prodConfig, err := production.LoadComposeFile(migrateComposePath)
		if err != nil {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				fmt.Sprintf("Failed to load compose file from %s", migrateComposePath),
			).WithError(err).WithContext("compose_path", migrateComposePath)
		}

		if migrateOutputPath == "" {
			migrateOutputPath = "raioz.yaml"
		}
		// A raioz.yaml is hand-edited; never replace one unasked.
		if _, statErr := os.Stat(migrateOutputPath); statErr == nil && !migrateForce {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				i18n.T("error.migrate_output_exists", migrateOutputPath),
			).WithSuggestion(i18n.T("error.migrate_output_exists_suggestion"))
		}

		migrated := production.MigrateCompose(prodConfig)
		cfg := migratedToYAMLConfig(migrated, migrateProjectName, migrateNetworkName)

		outputData, err := yaml.Marshal(cfg)
		if err != nil {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				"Failed to marshal raioz.yaml",
			).WithError(err)
		}

		if err := os.WriteFile(migrateOutputPath, outputData, 0644); err != nil {
			return errors.New(
				errors.ErrCodeInvalidConfig,
				fmt.Sprintf("Failed to write %s", migrateOutputPath),
			).WithError(err).WithContext("output_path", migrateOutputPath)
		}

		for _, warning := range migrated.Warnings {
			output.PrintWarning(warning)
		}
		output.PrintSuccess(i18n.T("output.migrate_written", migrateOutputPath))
		if len(migrated.Warnings) > 0 {
			output.PrintWarning(i18n.T("output.migrate_review", len(migrated.Warnings)))
		} else {
			output.PrintSuccess(i18n.T("output.migrate_success"))
		}

		return nil
	},
}

// migratedToYAMLConfig renders a classified compose project as raioz.yaml.
func migratedToYAMLConfig(m *production.MigratedProject, project, network string) config.RaiozConfig {
	cfg := config.RaiozConfig{
		Version:  config.CurrentSchemaVersion,
		Project:  project,
		Services: make(map[string]config.YAMLService),
		Deps:     make(map[string]config.YAMLDependency),
	}
	if network != "" {
		cfg.Network = &config.YAMLNetwork{Name: network}
	}
	for name, svc := range m.Services {
		cfg.Services[name] = config.YAMLService{
			Path:      svc.Path,
			DependsOn: config.YAMLStringSlice(svc.DependsOn),
			Env:       config.YAMLStringSlice(svc.EnvFiles),
		}
	}
	for name, dep := range m.Dependencies {
		cfg.Deps[name] = config.YAMLDependency{
			Image:   dep.Image,
			Ports:   config.YAMLStringSlice(dep.Ports),
			Volumes: config.YAMLStringSlice(dep.Volumes),
			Env:     config.YAMLStringSlice(dep.EnvFiles),
		}
	}
	return cfg
}

func init() {
	migrateCmd.Flags().StringVarP(
		&migrateComposePath,
		"compose",
		"c",
		"",
		"Path to docker-compose.yml file (required)",
	)
	migrateCmd.Flags().StringVarP(
		&migrateOutputPath,
		"output",
		"o",
		"raioz.yaml",
		"Output path for the generated raioz.yaml",
	)
	migrateCmd.Flags().BoolVar(&migrateForce, "force", false, "Overwrite the output file if it exists")
	migrateCmd.Flags().StringVarP(
		&migrateProjectName,
		"project",
		"p",
		"",
		"Project name (required)",
	)
	migrateCmd.Flags().StringVar(
		&migrateNetworkName,
		"network",
		"",
		"Network name (defaults to the one raioz derives from the project)",
	)
	// MarkFlagRequired only errors when the flag name doesn't exist — a
	// compile-time programmer mistake, not a runtime condition.
	_ = migrateCmd.MarkFlagRequired("compose")
	_ = migrateCmd.MarkFlagRequired("project")
	// Note: migrateCmd is added to rootCmd in root.go init() to avoid circular dependencies
}
