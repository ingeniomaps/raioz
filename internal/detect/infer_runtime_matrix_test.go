package detect

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/domain/models"
)

// touch creates an empty file (and its parents) inside dir. Detection
// keys off which files are present, so that is all a fixture needs.
func touch(t *testing.T, dir, rel string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

// This is what a user gets when they declare a service with only a
// `path:` — raioz infers the command and the port. A wrong inference
// means the service silently never starts, or starts on a port the proxy
// is not routing to.
func TestInferCommandsPerRuntime(t *testing.T) {
	cases := []struct {
		name     string
		rt       models.Runtime
		files    []string
		wantCmd  string
		wantPort int
		wantHot  bool
	}{
		{name: "go", rt: models.RuntimeGo, wantCmd: "go run ."},
		{name: "python", rt: models.RuntimePython, wantCmd: "python -m flask run"},
		{name: "rust", rt: models.RuntimeRust, wantCmd: "cargo run"},
		{name: "just", rt: models.RuntimeJust, wantCmd: "just dev"},
		{name: "task", rt: models.RuntimeTask, wantCmd: "task dev"},
		{name: "php", rt: models.RuntimePHP, wantCmd: "php -S 0.0.0.0:8000 -t public", wantPort: 8000},
		{name: "java maven", rt: models.RuntimeJava, wantCmd: "./mvnw spring-boot:run", wantPort: 8080},
		{name: "dotnet", rt: models.RuntimeDotnet, wantCmd: "dotnet watch run", wantPort: 5000, wantHot: true},
		{name: "ruby plain", rt: models.RuntimeRuby, wantCmd: "bundle exec ruby app.rb", wantPort: 3000},
		{name: "elixir", rt: models.RuntimeElixir, wantCmd: "mix phx.server", wantPort: 4000, wantHot: true},
		{name: "dart", rt: models.RuntimeDart, wantCmd: "dart run", wantPort: 8080},
		{name: "swift", rt: models.RuntimeSwift, wantCmd: "swift run", wantPort: 8080},
		{name: "scala", rt: models.RuntimeScala, wantCmd: "sbt run", wantPort: 9000},
		{name: "clojure deps", rt: models.RuntimeClojure, wantCmd: "clj -M:dev", wantPort: 3000},
		{name: "zig", rt: models.RuntimeZig, wantCmd: "zig build run", wantPort: 8080},
		{name: "gleam", rt: models.RuntimeGleam, wantCmd: "gleam run", wantPort: 8080},
		{name: "haskell cabal", rt: models.RuntimeHaskell, wantCmd: "cabal run", wantPort: 3000},
		{name: "deno", rt: models.RuntimeDeno, wantCmd: "deno task dev", wantPort: 8000, wantHot: true},
		{name: "bun", rt: models.RuntimeBun, wantCmd: "bun run dev", wantPort: 3000, wantHot: true},

		// Files in the directory change the answer for several runtimes.
		{
			name: "go with air reloads itself", rt: models.RuntimeGo,
			files: []string{".air.toml"}, wantCmd: "go run .", wantHot: true,
		},
		{
			name: "php with artisan is laravel", rt: models.RuntimePHP,
			files:   []string{"artisan"},
			wantCmd: "php artisan serve --host=0.0.0.0", wantPort: 8000,
		},
		{
			name: "java with gradlew", rt: models.RuntimeJava,
			files: []string{"gradlew"}, wantCmd: "./gradlew bootRun", wantPort: 8080,
		},
		{
			name: "ruby with rails", rt: models.RuntimeRuby,
			files: []string{"bin/rails"}, wantCmd: "bundle exec rails server", wantPort: 3000,
		},
		{
			name: "clojure with lein only", rt: models.RuntimeClojure,
			files: []string{"project.clj"}, wantCmd: "lein run", wantPort: 3000,
		},
		{
			name: "haskell with stack", rt: models.RuntimeHaskell,
			files: []string{"stack.yaml"}, wantCmd: "stack run", wantPort: 3000,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tc.files {
				touch(t, dir, f)
			}

			got := &models.DetectResult{Runtime: tc.rt}
			InferCommandsForRuntime(got, dir, tc.rt)

			if got.StartCommand != tc.wantCmd {
				t.Errorf("StartCommand = %q, want %q", got.StartCommand, tc.wantCmd)
			}
			if got.Port != tc.wantPort {
				t.Errorf("Port = %d, want %d", got.Port, tc.wantPort)
			}
			if got.HasHotReload != tc.wantHot {
				t.Errorf("HasHotReload = %v, want %v", got.HasHotReload, tc.wantHot)
			}
		})
	}
}

// A port the user (or an earlier detection step) already established must
// win — overwriting it would send the proxy to the wrong place.
func TestInferCommandsKeepsAnExistingPort(t *testing.T) {
	got := &models.DetectResult{Runtime: models.RuntimeJava, Port: 9999}
	InferCommandsForRuntime(got, t.TempDir(), models.RuntimeJava)
	if got.Port != 9999 {
		t.Errorf("Port = %d, want the already-known 9999", got.Port)
	}
}

func TestInferCommandsNilResult(t *testing.T) {
	// Callers pass whatever detection produced; a nil must not panic.
	InferCommandsForRuntime(nil, t.TempDir(), models.RuntimeGo)
}

// Running `npm run dev` in a bun or pnpm project either fails or silently
// installs a second lockfile, so the inferred command follows whichever
// lockfile is on disk.
func TestNodeLockfilePreference(t *testing.T) {
	cases := []struct {
		name    string
		files   []string
		wantCmd string
	}{
		{name: "no lockfile stays on npm", wantCmd: "npm run dev"},
		{name: "bun.lockb", files: []string{"bun.lockb"}, wantCmd: "bun run dev"},
		{name: "bunfig.toml", files: []string{"bunfig.toml"}, wantCmd: "bun run dev"},
		{name: "pnpm-lock.yaml", files: []string{"pnpm-lock.yaml"}, wantCmd: "pnpm run dev"},
		{name: "yarn.lock", files: []string{"yarn.lock"}, wantCmd: "yarn run dev"},
		{
			// bun is checked first, so a project carrying both keeps bun.
			name:  "bun wins over yarn",
			files: []string{"bun.lockb", "yarn.lock"}, wantCmd: "bun run dev",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tc.files {
				touch(t, dir, f)
			}

			got := &models.DetectResult{
				Runtime:      models.RuntimeNPM,
				StartCommand: "npm run dev",
				DevCommand:   "npm run dev",
			}
			applyNodeLockfilePreference(got, dir)

			if got.StartCommand != tc.wantCmd {
				t.Errorf("StartCommand = %q, want %q", got.StartCommand, tc.wantCmd)
			}
			if got.DevCommand != tc.wantCmd {
				t.Errorf("DevCommand = %q, want %q", got.DevCommand, tc.wantCmd)
			}
		})
	}
}

// A command that never mentioned npm (a Makefile target, say) must pass
// through untouched — the swap is a prefix rewrite, not a guess.
func TestNodeLockfilePreferenceLeavesForeignCommands(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "pnpm-lock.yaml")

	got := &models.DetectResult{StartCommand: "make dev", DevCommand: "make dev"}
	applyNodeLockfilePreference(got, dir)

	if got.StartCommand != "make dev" {
		t.Errorf("StartCommand = %q, want it untouched", got.StartCommand)
	}
}
