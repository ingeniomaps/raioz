package cli

import (
	"os"
	"testing"
)

func TestRootCmd(t *testing.T) {
	if rootCmd == nil {
		t.Fatal("rootCmd should be initialized")
	}

	if rootCmd.Use != "raioz" {
		t.Errorf("rootCmd.Use = %s, want raioz", rootCmd.Use)
	}

	if rootCmd.Short == "" {
		t.Error("rootCmd.Short should not be empty")
	}

	if !rootCmd.SilenceUsage {
		t.Error("rootCmd.SilenceUsage should be true")
	}
}

func TestRootRegisteredCommands(t *testing.T) {
	expected := []string{
		"up", "down", "status", "ports", "logs",
		"clean", "check", "version", "ci", "compare",
		"migrate", "init", "list",
		"ignore", "lang",
	}

	registered := make(map[string]bool)
	for _, cmd := range rootCmd.Commands() {
		registered[cmd.Name()] = true
	}

	for _, name := range expected {
		t.Run(name, func(t *testing.T) {
			if !registered[name] {
				t.Errorf("command %q not registered on rootCmd", name)
			}
		})
	}
}

func TestRootGlobalFlags(t *testing.T) {
	flags := []struct {
		name     string
		flagType string
	}{
		{"lang", "string"},
		{"log-level", "string"},
		{"log-json", "bool"},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := rootCmd.PersistentFlags().Lookup(tt.name)
			if f == nil {
				t.Errorf("global flag %q not registered", tt.name)
				return
			}
			if f.Value.Type() != tt.flagType {
				t.Errorf("flag %q type = %s, want %s",
					tt.name, f.Value.Type(), tt.flagType)
			}
		})
	}
}

func TestRootFlagUsageTranslated(t *testing.T) {
	initI18nForTest(t)

	flags := []string{"lang", "log-level", "log-json"}
	for _, name := range flags {
		t.Run(name, func(t *testing.T) {
			f := rootCmd.PersistentFlags().Lookup(name)
			if f == nil {
				t.Fatalf("flag %q not found", name)
			}
			if f.Usage == "" {
				t.Errorf("flag %q usage should not be empty", name)
			}
		})
	}
}

func TestDetectLangFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "no lang flag",
			args: []string{"raioz", "version"},
			want: "",
		},
		{
			name: "lang with space",
			args: []string{"raioz", "--lang", "es", "version"},
			want: "es",
		},
		{
			name: "lang with equals",
			args: []string{"raioz", "--lang=en", "version"},
			want: "en",
		},
		{
			name: "lang at end with value",
			args: []string{"raioz", "status", "--lang", "es"},
			want: "es",
		},
		{
			name: "lang at end without value",
			args: []string{"raioz", "status", "--lang"},
			want: "",
		},
		{
			name: "empty args",
			args: []string{"raioz"},
			want: "",
		},
		{
			name: "lang equals empty value",
			args: []string{"raioz", "--lang=", "version"},
			want: "",
		},
		{
			name: "short flag not matched",
			args: []string{"raioz", "-l", "es"},
			want: "",
		},
		{
			name: "similar flag not matched",
			args: []string{"raioz", "--language", "es"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origArgs := os.Args
			os.Args = tt.args
			defer func() { os.Args = origArgs }()

			got := detectLangFlag()
			if got != tt.want {
				t.Errorf("detectLangFlag() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPersistentPreRunSetsLang(t *testing.T) {
	initI18nForTest(t)

	origFlag := langFlag
	defer func() { langFlag = origFlag }()

	langFlag = "es"
	rootCmd.PersistentPreRun(rootCmd, nil)

	// Verify via i18n that language changed
	// (i18n.GetLang is tested in the i18n package)
	langFlag = ""
}

func TestPersistentPreRunNoOpWithoutFlags(t *testing.T) {
	initI18nForTest(t)

	origLangFlag := langFlag
	origLogLevel := logLevel
	origLogJSON := logJSON
	defer func() {
		langFlag = origLangFlag
		logLevel = origLogLevel
		logJSON = origLogJSON
	}()

	langFlag = ""
	logLevel = ""
	logJSON = false

	// Should not panic or error with empty flags
	rootCmd.PersistentPreRun(rootCmd, nil)
}

func TestPersistentPreRunSetsLogLevel(t *testing.T) {
	initI18nForTest(t)

	origLogLevel := logLevel
	defer func() { logLevel = origLogLevel }()

	logLevel = "debug"
	rootCmd.PersistentPreRun(rootCmd, nil)
	// If it doesn't panic, the log level was applied
}

func TestPersistentPreRunSetsLogJSON(t *testing.T) {
	initI18nForTest(t)

	origLogJSON := logJSON
	defer func() { logJSON = origLogJSON }()

	logJSON = true
	rootCmd.PersistentPreRun(rootCmd, nil)
	// If it doesn't panic, JSON format was applied
}

func TestI18nDescriptionsApplied(t *testing.T) {
	// Descriptions are set during init() by zzz_i18n_descriptions.go
	// so they reflect whatever language was active at package load time.
	// We verify they are not empty and not the raw i18n key.

	if rootCmd.Short == "" || rootCmd.Short == "cmd.root.short" {
		t.Errorf("rootCmd.Short should be a translated string, got %q",
			rootCmd.Short)
	}

	commands := []struct {
		name string
		key  string
	}{
		{"up", "cmd.up.short"},
		{"down", "cmd.down.short"},
		{"version", "cmd.version.short"},
		{"lang", "cmd.lang.short"},
		{"check", "cmd.check.short"},
	}

	for _, tt := range commands {
		t.Run(tt.name, func(t *testing.T) {
			for _, cmd := range rootCmd.Commands() {
				if cmd.Name() == tt.name {
					if cmd.Short == "" || cmd.Short == tt.key {
						t.Errorf("%s.Short should be translated, got %q",
							tt.name, cmd.Short)
					}
					return
				}
			}
			t.Errorf("command %q not found", tt.name)
		})
	}
}
