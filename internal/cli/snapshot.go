package cli

import (
	"fmt"

	"raioz/internal/app/snapshotcase"
	"raioz/internal/output"

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
		deps := newDependencies()
		uc := snapshotcase.CreateUseCase{Deps: &snapshotcase.Dependencies{
			ConfigLoader:    deps.ConfigLoader,
			SnapshotManager: deps.SnapshotManager,
		}}
		res, err := uc.Execute(cmd.Context(), snapshotcase.CreateOptions{
			ConfigPath: ResolveConfigPath(snapshotConfigPath),
			Name:       args[0],
		})
		if err != nil {
			return err
		}
		if res.NoVolumes {
			output.PrintInfo("No volumes found to snapshot")
			return nil
		}
		output.PrintSuccess(fmt.Sprintf(
			"Snapshot '%s' created with %d volumes",
			res.Snapshot.Name, len(res.Snapshot.Volumes)))
		return nil
	},
}

var snapshotRestoreCmd = &cobra.Command{
	Use:          "restore [name]",
	Short:        "Restore volumes from a snapshot",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := newDependencies()
		uc := snapshotcase.RestoreUseCase{Deps: &snapshotcase.Dependencies{
			ConfigLoader:    deps.ConfigLoader,
			SnapshotManager: deps.SnapshotManager,
		}}
		if err := uc.Execute(cmd.Context(), snapshotcase.RestoreOptions{
			ConfigPath: ResolveConfigPath(snapshotConfigPath),
			Name:       args[0],
		}); err != nil {
			return err
		}
		output.PrintSuccess(fmt.Sprintf("Snapshot '%s' restored", args[0]))
		return nil
	},
}

var snapshotListCmd = &cobra.Command{
	Use:          "list",
	Short:        "List snapshots",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		deps := newDependencies()
		uc := snapshotcase.ListUseCase{Deps: &snapshotcase.Dependencies{
			ConfigLoader:    deps.ConfigLoader,
			SnapshotManager: deps.SnapshotManager,
		}}
		snaps, err := uc.Execute(cmd.Context(), snapshotcase.ListOptions{
			ConfigPath: ResolveConfigPath(snapshotConfigPath),
		})
		if err != nil {
			return err
		}
		if len(snaps) == 0 {
			output.PrintInfo("No snapshots found")
			return nil
		}
		for _, snap := range snaps {
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
		deps := newDependencies()
		uc := snapshotcase.DeleteUseCase{Deps: &snapshotcase.Dependencies{
			ConfigLoader:    deps.ConfigLoader,
			SnapshotManager: deps.SnapshotManager,
		}}
		if err := uc.Execute(cmd.Context(), snapshotcase.DeleteOptions{
			ConfigPath: ResolveConfigPath(snapshotConfigPath),
			Name:       args[0],
		}); err != nil {
			return err
		}
		output.PrintSuccess(fmt.Sprintf("Snapshot '%s' deleted", args[0]))
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
