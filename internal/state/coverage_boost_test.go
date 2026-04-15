package state

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"raioz/internal/config"
	"raioz/internal/workspace"
)

// --- formatSlice ---

func TestFormatSlice(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"nil", nil, "[]"},
		{"empty", []string{}, "[]"},
		{"one", []string{"a"}, "[a]"},
		{"many", []string{"a", "b", "c"}, "[a b c]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSlice(tt.in)
			if got != tt.want {
				t.Errorf("formatSlice(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// --- compareServiceFields ---

func TestCompareServiceFields_AllPaths(t *testing.T) {
	tests := []struct {
		name       string
		oldSvc     config.Service
		newSvc     config.Service
		wantFields []string
	}{
		{
			name:       "identical",
			oldSvc:     config.Service{Source: config.SourceConfig{Branch: "main"}},
			newSvc:     config.Service{Source: config.SourceConfig{Branch: "main"}},
			wantFields: nil,
		},
		{
			name:       "branch change",
			oldSvc:     config.Service{Source: config.SourceConfig{Branch: "main"}},
			newSvc:     config.Service{Source: config.SourceConfig{Branch: "dev"}},
			wantFields: []string{"source.branch"},
		},
		{
			name:       "tag change",
			oldSvc:     config.Service{Source: config.SourceConfig{Tag: "v1"}},
			newSvc:     config.Service{Source: config.SourceConfig{Tag: "v2"}},
			wantFields: []string{"source.tag"},
		},
		{
			name:       "image change",
			oldSvc:     config.Service{Source: config.SourceConfig{Image: "a"}},
			newSvc:     config.Service{Source: config.SourceConfig{Image: "b"}},
			wantFields: []string{"source.image"},
		},
		{
			name:       "dependsOn change",
			oldSvc:     config.Service{DependsOn: []string{"a"}},
			newSvc:     config.Service{DependsOn: []string{"a", "b"}},
			wantFields: []string{"dependsOn"},
		},
		{
			name: "docker ports change",
			oldSvc: config.Service{Docker: &config.DockerConfig{
				Ports: []string{"8080:8080"},
			}},
			newSvc: config.Service{Docker: &config.DockerConfig{
				Ports: []string{"9090:9090"},
			}},
			wantFields: []string{"docker.ports"},
		},
		{
			name: "docker dependsOn change",
			oldSvc: config.Service{Docker: &config.DockerConfig{
				DependsOn: []string{"db"},
			}},
			newSvc: config.Service{Docker: &config.DockerConfig{
				DependsOn: []string{"db", "cache"},
			}},
			wantFields: []string{"docker.dependsOn"},
		},
		{
			name: "dockerfile change",
			oldSvc: config.Service{Docker: &config.DockerConfig{
				Dockerfile: "Dockerfile",
			}},
			newSvc: config.Service{Docker: &config.DockerConfig{
				Dockerfile: "Dockerfile.dev",
			}},
			wantFields: []string{"docker.dockerfile"},
		},
		{
			name: "command change",
			oldSvc: config.Service{Docker: &config.DockerConfig{
				Command: "npm start",
			}},
			newSvc: config.Service{Docker: &config.DockerConfig{
				Command: "npm run dev",
			}},
			wantFields: []string{"docker.command"},
		},
		{
			name: "docker removed (was docker, now host)",
			oldSvc: config.Service{Docker: &config.DockerConfig{
				Command: "x",
			}},
			newSvc:     config.Service{Docker: nil},
			wantFields: []string{"docker"},
		},
		{
			name:   "docker added (was host, now docker)",
			oldSvc: config.Service{Docker: nil},
			newSvc: config.Service{Docker: &config.DockerConfig{
				Command: "x",
			}},
			wantFields: []string{"docker"},
		},
		{
			name: "multiple changes at once",
			oldSvc: config.Service{
				Source:    config.SourceConfig{Branch: "main", Tag: "v1"},
				DependsOn: []string{"a"},
				Docker: &config.DockerConfig{
					Ports:     []string{"80:80"},
					DependsOn: []string{"db"},
				},
			},
			newSvc: config.Service{
				Source:    config.SourceConfig{Branch: "dev", Tag: "v2"},
				DependsOn: []string{"a", "b"},
				Docker: &config.DockerConfig{
					Ports:     []string{"81:81"},
					DependsOn: []string{"db", "cache"},
				},
			},
			wantFields: []string{
				"source.branch", "source.tag", "dependsOn",
				"docker.ports", "docker.dependsOn",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := compareServiceFields("svc", tt.oldSvc, tt.newSvc)
			if len(changes) != len(tt.wantFields) {
				t.Fatalf("got %d changes, want %d: %+v", len(changes), len(tt.wantFields), changes)
			}
			for i, want := range tt.wantFields {
				if changes[i].Field != want {
					t.Errorf("[%d] got field=%q, want %q", i, changes[i].Field, want)
				}
			}
		})
	}
}

// --- compareInfra ---

func TestCompareInfra_AllPaths(t *testing.T) {
	tests := []struct {
		name       string
		oldInfra   map[string]config.InfraEntry
		newInfra   map[string]config.InfraEntry
		wantFields []string
	}{
		{
			name:       "both empty",
			oldInfra:   nil,
			newInfra:   nil,
			wantFields: nil,
		},
		{
			name:     "infra added",
			oldInfra: map[string]config.InfraEntry{},
			newInfra: map[string]config.InfraEntry{
				"pg": {Inline: &config.Infra{Image: "postgres", Tag: "16"}},
			},
			wantFields: []string{"added"},
		},
		{
			name: "infra removed",
			oldInfra: map[string]config.InfraEntry{
				"pg": {Inline: &config.Infra{Image: "postgres"}},
			},
			newInfra:   map[string]config.InfraEntry{},
			wantFields: []string{"removed"},
		},
		{
			name: "definition change (path→inline)",
			oldInfra: map[string]config.InfraEntry{
				"pg": {Path: "./pg.yml"},
			},
			newInfra: map[string]config.InfraEntry{
				"pg": {Inline: &config.Infra{Image: "postgres"}},
			},
			wantFields: []string{"definition"},
		},
		{
			name: "definition change (inline→path)",
			oldInfra: map[string]config.InfraEntry{
				"pg": {Inline: &config.Infra{Image: "postgres"}},
			},
			newInfra: map[string]config.InfraEntry{
				"pg": {Path: "./pg.yml"},
			},
			wantFields: []string{"definition"},
		},
		{
			name: "path change",
			oldInfra: map[string]config.InfraEntry{
				"pg": {Path: "./a.yml"},
			},
			newInfra: map[string]config.InfraEntry{
				"pg": {Path: "./b.yml"},
			},
			wantFields: []string{"path"},
		},
		{
			name: "path unchanged",
			oldInfra: map[string]config.InfraEntry{
				"pg": {Path: "./a.yml"},
			},
			newInfra: map[string]config.InfraEntry{
				"pg": {Path: "./a.yml"},
			},
			wantFields: nil,
		},
		{
			name: "inline image change",
			oldInfra: map[string]config.InfraEntry{
				"pg": {Inline: &config.Infra{Image: "postgres", Tag: "15"}},
			},
			newInfra: map[string]config.InfraEntry{
				"pg": {Inline: &config.Infra{Image: "mariadb", Tag: "15"}},
			},
			wantFields: []string{"image"},
		},
		{
			name: "inline tag change",
			oldInfra: map[string]config.InfraEntry{
				"pg": {Inline: &config.Infra{Image: "postgres", Tag: "15"}},
			},
			newInfra: map[string]config.InfraEntry{
				"pg": {Inline: &config.Infra{Image: "postgres", Tag: "16"}},
			},
			wantFields: []string{"tag"},
		},
		{
			name: "inline ports change",
			oldInfra: map[string]config.InfraEntry{
				"pg": {Inline: &config.Infra{Image: "postgres", Ports: []string{"5432"}}},
			},
			newInfra: map[string]config.InfraEntry{
				"pg": {Inline: &config.Infra{Image: "postgres", Ports: []string{"5433"}}},
			},
			wantFields: []string{"ports"},
		},
		{
			name: "inline nil skips",
			oldInfra: map[string]config.InfraEntry{
				"pg": {},
			},
			newInfra: map[string]config.InfraEntry{
				"pg": {},
			},
			wantFields: nil,
		},
		{
			name: "inline all changed",
			oldInfra: map[string]config.InfraEntry{
				"pg": {Inline: &config.Infra{Image: "a", Tag: "1", Ports: []string{"1"}}},
			},
			newInfra: map[string]config.InfraEntry{
				"pg": {Inline: &config.Infra{Image: "b", Tag: "2", Ports: []string{"2"}}},
			},
			wantFields: []string{"image", "tag", "ports"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := compareInfra(tt.oldInfra, tt.newInfra)
			if len(changes) != len(tt.wantFields) {
				t.Fatalf("got %d, want %d: %+v", len(changes), len(tt.wantFields), changes)
			}
			for i, want := range tt.wantFields {
				if changes[i].Field != want {
					t.Errorf("[%d] got field=%q, want %q", i, changes[i].Field, want)
				}
			}
		})
	}
}

// --- CheckAlignment ---

func TestCheckAlignment_NoSavedState(t *testing.T) {
	ws := &workspace.Workspace{Root: t.TempDir()}
	deps := &config.Deps{
		Project:  config.Project{Name: "test"},
		Services: map[string]config.Service{},
		Infra:    map[string]config.InfraEntry{},
	}
	issues, err := CheckAlignment(ws, deps)
	if err != nil {
		t.Fatalf("CheckAlignment: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues with no saved state, got %d", len(issues))
	}
}

func TestCheckAlignment_CorruptedSavedState(t *testing.T) {
	dir := t.TempDir()
	ws := &workspace.Workspace{Root: dir}
	// Write corrupt JSON to state file
	if err := os.WriteFile(filepath.Join(dir, ".state.json"), []byte("{not json"), 0600); err != nil {
		t.Fatal(err)
	}
	deps := &config.Deps{Services: map[string]config.Service{}}
	_, err := CheckAlignment(ws, deps)
	if err == nil {
		t.Error("expected error on corrupted state")
	}
}

func TestCheckAlignment_DetectsPortChange(t *testing.T) {
	ws := &workspace.Workspace{Root: t.TempDir()}
	oldDeps := &config.Deps{
		Project: config.Project{Name: "test"},
		Services: map[string]config.Service{
			"api": {
				Source: config.SourceConfig{Kind: "local"},
				Docker: &config.DockerConfig{Ports: []string{"80:80"}},
			},
		},
	}
	if err := Save(ws, oldDeps); err != nil {
		t.Fatal(err)
	}

	newDeps := &config.Deps{
		Project: config.Project{Name: "test"},
		Services: map[string]config.Service{
			"api": {
				Source: config.SourceConfig{Kind: "local"},
				Docker: &config.DockerConfig{Ports: []string{"81:80"}},
			},
		},
	}

	issues, err := CheckAlignment(ws, newDeps)
	if err != nil {
		t.Fatalf("CheckAlignment: %v", err)
	}

	foundCritical := false
	for _, issue := range issues {
		if issue.Severity == "critical" {
			foundCritical = true
		}
	}
	if !foundCritical {
		t.Errorf("expected critical issue for port change, got: %+v", issues)
	}
}

func TestCheckAlignment_DetectsServiceAdded(t *testing.T) {
	ws := &workspace.Workspace{Root: t.TempDir()}
	oldDeps := &config.Deps{
		Project:  config.Project{Name: "test"},
		Services: map[string]config.Service{},
	}
	if err := Save(ws, oldDeps); err != nil {
		t.Fatal(err)
	}

	newDeps := &config.Deps{
		Project: config.Project{Name: "test"},
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "local"}},
		},
	}

	issues, err := CheckAlignment(ws, newDeps)
	if err != nil {
		t.Fatalf("CheckAlignment: %v", err)
	}

	foundAdded := false
	for _, issue := range issues {
		if strings.Contains(issue.Description, "added") {
			foundAdded = true
		}
	}
	if !foundAdded {
		t.Errorf("expected issue referring to added service, got: %+v", issues)
	}
}

