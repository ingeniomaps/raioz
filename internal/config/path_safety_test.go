package config

import (
	stderrors "errors"
	"path/filepath"
	"testing"

	"raioz/internal/errors"
)

func TestCheckInsidePath_Accepts(t *testing.T) {
	base := t.TempDir()
	cases := []string{
		"",
		"api",
		"./api",
		"./services/api",
		"./scripts/setup.sh",
		".env.api",
		filepath.Join(base, "api"),             // absolute, inside
		filepath.Join(base, "services", "api"), // deeper absolute
		"./" + filepath.Base(base) + "/../api", // re-enters base
		"../" + filepath.Base(base) + "/api",   // exits base then re-enters
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			if err := checkInsidePath(p, base, "test"); err != nil {
				t.Errorf("expected nil for %q, got: %v", p, err)
			}
		})
	}
}

func TestCheckInsidePath_RejectsEscapes(t *testing.T) {
	base := t.TempDir()
	cases := []string{
		"../shared",
		"../../etc/passwd",
		"../../../tmp/raioz-test", // escapes base via parents (also ends in /tmp which is NOT blocked, so this catches escape only)
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			err := checkInsidePath(p, base, "test")
			if err == nil {
				t.Fatalf("expected error for %q, got nil", p)
			}
			var rerr *errors.RaiozError
			if !stderrors.As(err, &rerr) {
				t.Fatalf("expected *RaiozError, got %T", err)
			}
			if rerr.Code != errors.ErrCodeUnsafePath {
				t.Errorf("expected UNSAFE_PATH, got %s", rerr.Code)
			}
			if rerr.Context["field"] != "test" {
				t.Errorf("expected field=test in context, got %v", rerr.Context["field"])
			}
			if rerr.Suggestion == "" {
				t.Error("expected suggestion, got empty")
			}
		})
	}
}

func TestCheckInsidePath_RejectsSystemDirs(t *testing.T) {
	base := t.TempDir()
	cases := []struct {
		path    string
		wantDir string
	}{
		{"/etc/passwd", "/etc"},
		{"/etc", "/etc"},
		{"/root/.ssh", "/root"},
		{"/var/lib/docker", "/var/lib"},
		{"/sys/class/net", "/sys"},
		{"/proc/1/environ", "/proc"},
		{"/dev/sda", "/dev"},
		{"/boot/grub", "/boot"},
		// Relative paths that resolve into a system dir must also be caught.
		{"../../../../../../etc/passwd", "/etc"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			err := checkInsidePath(tc.path, base, "test")
			if err == nil {
				t.Fatalf("expected error for %q, got nil", tc.path)
			}
			var rerr *errors.RaiozError
			if !stderrors.As(err, &rerr) {
				t.Fatalf("expected *RaiozError, got %T", err)
			}
			if rerr.Code != errors.ErrCodeUnsafePath {
				t.Errorf("expected UNSAFE_PATH, got %s", rerr.Code)
			}
			if rerr.Context["system_dir"] != tc.wantDir {
				t.Errorf("expected system_dir=%q, got %v", tc.wantDir, rerr.Context["system_dir"])
			}
		})
	}
}

// TestCheckInsidePath_AbsoluteOutsideButNotSystem covers the case
// where the user writes an absolute path that's outside both the
// project AND the system blocklist (e.g. /tmp/foo). It must still
// fail — the rule is "inside project", not "outside /etc".
func TestCheckInsidePath_AbsoluteOutsideButNotSystem(t *testing.T) {
	base := t.TempDir()
	err := checkInsidePath("/tmp/raioz-not-here", base, "test")
	if err == nil {
		t.Fatal("expected error for absolute path outside project")
	}
	var rerr *errors.RaiozError
	if !stderrors.As(err, &rerr) {
		t.Fatalf("expected *RaiozError, got %T", err)
	}
	if rerr.Context["system_dir"] != nil {
		t.Errorf("/tmp is not a system dir; expected nil, got %v", rerr.Context["system_dir"])
	}
	// Should be an "escapes_repo"-style error: path is in context, no system_dir set.
	if rerr.Context["path"] != "/tmp/raioz-not-here" {
		t.Errorf("expected path=%q in context, got %v", "/tmp/raioz-not-here", rerr.Context["path"])
	}
	if rerr.Context["resolved"] != "/tmp/raioz-not-here" {
		t.Errorf("expected resolved=%q in context, got %v", "/tmp/raioz-not-here", rerr.Context["resolved"])
	}
}

