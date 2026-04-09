package cli

import (
	"fmt"
	"os"

	"raioz/internal/config"
	"raioz/internal/output"

	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
)

var migrateYAMLFrom string
var migrateYAMLOutput string

var migrateYAMLCmd = &cobra.Command{
	Use:   "yaml",
	Short: "Convert .raioz.json to raioz.yaml",
	Long:  "Convert an existing .raioz.json config to the new raioz.yaml format.",
	RunE: func(cmd *cobra.Command, args []string) error {
		from := migrateYAMLFrom
		if from == "" {
			from = ".raioz.json"
		}

		// Load old config
		deps, _, err := config.LoadDeps(from)
		if err != nil {
			return fmt.Errorf("failed to load %s: %w", from, err)
		}

		// Convert to new YAML format
		cfg := depsToYAMLConfig(deps)

		// Marshal to YAML
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("failed to generate YAML: %w", err)
		}

		out := migrateYAMLOutput
		if out == "" {
			out = "raioz.yaml"
		}

		if err := os.WriteFile(out, data, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", out, err)
		}

		output.PrintSuccess(fmt.Sprintf("Generated %s from %s", out, from))
		output.PrintInfo("You can now delete " + from + " when ready")
		return nil
	},
}

func init() {
	migrateCmd.AddCommand(migrateYAMLCmd)
	migrateYAMLCmd.Flags().StringVar(&migrateYAMLFrom, "from", "", "Path to .raioz.json (default: .raioz.json)")
	migrateYAMLCmd.Flags().StringVarP(&migrateYAMLOutput, "output", "o", "", "Output path (default: raioz.yaml)")
}

// depsToYAMLConfig converts an old Deps struct to the new RaiozConfig format.
func depsToYAMLConfig(deps *config.Deps) config.RaiozConfig {
	cfg := config.RaiozConfig{
		Project:   deps.Project.Name,
		Workspace: deps.Workspace,
		Services:  make(map[string]config.YAMLService),
		Deps:      make(map[string]config.YAMLDependency),
	}

	// Convert services
	for name, svc := range deps.Services {
		yamlSvc := config.YAMLService{
			DependsOn: config.YAMLStringSlice(svc.GetDependsOn()),
		}

		switch svc.Source.Kind {
		case "git":
			yamlSvc.Git = svc.Source.Repo
			yamlSvc.Branch = svc.Source.Branch
			if svc.Source.Path != "" {
				yamlSvc.Path = svc.Source.Path
			}
		case "local":
			yamlSvc.Path = svc.Source.Path
		}

		if svc.Docker != nil && len(svc.Docker.Ports) > 0 {
			yamlSvc.Ports = config.YAMLStringSlice(svc.Docker.Ports)
		}

		cfg.Services[name] = yamlSvc
	}

	// Convert infra to dependencies
	for name, entry := range deps.Infra {
		if entry.Inline != nil {
			imageRef := entry.Inline.Image
			if entry.Inline.Tag != "" {
				imageRef += ":" + entry.Inline.Tag
			}

			dep := config.YAMLDependency{
				Image: imageRef,
			}
			if len(entry.Inline.Ports) > 0 {
				dep.Ports = config.YAMLStringSlice(entry.Inline.Ports)
			}
			if len(entry.Inline.Volumes) > 0 {
				dep.Volumes = config.YAMLStringSlice(entry.Inline.Volumes)
			}

			cfg.Deps[name] = dep
		}
	}

	return cfg
}