func TestCheckAlignment_DetectsServiceRemoved(t *testing.T) {
	ws := &workspace.Workspace{Root: t.TempDir()}
	oldDeps := &config.Deps{
		Project: config.Project{Name: "test"},
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "local"}},
		},
	}
	if err := Save(ws, oldDeps); err != nil {
		t.Fatal(err)
	}

	newDeps := &config.Deps{
		Project:  config.Project{Name: "test"},
		Services: map[string]config.Service{},
	}

	issues, err := CheckAlignment(ws, newDeps)
	if err != nil {
		t.Fatalf("CheckAlignment: %v", err)
	}

	// "removed" is critical
	foundCritical := false
	for _, issue := range issues {
		if issue.Severity == "critical" {
			foundCritical = true
		}
	}
	if !foundCritical {
		t.Errorf("expected critical issue for removed service, got: %+v", issues)
	}
}

func TestCheckAlignment_SkipsBranchAndTagChangesHere(t *testing.T) {
	// source.branch, source.tag, image, tag are handled in drift detection
	// and should be skipped in the main loop.
	ws := &workspace.Workspace{Root: t.TempDir()}
	oldDeps := &config.Deps{
		Project: config.Project{Name: "test"},
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "image", Image: "nginx", Tag: "1.0"}},
		},
	}
	if err := Save(ws, oldDeps); err != nil {
		t.Fatal(err)
	}

	newDeps := &config.Deps{
		Project: config.Project{Name: "test"},
		Services: map[string]config.Service{
			"api": {Source: config.SourceConfig{Kind: "image", Image: "nginx", Tag: "1.1"}},
		},
	}

	issues, err := CheckAlignment(ws, newDeps)
	if err != nil {
		t.Fatalf("CheckAlignment: %v", err)
	}
	// Tag change should produce a warning-level issue for image-based services
	// via the dedicated tag/image check at bottom of function.
	found := false
	for _, issue := range issues {
		if strings.Contains(issue.Description, "Image tag changed") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected image tag change issue, got: %+v", issues)
	}
}

