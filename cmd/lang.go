package cmd

import (
	"fmt"
	"strings"

	"raioz/internal/i18n"

	"github.com/spf13/cobra"
)

var langCmd = &cobra.Command{
	Use:   "lang",
	Short: i18n.T("cmd.lang.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(i18n.T("lang.current", i18n.GetLang()))
		return nil
	},
}

var langSetCmd = &cobra.Command{
	Use:   "set <language>",
	Short: i18n.T("cmd.lang.set.short"),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		lang := strings.ToLower(args[0])

		if err := i18n.SetLang(lang); err != nil {
			available := strings.Join(i18n.Available(), ", ")
			return fmt.Errorf(i18n.T("lang.invalid", lang, available))
		}

		if err := i18n.SavePreference(lang); err != nil {
			return fmt.Errorf("failed to save preference: %w", err)
		}

		fmt.Println(i18n.T("lang.set_success", lang))
		return nil
	},
}

var langListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T("cmd.lang.list.short"),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(i18n.T("lang.available"))
		current := i18n.GetLang()
		for _, lang := range i18n.Available() {
			name := i18n.T("lang.name." + lang)
			marker := "  "
			if lang == current {
				marker = "* "
			}
			fmt.Printf("  %s%s (%s)\n", marker, lang, name)
		}
		return nil
	},
}

func init() {
	langCmd.AddCommand(langSetCmd)
	langCmd.AddCommand(langListCmd)
}
