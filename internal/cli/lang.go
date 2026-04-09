package cli

import (
	"fmt"
	"io"
	"strings"

	"raioz/internal/i18n"

	"github.com/spf13/cobra"
)

var langCmd = &cobra.Command{
	Use:   "lang",
	Short: "Manage language settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		printCurrentLang(cmd.OutOrStdout())
		return nil
	},
}

var langSetCmd = &cobra.Command{
	Use:   "set <language>",
	Short: "Set the display language",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return setLang(cmd.OutOrStdout(), strings.ToLower(args[0]))
	},
}

var langListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available languages",
	RunE: func(cmd *cobra.Command, args []string) error {
		printLangList(cmd.OutOrStdout())
		return nil
	},
}

func printCurrentLang(w io.Writer) {
	fmt.Fprintln(w, i18n.T("lang.current", i18n.GetLang()))
}

func setLang(w io.Writer, lang string) error {
	if err := i18n.SetLang(lang); err != nil {
		available := strings.Join(i18n.Available(), ", ")
		return fmt.Errorf("%s", i18n.T("lang.invalid", lang, available))
	}

	if err := i18n.SavePreference(lang); err != nil {
		return fmt.Errorf("failed to save preference: %w", err)
	}

	fmt.Fprintln(w, i18n.T("lang.set_success", lang))
	return nil
}

func printLangList(w io.Writer) {
	fmt.Fprintln(w, i18n.T("lang.available"))
	current := i18n.GetLang()
	for _, lang := range i18n.Available() {
		name := i18n.T("lang.name." + lang)
		marker := "  "
		if lang == current {
			marker = "* "
		}
		fmt.Fprintf(w, "  %s%s (%s)\n", marker, lang, name)
	}
}

func init() {
	langCmd.AddCommand(langSetCmd)
	langCmd.AddCommand(langListCmd)
}
