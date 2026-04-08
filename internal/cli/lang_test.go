package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"raioz/internal/i18n"
)

func initI18nForTest(t *testing.T) {
	t.Helper()
	os.Setenv("RAIOZ_LANG", "en")
	t.Cleanup(func() { os.Unsetenv("RAIOZ_LANG") })
	i18n.Init("en")
}

func TestLangCmd(t *testing.T) {
	if langCmd == nil {
		t.Fatal("langCmd should be initialized")
	}

	if langCmd.Use != "lang" {
		t.Errorf("langCmd.Use = %s, want lang", langCmd.Use)
	}

	if langCmd.Short == "" {
		t.Error("langCmd.Short should not be empty")
	}
}

func TestLangSubcommands(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
	}{
		{"set subcommand registered", "set"},
		{"list subcommand registered", "list"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := false
			for _, sub := range langCmd.Commands() {
				if sub.Name() == tt.cmd {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("subcommand %q not registered on langCmd", tt.cmd)
			}
		})
	}
}

func TestPrintCurrentLang(t *testing.T) {
	initI18nForTest(t)

	var buf bytes.Buffer
	printCurrentLang(&buf)
	output := buf.String()

	if !strings.Contains(output, "en") {
		t.Errorf("expected output to contain current lang 'en'\ngot: %s", output)
	}
}

func TestPrintCurrentLangSpanish(t *testing.T) {
	initI18nForTest(t)
	i18n.SetLang("es")
	t.Cleanup(func() { i18n.SetLang("en") })

	var buf bytes.Buffer
	printCurrentLang(&buf)
	output := buf.String()

	if !strings.Contains(output, "es") {
		t.Errorf("expected output to contain current lang 'es'\ngot: %s", output)
	}
}

func TestSetLang(t *testing.T) {
	initI18nForTest(t)

	// Override HOME to avoid writing to real config
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	t.Cleanup(func() {
		os.Setenv("HOME", origHome)
		i18n.SetLang("en")
	})

	tests := []struct {
		name    string
		lang    string
		wantErr bool
		wantOut string
	}{
		{
			name:    "set to spanish",
			lang:    "es",
			wantOut: "es",
		},
		{
			name:    "set to english",
			lang:    "en",
			wantOut: "en",
		},
		{
			name:    "invalid language",
			lang:    "fr",
			wantErr: true,
		},
		{
			name:    "empty language",
			lang:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := setLang(&buf, tt.lang)

			if (err != nil) != tt.wantErr {
				t.Errorf("setLang(%q) error = %v, wantErr %v",
					tt.lang, err, tt.wantErr)
				return
			}

			if !tt.wantErr && !strings.Contains(buf.String(), tt.wantOut) {
				t.Errorf("setLang(%q) output missing %q\ngot: %s",
					tt.lang, tt.wantOut, buf.String())
			}
		})
	}
}

func TestSetLangInvalidContainsAvailable(t *testing.T) {
	initI18nForTest(t)

	var buf bytes.Buffer
	err := setLang(&buf, "xyz")

	if err == nil {
		t.Fatal("expected error for invalid language")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "xyz") {
		t.Errorf("error should mention invalid lang 'xyz'\ngot: %s", errMsg)
	}

	if !strings.Contains(errMsg, "en") || !strings.Contains(errMsg, "es") {
		t.Errorf("error should list available languages\ngot: %s", errMsg)
	}
}

func TestPrintLangList(t *testing.T) {
	initI18nForTest(t)

	var buf bytes.Buffer
	printLangList(&buf)
	output := buf.String()

	if !strings.Contains(output, "en") {
		t.Error("list should contain 'en'")
	}

	if !strings.Contains(output, "es") {
		t.Error("list should contain 'es'")
	}
}

func TestPrintLangListCurrentMarker(t *testing.T) {
	initI18nForTest(t)

	tests := []struct {
		name       string
		currentLang string
		wantMarked string
	}{
		{
			name:        "english marked as current",
			currentLang: "en",
			wantMarked:  "* en",
		},
		{
			name:        "spanish marked as current",
			currentLang: "es",
			wantMarked:  "* es",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i18n.SetLang(tt.currentLang)
			t.Cleanup(func() { i18n.SetLang("en") })

			var buf bytes.Buffer
			printLangList(&buf)
			output := buf.String()

			if !strings.Contains(output, tt.wantMarked) {
				t.Errorf("expected %q to be marked as current\ngot:\n%s",
					tt.currentLang, output)
			}
		})
	}
}

func TestPrintLangListShowsNames(t *testing.T) {
	initI18nForTest(t)

	var buf bytes.Buffer
	printLangList(&buf)
	output := buf.String()

	if !strings.Contains(output, "English") {
		t.Error("list should show language name 'English'")
	}

	if !strings.Contains(output, "Spanish") {
		t.Error("list should show language name 'Spanish'")
	}
}

func TestLangCmdRunE(t *testing.T) {
	initI18nForTest(t)

	var buf bytes.Buffer
	langCmd.SetOut(&buf)
	err := langCmd.RunE(langCmd, nil)

	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("RunE should produce output")
	}
}

func TestLangListCmdRunE(t *testing.T) {
	initI18nForTest(t)

	var buf bytes.Buffer
	langListCmd.SetOut(&buf)
	err := langListCmd.RunE(langListCmd, nil)

	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "en") || !strings.Contains(output, "es") {
		t.Errorf("list RunE should show languages\ngot:\n%s", output)
	}
}

func TestLangSetCmdRunE(t *testing.T) {
	initI18nForTest(t)

	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	t.Cleanup(func() {
		os.Setenv("HOME", origHome)
		i18n.SetLang("en")
	})

	var buf bytes.Buffer
	langSetCmd.SetOut(&buf)
	err := langSetCmd.RunE(langSetCmd, []string{"es"})

	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "es") {
		t.Errorf("set RunE should confirm language\ngot:\n%s", output)
	}
}

func TestLangSetCmdArgsValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name:    "no args rejected",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "too many args rejected",
			args:    []string{"en", "es"},
			wantErr: true,
		},
		{
			name:    "one arg accepted",
			args:    []string{"en"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := langSetCmd.Args(langSetCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%v) error = %v, wantErr %v",
					tt.args, err, tt.wantErr)
			}
		})
	}
}
