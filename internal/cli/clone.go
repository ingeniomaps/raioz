package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"raioz/internal/app"
	"raioz/internal/errors"
	"raioz/internal/i18n"
	"raioz/internal/output"

	"github.com/spf13/cobra"
)

var cloneCmd = &cobra.Command{
	Use:   "clone <repo-url> [directory]",
	Short: i18n.T("cmd.clone.short"),
	Long: "Clone a git repository and start the project with raioz up.\n" +
		"If the repo contains a raioz.yaml, everything is auto-configured.\n\n" +
		"Examples:\n" +
		"  raioz clone git@github.com:acme/platform.git\n" +
		"  raioz clone https://github.com/acme/platform.git my-workspace\n" +
		"  raioz clone git@github.com:acme/platform.git --no-up",
	Args:         cobra.RangeArgs(1, 2),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		defer func() {
			if panicErr := errors.RecoverPanic("raioz clone"); panicErr != nil {
				err = panicErr
			}
		}()

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		repoURL := args[0]
		targetDir := ""
		if len(args) > 1 {
			targetDir = args[1]
		} else {
			targetDir = dirNameFromRepo(repoURL)
		}

		noUp, _ := cmd.Flags().GetBool("no-up")
		branch, _ := cmd.Flags().GetString("branch")

		// 1. Clone
		output.PrintInfo(i18n.T("clone.cloning", repoURL))

		cloneArgs := []string{"clone", "--depth", "1"}
		if branch != "" {
			cloneArgs = append(cloneArgs, "-b", branch)
		}
		cloneArgs = append(cloneArgs, repoURL, targetDir)

		gitCmd := exec.CommandContext(ctx, "git", cloneArgs...)
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		if err := gitCmd.Run(); err != nil {
			return errors.New(
				errors.ErrCodeGitCloneFailed,
				i18n.T("clone.failed", repoURL),
			).WithError(err).WithSuggestion(
				i18n.T("clone.failed_suggestion"),
			)
		}

		absDir, _ := filepath.Abs(targetDir)
		output.PrintSuccess(i18n.T("clone.cloned", absDir))

		// 2. Check for raioz.yaml
		configPath := findConfig(absDir)
		if configPath == "" {
			output.PrintWarning(i18n.T("clone.no_config"))
			fmt.Printf("\n  cd %s\n  raioz init\n  raioz up\n\n", targetDir)
			return nil
		}

		if noUp {
			fmt.Printf("\n  cd %s\n  raioz up\n\n", targetDir)
			return nil
		}

		// 3. raioz up
		output.PrintInfo(i18n.T("clone.starting"))

		if err := os.Chdir(absDir); err != nil {
			return err
		}

		deps := app.NewDependencies()
		upUC := app.NewUpUseCase(deps)
		return upUC.Execute(ctx, app.UpOptions{
			ConfigPath: configPath,
		})
	},
}

func init() {
	cloneCmd.Flags().BoolP("no-up", "n", false, "Clone only, do not run raioz up")
	cloneCmd.Flags().StringP("branch", "b", "", "Branch to clone")
}

// dirNameFromRepo extracts a directory name from a git URL.
// git@github.com:acme/platform.git → platform
// https://github.com/acme/platform.git → platform
func dirNameFromRepo(url string) string {
	// Remove trailing .git
	url = strings.TrimSuffix(url, ".git")
	// Take last segment after / or :
	if idx := strings.LastIndexAny(url, "/:"); idx >= 0 {
		return url[idx+1:]
	}
	return url
}

// findConfig looks for raioz.yaml or .raioz.json in a directory.
func findConfig(dir string) string {
	for _, name := range []string{
		"raioz.yaml", "raioz.yml", ".raioz.json",
	} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
