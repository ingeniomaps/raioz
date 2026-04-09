package cli

import (
	"fmt"

	"raioz/internal/app"
	"raioz/internal/output"
	"raioz/internal/snapshot"

	"github.com/spf13/cobra"
)

var snapshotConfigPath string

var snapshotCmd = &cobra.Command{
	Use:          "snapshot",
	Short:        "Manage volume snapshots",
	SilenceUsage: true,
}

var snapshotCreateCmd = &cobra.Command{
	Use:          "create [name]",
	Short:        "Create a snapshot of project volumes",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		deps := app.NewDependencies()
		configPath := ResolveConfigPath(snapshotConfigPath)

		cfgDeps, _, err := deps.ConfigLoader.LoadDeps(configPath)
		if err != nil {
			return err
		}

		// Collect volumes from infra
		volumes := make(map[string]string)
		for svcName, entry := range cfgDeps.Infra {
			if entry.Inline != nil {
				for _, vol := range entry.Inline.Volumes {
					volumes[vol] = svcName
				}
			}
		}

		if len(volumes) == 0 {
			output.PrintInfo("No volumes found to snapshot")
			return nil
		}

		mgr := snapshot.NewManager("")
		snap, err := mgr.Create(cfgDeps.Project.Name, name, volumes)
		if err != nil {
			return err
		}

		output.PrintSuccess(fmt.Sprintf("Snapshot '%s' created with %d volumes", snap.Name, len(snap.Volumes)))
		return nil
	},
}

var snapshotRestoreCmd = &cobra.Command{
	Use:          "restore [name]",
	Short:        "Restore volumes from a snapshot",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		deps := app.NewDependencies()
		configPath := ResolveConfigPath(snapshotConfigPath)

		cfgDeps, _, err := deps.ConfigLoader.LoadDeps(configPath)
		if err != nil {
			return err
		}

		mgr := snapshot.NewManager("")
		if err := mgr.Restore(cfgDeps.Project.Name, name); err != nil {
			return err
		}

		output.PrintSuccess(fmt.Sprintf("Snapshot '%s' restored", name))
		return nil
	},
}

var snapshotListCmd = &cobra.Command{
	Use:          "list",
	Short:        "List snapshots",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := app.NewDependencies()
		configPath := ResolveConfigPath(snapshotConfigPath)

		cfgDeps, _, err := deps.ConfigLoader.LoadDeps(configPath)
		if err != nil {
			return err
		}

		mgr := snapshot.NewManager("")
		snapshots, err := mgr.List(cfgDeps.Project.Name)
		if err != nil {
			return err
		}

		if len(snapshots) == 0 {
			output.PrintInfo("No snapshots found")
			return nil
		}

		for _, snap := range snapshots {
			var totalSize int64
			for _, vol := range snap.Volumes {
				totalSize += vol.SizeBytes
			}
			fmt.Printf("  %-20s %s  %d volumes  %.1f MB\n",
				snap.Name,
				snap.CreatedAt.Format("2006-01-02 15:04"),
				len(snap.Volumes),
				float64(totalSize)/1024/1024,
			)
		}
		return nil
	},
}

var snapshotDeleteCmd = &cobra.Command{
	Use:          "delete [name]",
	Short:        "Delete a snapshot",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		deps := app.NewDependencies()
		configPath := ResolveConfigPath(snapshotConfigPath)

		cfgDeps, _, err := deps.ConfigLoader.LoadDeps(configPath)
		if err != nil {
			return err
		}

		mgr := snapshot.NewManager("")
		if err := mgr.Delete(cfgDeps.Project.Name, name); err != nil {
			return err
		}

		output.PrintSuccess(fmt.Sprintf("Snapshot '%s' deleted", name))
		return nil
	},
}

func init() {
	snapshotCmd.PersistentFlags().StringVarP(&snapshotConfigPath, "file", "f", "", "Path to config file")
	snapshotCmd.AddCommand(snapshotCreateCmd)
	snapshotCmd.AddCommand(snapshotRestoreCmd)
	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotCmd.AddCommand(snapshotDeleteCmd)
}