func TestCheckAlignment_InfraTagChange(t *testing.T) {
	ws := &workspace.Workspace{Root: t.TempDir()}
	oldDeps := &config.Deps{
		Project: config.Project{Name: "test"},
		Infra: map[string]config.InfraEntry{
			"pg": {Inline: &config.Infra{Image: "postgres", Tag: "15"}},
		},
	}
	if err := Save(ws, oldDeps); err != nil {
		t.Fatal(err)
	}

	newDeps := &config.Deps{
		Project: config.Project{Name: "test"},
		Infra: map[string]config.InfraEntry{
			"pg": {Inline: &config.Infra{Image: "postgres", Tag: "16"}},
		},
	}

	issues, err := CheckAlignment(ws, newDeps)
	if err != nil {
		t.Fatalf("CheckAlignment: %v", err)
	}

	found := false
	for _, issue := range issues {
		if strings.Contains(issue.Description, "Infra tag changed") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected infra tag change, got: %+v", issues)
	}
}

func TestCheckAlignment_GitServiceWithoutRepo(t *testing.T) {
	ws := &workspace.Workspace{Root: t.TempDir()}
	oldDeps := &config.Deps{
		Project: config.Project{Name: "test"},
		Services: map[string]config.Service{
			"api": {
				Source: config.SourceConfig{
					Kind: "git", Branch: "main", Repo: "https://example.com/a.git",
					Path: "./api",
				},
			},
		},
	}
	if err := Save(ws, oldDeps); err != nil {
		t.Fatal(err)
	}

	newDeps := oldDeps
	// No repo on disk — drift detection should be skipped without error
	issues, err := CheckAlignment(ws, newDeps)
	if err != nil {
		t.Fatalf("CheckAlignment: %v", err)
	}
	// May produce 0 issues since config is identical and no repo exists
	_ = issues
}

