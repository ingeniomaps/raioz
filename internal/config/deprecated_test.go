package config

import (
	"os"
	"testing"
)

func TestCheckDeprecatedFields(t *testing.T) {
	tests := []struct {
		name     string
		jsonData string
		wantWarn int // Expected number of warnings
	}{
		{
			name: "no deprecated fields",
			jsonData: `{
				"schemaVersion": "1.0",
				"project": {
					"name": "test",
					"network": "test"
				},
				"services": {},
				"infra": {},
				"env": {
					"useGlobal": true,
					"files": []
				}
			}`,
			wantWarn: 0,
		},
		{
			name: "namespace in project",
			jsonData: `{
				"schemaVersion": "1.0",
				"project": {
					"name": "test",
					"namespace": "deprecated",
					"network": "test"
				},
				"services": {},
				"infra": {},
				"env": {
					"useGlobal": true,
					"files": []
				}
			}`,
			wantWarn: 1,
		},
		{
			name: "type in service",
			jsonData: `{
				"schemaVersion": "1.0",
				"project": {
					"name": "test",
					"network": "test"
				},
				"services": {
					"test": {
						"type": "microservice",
						"source": {
							"kind": "image",
							"image": "test/image",
							"tag": "latest"
						},
						"docker": {
							"mode": "dev"
						}
					}
				},
				"infra": {},
				"env": {
					"useGlobal": true,
					"files": []
				}
			}`,
			wantWarn: 1,
		},
		{
			name: "type in infra",
			jsonData: `{
				"schemaVersion": "1.0",
				"project": {
					"name": "test",
					"network": "test"
				},
				"services": {},
				"infra": {
					"postgres": {
						"type": "database",
						"image": "postgres",
						"tag": "15"
					}
				},
				"env": {
					"useGlobal": true,
					"files": []
				}
			}`,
			wantWarn: 1,
		},
		{
			name: "all deprecated fields",
			jsonData: `{
				"schemaVersion": "1.0",
				"project": {
					"name": "test",
					"namespace": "deprecated",
					"network": "test"
				},
				"services": {
					"svc1": {
						"type": "microservice",
						"source": {
							"kind": "image",
							"image": "test/image",
							"tag": "latest"
						},
						"docker": {
							"mode": "dev"
						}
					}
				},
				"infra": {
					"db": {
						"type": "database",
						"image": "postgres",
						"tag": "15"
					}
				},
				"env": {
					"useGlobal": true,
					"files": []
				}
			}`,
			wantWarn: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings, err := CheckDeprecatedFields([]byte(tt.jsonData))
			if err != nil {
				t.Fatalf("CheckDeprecatedFields() error = %v", err)
			}

			if len(warnings) != tt.wantWarn {
				t.Errorf(
					"CheckDeprecatedFields() warnings = %d, want %d. Warnings: %v",
					len(warnings), tt.wantWarn, warnings,
				)
			}
		})
	}
}

func TestLoadDepsWithDeprecatedFields(t *testing.T) {
	content := `{
		"schemaVersion": "1.0",
		"project": {
			"name": "test-project",
			"namespace": "deprecated-namespace",
			"network": "test-network"
		},
		"services": {
			"test-service": {
				"type": "microservice",
				"source": {
					"kind": "git",
					"repo": "git@github.com:test/repo.git",
					"branch": "main",
					"path": "services/test"
				},
				"docker": {
					"mode": "dev",
					"dockerfile": "Dockerfile"
				}
			}
		},
		"infra": {
			"postgres": {
				"type": "database",
				"image": "postgres",
				"tag": "15"
			}
		},
		"env": {
			"useGlobal": true,
			"files": []
		}
	}`

	tmpfile, err := os.CreateTemp("", "deps*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	deps, warnings, err := LoadDeps(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadDeps() error = %v", err)
	}

	if deps == nil {
		t.Fatal("LoadDeps() returned nil deps")
	}

	// Should have warnings for deprecated fields
	if len(warnings) < 3 {
		t.Errorf(
			"LoadDeps() warnings = %d, want at least 3. Warnings: %v",
			len(warnings), warnings,
		)
	}

	// Verify deps still loaded correctly despite deprecated fields
	if deps.Project.Name != "test-project" {
		t.Errorf("Expected project name 'test-project', got %s", deps.Project.Name)
	}

	if len(deps.Services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(deps.Services))
	}

	if len(deps.Infra) != 1 {
		t.Errorf("Expected 1 infra, got %d", len(deps.Infra))
	}
}
