package cli

import (
	"os"
	"testing"

	"raioz/internal/config"
)

func TestMigrateCmd(t *testing.T) {
	if migrateCmd == nil {
		t.Fatal("migrateCmd should be initialized")
	}
	if migrateCmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestMigrateCmdFlags(t *testing.T) {
	flags := []struct {
		name      string
		shorthand string
	}{
		{"compose", "c"},
		{"output", "o"},
		{"project", "p"},
		{"network", ""},
	}

	for _, tt := range flags {
		t.Run(tt.name, func(t *testing.T) {
			f := migrateCmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("flag %q not registered", tt.name)
			}
			if tt.shorthand != "" && f.Shorthand != tt.shorthand {
				t.Errorf("shorthand = %s, want %s", f.Shorthand, tt.shorthand)
			}
		})
	}
}

func TestMigrateCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "migrate" {
			found = true
			break
		}
	}
	if !found {
		t.Error("migrateCmd not registered on rootCmd")
	}
}

// migrate writes a raioz.yaml the loader accepts, and never replaces an
// existing one unasked.
func TestMigrateCmd_WritesLoadableYAML(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	compose := `
services:
  back:
    build: ./back
    environment:
      DATABASE_URL: postgres://db
    depends_on: [postgres]
  postgres:
    image: postgres:16
    ports: ["5432:5432"]
`
	if err := os.WriteFile("docker-compose.yml", []byte(compose), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("back", 0o755); err != nil {
		t.Fatal(err)
	}

	prevC, prevO, prevP, prevF := migrateComposePath, migrateOutputPath, migrateProjectName, migrateForce
	t.Cleanup(func() {
		migrateComposePath, migrateOutputPath, migrateProjectName, migrateForce = prevC, prevO, prevP, prevF
	})
	migrateComposePath, migrateOutputPath, migrateProjectName, migrateForce = "docker-compose.yml", "raioz.yaml", "shop", false

	if err := migrateCmd.RunE(migrateCmd, nil); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	deps, _, err := config.LoadDepsFromYAML("raioz.yaml")
	if err != nil {
		t.Fatalf("the generated raioz.yaml does not load: %v", err)
	}
	if _, ok := deps.Services["back"]; !ok {
		t.Errorf("back (build:) should be a service: %v", deps.Services)
	}
	if _, ok := deps.Infra["postgres"]; !ok {
		t.Errorf("postgres (image:) should be a dependency: %v", deps.Infra)
	}

	if err := os.WriteFile("raioz.yaml", []byte("# hand edited\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := migrateCmd.RunE(migrateCmd, nil); err == nil {
		t.Error("migrate replaced an existing raioz.yaml without --force")
	}
	if data, _ := os.ReadFile("raioz.yaml"); string(data) != "# hand edited\n" {
		t.Errorf("existing file was modified: %q", data)
	}
}