// --- LoadGlobalState/SaveGlobalState edge cases ---

func TestLoadGlobalState_CorruptedJSON(t *testing.T) {
	setupGlobalHome(t)
	path, err := GetGlobalStatePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{invalid"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err = LoadGlobalState()
	if err == nil {
		t.Error("expected error on corrupted JSON")
	}
}

func TestLoadGlobalState_NilMapsInitialized(t *testing.T) {
	setupGlobalHome(t)
	path, err := GetGlobalStatePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	// Write a state file where Projects and ActiveProjects are null
	if err := os.WriteFile(path, []byte(`{"activeProjects":null,"projects":null}`), 0600); err != nil {
		t.Fatal(err)
	}

	state, err := LoadGlobalState()
	if err != nil {
		t.Fatalf("LoadGlobalState: %v", err)
	}
	if state.Projects == nil {
		t.Error("Projects should be non-nil")
	}
	if state.ActiveProjects == nil {
		t.Error("ActiveProjects should be non-nil")
	}
}

func TestSaveGlobalState_ReadOnlyDir(t *testing.T) {
	// Skip on windows (permission model differs)
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	// Skip when running as root — chmod is a no-op for root
	if os.Geteuid() == 0 {
		t.Skip("skip when running as root")
	}

	base := t.TempDir()
	readonlyDir := filepath.Join(base, "readonly")
	if err := os.MkdirAll(readonlyDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(readonlyDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(readonlyDir, 0700)
	})

	t.Setenv("RAIOZ_HOME", filepath.Join(readonlyDir, "sub"))

	err := SaveGlobalState(&GlobalState{})
	if err == nil {
		t.Error("expected error writing to read-only dir")
	}
}

// --- LoadLocalState/SaveLocalState edge cases ---

func TestLoadLocalState_CorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".raioz.state.json"), []byte("{bad"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadLocalState(dir)
	if err == nil {
		t.Error("expected error on corrupt local state")
	}
}