func TestCheckSystemBlocklist_AcceptsOutsideProject(t *testing.T) {
	base := t.TempDir()
	// Sibling project paths legitimately escape baseDir. They must
	// still NOT target system dirs.
	cases := []struct {
		path    string
		wantErr bool
	}{
		{"", false},
		{"../sibling-project", false},            // ok, outside base but not system
		{"/home/dev/projects/sibling", false},    // ok, absolute but not system
		{filepath.Join(base, "sibling"), false},  // ok, inside base
		{"/etc/raioz-sibling", true},             // bad: system dir
		{"/root/projects/sibling", true},         // bad: system dir
		{"../../../../../var/lib/sibling", true}, // resolves into /var/lib
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			err := checkSystemBlocklist(tc.path, base, "test")
			if (err != nil) != tc.wantErr {
				t.Errorf("checkSystemBlocklist(%q) err=%v, wantErr=%v", tc.path, err, tc.wantErr)
			}
		})
	}
}

func TestPathFromCommand(t *testing.T) {
	cases := []struct {
		cmd      string
		wantOK   bool
		wantPath string
	}{
		{"", false, ""},
		{"   ", false, ""},
		{"make build", false, ""},
		{"rustup", false, ""},
		{"./scripts/setup.sh", true, "./scripts/setup.sh"},
		{"./scripts/setup.sh --verbose", true, "./scripts/setup.sh"},
		{"   ./scripts/setup.sh   ", true, "./scripts/setup.sh"},
		{"/usr/local/bin/foo", true, "/usr/local/bin/foo"},
		{"/usr/local/bin/foo bar", true, "/usr/local/bin/foo"},
		{"../scripts/foo.sh", true, "../scripts/foo.sh"},
		{"bash ./scripts/foo.sh", false, ""}, // first token is "bash"
		{"cat ./foo | wc", false, ""},
		{"npm run dev", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.cmd, func(t *testing.T) {
			got, ok := pathFromCommand(tc.cmd)
			if ok != tc.wantOK {
				t.Errorf("pathFromCommand(%q): ok=%v, want %v", tc.cmd, ok, tc.wantOK)
			}
			if got != tc.wantPath {
				t.Errorf("pathFromCommand(%q): path=%q, want %q", tc.cmd, got, tc.wantPath)
			}
		})
	}
}

func TestValidatePathSafety_AcceptsCleanConfig(t *testing.T) {
	base := t.TempDir()
	cfg := &RaiozConfig{
		Project: "test",
		Services: map[string]YAMLService{
			"api": {
				Path:    "./api",
				Compose: []string{"./compose.api.yml"},
				Env:     []string{".env.api"},
				Command: "make dev",
				Stop:    "make stop",
			},
			"web": {
				Path:    "./web",
				Command: "./scripts/start-web.sh",
				Stop:    "./scripts/stop-web.sh --force",
			},
		},
		Deps: map[string]YAMLDependency{
			"postgres": {
				Image: "postgres:16",
				Env:   []string{".env.postgres"},
			},
			"adminer": {
				Compose: []string{"./infra/adminer.yml"},
			},
		},
		Pre:   []string{"./scripts/fetch-secrets.sh", "make seed-db"},
		PreUp: []string{"./scripts/migrate.sh"},
		Post:  []string{"rm -f .env.*.tmp", "./scripts/cleanup.sh --quiet"},
	}
	if err := validatePathSafety(cfg, base); err != nil {
		t.Errorf("expected clean config to pass, got: %v", err)
	}
}

