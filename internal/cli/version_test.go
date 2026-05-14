package cli

import (
	"bytes"
	"runtime/debug"
	"strings"
	"testing"
)

func TestVersionCmd(t *testing.T) {
	if versionCmd == nil {
		t.Fatal("versionCmd should be initialized")
	}

	if versionCmd.Use != "version" {
		t.Errorf("versionCmd.Use = %s, want version", versionCmd.Use)
	}

	if versionCmd.Short == "" {
		t.Error("versionCmd.Short should not be empty")
	}

	if versionCmd.Long == "" {
		t.Error("versionCmd.Long should not be empty")
	}
}

func TestVersionDefaults(t *testing.T) {
	if SchemaVersion != "1.0" {
		t.Errorf("SchemaVersion = %s, want 1.0", SchemaVersion)
	}
}

func TestPrintVersion(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		commit    string
		buildDate string
		wantLines []string
		wantNot   []string
	}{
		{
			name:      "dev build defaults",
			version:   "dev",
			commit:    "unknown",
			buildDate: "unknown",
			wantLines: []string{
				"raioz version dev",
				"Schema version: 1.0",
				"(Development build)",
			},
			wantNot: []string{
				"Commit:",
				"Build date:",
			},
		},
		{
			name:      "release build with all fields",
			version:   "v1.2.3",
			commit:    "abc1234",
			buildDate: "2026-04-01T10:00:00",
			wantLines: []string{
				"raioz version v1.2.3",
				"Schema version: 1.0",
				"Commit: abc1234",
				"Build date: 2026-04-01T10:00:00",
			},
			wantNot: []string{
				"(Development build)",
			},
		},
		{
			name:      "empty commit and build date",
			version:   "v2.0.0",
			commit:    "",
			buildDate: "",
			wantLines: []string{
				"raioz version v2.0.0",
				"Schema version: 1.0",
			},
			wantNot: []string{
				"Commit:",
				"Build date:",
				"(Development build)",
			},
		},
		{
			name:      "unknown commit shown as hidden",
			version:   "v1.0.0",
			commit:    "unknown",
			buildDate: "2026-01-01",
			wantLines: []string{
				"raioz version v1.0.0",
				"Build date: 2026-01-01",
			},
			wantNot: []string{
				"Commit:",
				"(Development build)",
			},
		},
		{
			name:      "unknown build date shown as hidden",
			version:   "v1.0.0",
			commit:    "def5678",
			buildDate: "unknown",
			wantLines: []string{
				"raioz version v1.0.0",
				"Commit: def5678",
			},
			wantNot: []string{
				"Build date:",
				"(Development build)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origVersion := Version
			origCommit := Commit
			origBuildDate := BuildDate
			defer func() {
				Version = origVersion
				Commit = origCommit
				BuildDate = origBuildDate
			}()

			Version = tt.version
			Commit = tt.commit
			BuildDate = tt.buildDate

			var buf bytes.Buffer
			printVersion(&buf)
			output := buf.String()

			for _, line := range tt.wantLines {
				if !strings.Contains(output, line) {
					t.Errorf("output missing %q\ngot:\n%s", line, output)
				}
			}

			for _, line := range tt.wantNot {
				if strings.Contains(output, line) {
					t.Errorf("output should not contain %q\ngot:\n%s", line, output)
				}
			}
		})
	}
}

func TestPopulateFromBuildInfo(t *testing.T) {
	tests := []struct {
		name        string
		initVer     string
		initCommit  string
		initDate    string
		info        *debug.BuildInfo
		wantVersion string
		wantCommit  string
		wantDate    string
	}{
		{
			name:       "module version fills dev defaults",
			initVer:    "dev",
			initCommit: "unknown",
			initDate:   "unknown",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "v1.2.3"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abcdef1234567"},
					{Key: "vcs.time", Value: "2026-05-14T10:00:00Z"},
				},
			},
			wantVersion: "v1.2.3",
			wantCommit:  "abcdef1",
			wantDate:    "2026-05-14T10:00:00Z",
		},
		{
			name:       "ldflags version is respected",
			initVer:    "v9.9.9",
			initCommit: "deadbee",
			initDate:   "2099-01-01",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "v1.2.3"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "feedface"},
					{Key: "vcs.time", Value: "2026-01-01T00:00:00Z"},
				},
			},
			wantVersion: "v9.9.9",
			wantCommit:  "deadbee",
			wantDate:    "2099-01-01",
		},
		{
			name:       "devel main version ignored, vcs info still captured",
			initVer:    "dev",
			initCommit: "unknown",
			initDate:   "unknown",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abcdef1234"},
					{Key: "vcs.time", Value: "2026-05-14T10:00:00Z"},
				},
			},
			wantVersion: "dev",
			wantCommit:  "abcdef1",
			wantDate:    "2026-05-14T10:00:00Z",
		},
		{
			name:        "nil buildinfo is a no-op",
			initVer:     "dev",
			initCommit:  "unknown",
			initDate:    "unknown",
			info:        nil,
			wantVersion: "dev",
			wantCommit:  "unknown",
			wantDate:    "unknown",
		},
		{
			name:       "short vcs revision below 7 chars is ignored",
			initVer:    "dev",
			initCommit: "unknown",
			initDate:   "unknown",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "v0.1.0"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abc"},
				},
			},
			wantVersion: "v0.1.0",
			wantCommit:  "unknown",
			wantDate:    "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origV, origC, origD := Version, Commit, BuildDate
			defer func() {
				Version, Commit, BuildDate = origV, origC, origD
			}()
			Version, Commit, BuildDate = tt.initVer, tt.initCommit, tt.initDate

			populateFromBuildInfo(tt.info)

			if Version != tt.wantVersion {
				t.Errorf("Version = %q, want %q", Version, tt.wantVersion)
			}
			if Commit != tt.wantCommit {
				t.Errorf("Commit = %q, want %q", Commit, tt.wantCommit)
			}
			if BuildDate != tt.wantDate {
				t.Errorf("BuildDate = %q, want %q", BuildDate, tt.wantDate)
			}
		})
	}
}

func TestVersionCmdRunE(t *testing.T) {
	origVersion := Version
	origCommit := Commit
	origBuildDate := BuildDate
	defer func() {
		Version = origVersion
		Commit = origCommit
		BuildDate = origBuildDate
	}()

	Version = "v1.0.0"
	Commit = "abc1234"
	BuildDate = "2026-04-01"

	var buf bytes.Buffer
	versionCmd.SetOut(&buf)
	err := versionCmd.RunE(versionCmd, nil)

	if err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "raioz version v1.0.0") {
		t.Errorf("RunE output missing version\ngot:\n%s", output)
	}
}