func TestLoadLocalState_NilMapsInitialized(t *testing.T) {
	dir := t.TempDir()
	// devOverrides/hostPIDs missing/null — should init maps
	content := `{"project":"p","devOverrides":null,"hostPIDs":null}`
	if err := os.WriteFile(filepath.Join(dir, ".raioz.state.json"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	state, err := LoadLocalState(dir)
	if err != nil {
		t.Fatalf("LoadLocalState: %v", err)
	}
	if state.DevOverrides == nil {
		t.Error("DevOverrides should be non-nil")
	}
	if state.HostPIDs == nil {
		t.Error("HostPIDs should be non-nil")
	}
}

func TestSaveLocalState_InvalidDir(t *testing.T) {
	// Path that doesn't exist and can't be written to.
	err := SaveLocalState("/this/path/does/not/exist-raioz-test", &LocalState{Project: "p"})
	if err == nil {
		t.Error("expected error when project dir doesn't exist")
	}
}

func TestLoadLocalState_ReadError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("skip when running as root")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, ".raioz.state.json")
	if err := os.WriteFile(path, []byte(`{"project":"p"}`), 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(path, 0600)
	})

	_, err := LoadLocalState(dir)
	if err == nil {
		t.Error("expected error reading unreadable file")
	}
}

// --- workspace_preferences edge cases ---

func TestLoadWorkspacePreferences_CorruptedJSON(t *testing.T) {
	setupGlobalHome(t)
	path, err := getWorkspacePreferencesPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not valid json"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err = loadWorkspacePreferences()
	if err == nil {
		t.Error("expected error on corrupted prefs")
	}
}

func TestLoadWorkspacePreferences_NilMap(t *testing.T) {
	setupGlobalHome(t)
	path, err := getWorkspacePreferencesPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"byWorkspace":null}`), 0600); err != nil {
		t.Fatal(err)
	}

	prefs, err := loadWorkspacePreferences()
	if err != nil {
		t.Fatalf("loadWorkspacePreferences: %v", err)
	}
	if prefs.ByWorkspace == nil {
		t.Error("ByWorkspace should be non-nil")
	}
}

func TestSaveWorkspacePreferences_ReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skip on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("skip when running as root")
	}

	base := t.TempDir()
	readonlyDir := filepath.Join(base, "readonly")
	if err := os.MkdirAll(readonlyDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(readonlyDir, 0700)
	})

	t.Setenv("RAIOZ_HOME", filepath.Join(readonlyDir, "sub"))

	err := saveWorkspacePreferences(&WorkspacePreferences{
		ByWorkspace: map[string]WorkspaceProjectPreference{},
	})
	if err == nil {
		t.Error("expected error writing to read-only dir")
	}
}

func TestSetWorkspaceProjectPreference_NilMapInitialized(t *testing.T) {
	setupGlobalHome(t)
	// Write a prefs file with null ByWorkspace, then call Set: load must init map
	path, err := getWorkspacePreferencesPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"byWorkspace":null}`), 0600); err != nil {
		t.Fatal(err)
	}

	err = SetWorkspaceProjectPreference("ws1", WorkspaceProjectPreference{
		PreferredProject: "proj1",
	})
	if err != nil {
		t.Fatalf("SetWorkspaceProjectPreference: %v", err)
	}
	got, err := GetWorkspaceProjectPreference("ws1")
	if err != nil {
		t.Fatalf("GetWorkspaceProjectPreference: %v", err)
	}
	if got == nil || got.PreferredProject != "proj1" {
		t.Errorf("preference not persisted: %+v", got)
	}
}