func TestValidatePathSafety_RejectsByField(t *testing.T) {
	base := t.TempDir()
	cases := []struct {
		name      string
		cfg       *RaiozConfig
		wantField string
	}{
		{
			"service path escapes",
			&RaiozConfig{Services: map[string]YAMLService{
				"api": {Path: "../shared"},
			}},
			"services.api.path",
		},
		{
			"service env at /etc",
			&RaiozConfig{Services: map[string]YAMLService{
				"api": {Path: "./api", Env: []string{"/etc/raioz.env"}},
			}},
			"services.api.env[0]",
		},
		{
			"service compose escapes",
			&RaiozConfig{Services: map[string]YAMLService{
				"api": {Path: "./api", Compose: []string{"../../shared/compose.yml"}},
			}},
			"services.api.compose[0]",
		},
		{
			"service command path escapes",
			&RaiozConfig{Services: map[string]YAMLService{
				"api": {Path: "./api", Command: "../../scripts/start.sh"},
			}},
			"services.api.command",
		},
		{
			"service stop path in system dir",
			&RaiozConfig{Services: map[string]YAMLService{
				"api": {Path: "./api", Stop: "/etc/scripts/stop.sh"},
			}},
			"services.api.stop",
		},
		{
			"dep env escapes",
			&RaiozConfig{Deps: map[string]YAMLDependency{
				"pg": {Image: "postgres:16", Env: []string{"../../shared/.env"}},
			}},
			"dependencies.pg.env[0]",
		},
		{
			"dep compose at /etc",
			&RaiozConfig{Deps: map[string]YAMLDependency{
				"pg": {Compose: []string{"/etc/postgres/compose.yml"}},
			}},
			"dependencies.pg.compose[0]",
		},
		{
			"dep dev path escapes",
			&RaiozConfig{Deps: map[string]YAMLDependency{
				"pg": {Image: "postgres:16", Dev: &YAMLDevConfig{Path: "../../local-pg"}},
			}},
			"dependencies.pg.dev.path",
		},
		{
			"dep project in system dir",
			&RaiozConfig{Deps: map[string]YAMLDependency{
				"sib": {Project: "/etc/sibling"},
			}},
			"dependencies.sib.project",
		},
		{
			"dep siblingProject in system dir",
			&RaiozConfig{Deps: map[string]YAMLDependency{
				"sib": {SiblingProject: "/root/projects/sib", Image: "x:y"},
			}},
			"dependencies.sib.siblingProject",
		},
		{
			"pre path escapes",
			&RaiozConfig{Pre: []string{"../../etc/setup.sh"}},
			"pre[0]",
		},
		{
			"preUp absolute system",
			&RaiozConfig{PreUp: []string{"/etc/migrate.sh"}},
			"preUp[0]",
		},
		{
			"post path escapes",
			&RaiozConfig{Post: []string{"../../../tmp/clean.sh"}},
			"post[0]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePathSafety(tc.cfg, base)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			var rerr *errors.RaiozError
			if !stderrors.As(err, &rerr) {
				t.Fatalf("expected *RaiozError, got %T", err)
			}
			if rerr.Code != errors.ErrCodeUnsafePath {
				t.Errorf("expected UNSAFE_PATH, got %s", rerr.Code)
			}
			if rerr.Context["field"] != tc.wantField {
				t.Errorf("expected field=%q, got %v", tc.wantField, rerr.Context["field"])
			}
		})
	}
}

// TestValidatePathSafety_SiblingProjectOutsideOK confirms that sibling
// project paths legitimately escape baseDir (ADR-008) without being
// rejected by H2 — only the system blocklist applies.
func TestValidatePathSafety_SiblingProjectOutsideOK(t *testing.T) {
	base := t.TempDir()
	cfg := &RaiozConfig{
		Deps: map[string]YAMLDependency{
			"sib1": {Project: "../sibling-raioz-project"},
			"sib2": {SiblingProject: "../../other-sibling", Image: "fallback:1"},
		},
	}
	if err := validatePathSafety(cfg, base); err != nil {
		t.Errorf("sibling project paths should be allowed outside base, got: %v", err)
	}
}

// TestValidatePathSafety_WorkspaceRootWidensContainment confirms the
// opt-in escape hatch: declaring workspaceRoot moves the containment
// boundary so multi-repo sibling paths (../api) validate as contained,
// while the same paths are still rejected when no workspaceRoot is set.
func TestValidatePathSafety_WorkspaceRootWidensContainment(t *testing.T) {
	root := t.TempDir()                  // the workspace root (parent of repos)
	base := filepath.Join(root, "front") // raioz.yaml lives in front/

	// Baseline: without workspaceRoot, ../api escapes baseDir → rejected.
	strict := &RaiozConfig{
		Services: map[string]YAMLService{"api": {Path: "../api"}},
	}
	if err := validatePathSafety(strict, base); err == nil {
		t.Fatal("expected ../api to be rejected without workspaceRoot")
	}

	// With workspaceRoot: .. the sibling paths resolve inside root → ok.
	widened := &RaiozConfig{
		WorkspaceRoot: "..",
		Services: map[string]YAMLService{
			"api": {
				Path:    "../api",
				Compose: []string{"../api/docker-compose.yml"},
				Command: "../api/run.sh",
				Env:     []string{"../api/.env"},
			},
		},
		Deps: map[string]YAMLDependency{
			"db": {Compose: []string{"../api/db.yml"}},
		},
	}
	if err := validatePathSafety(widened, base); err != nil {
		t.Errorf("expected multi-repo paths to pass with workspaceRoot, got: %v", err)
	}
}

// TestValidatePathSafety_WorkspaceRootStillContains confirms the widened
// boundary is still HARD containment: a path that escapes even the
// declared root is rejected (not silently downgraded to blocklist-only).
func TestValidatePathSafety_WorkspaceRootStillContains(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "front")
	cfg := &RaiozConfig{
		WorkspaceRoot: "..",
		Services: map[string]YAMLService{
			"api": {Path: "../../outside-the-root"},
		},
	}
	err := validatePathSafety(cfg, base)
	if err == nil {
		t.Fatal("expected path escaping the widened root to be rejected")
	}
	var rerr *errors.RaiozError
	if !stderrors.As(err, &rerr) || rerr.Code != errors.ErrCodeUnsafePath {
		t.Fatalf("expected UNSAFE_PATH, got %v", err)
	}
}

// TestValidatePathSafety_WorkspaceRootSystemDirRejected confirms the
// declared root itself cannot be a system dir — otherwise it would
// re-open everything H2 closes.
func TestValidatePathSafety_WorkspaceRootSystemDirRejected(t *testing.T) {
	base := t.TempDir()
	cfg := &RaiozConfig{
		WorkspaceRoot: "/etc",
		Services:      map[string]YAMLService{"api": {Path: "./api"}},
	}
	err := validatePathSafety(cfg, base)
	if err == nil {
		t.Fatal("expected workspaceRoot=/etc to be rejected")
	}
	var rerr *errors.RaiozError
	if !stderrors.As(err, &rerr) {
		t.Fatalf("expected *RaiozError, got %T", err)
	}
	if rerr.Context["field"] != "workspaceRoot" {
		t.Errorf("expected field=workspaceRoot, got %v", rerr.Context["field"])
	}
}

// TestValidatePathSafety_WorkspaceRootStillBlocksSystemDirs confirms the
// system blocklist still applies to every path even when a workspaceRoot
// widens the containment boundary.
func TestValidatePathSafety_WorkspaceRootStillBlocksSystemDirs(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "front")
	cfg := &RaiozConfig{
		WorkspaceRoot: "..",
		Services: map[string]YAMLService{
			"api": {Path: "./api", Env: []string{"/etc/secret.env"}},
		},
	}
	err := validatePathSafety(cfg, base)
	if err == nil {
		t.Fatal("expected /etc env path to be blocked even with workspaceRoot")
	}
	var rerr *errors.RaiozError
	if !stderrors.As(err, &rerr) || rerr.Context["system_dir"] != "/etc" {
		t.Fatalf("expected system_dir=/etc, got %v", err)
	}
}

// TestValidatePathSafety_CommandWithBinaryArgNotValidated confirms the
// documented heuristic miss: a command whose first token is a binary
// (PATH-resolved) and whose argument happens to be a path is NOT
// validated. This is intentional — shell construction is the user's
// responsibility. Documented in path_safety.go's pathFromCommand.
func TestValidatePathSafety_CommandWithBinaryArgNotValidated(t *testing.T) {
	base := t.TempDir()
	cfg := &RaiozConfig{
		Services: map[string]YAMLService{
			"api": {
				Path:    "./api",
				Command: "bash ../../etc/dangerous.sh", // path arg, but first token is "bash"
			},
		},
	}
	if err := validatePathSafety(cfg, base); err != nil {
		t.Errorf("documented heuristic miss: expected no error for shell-style command, got: %v", err)
	}
}
