package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetect_AllRuntimes(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string // filename → content
		expected Runtime
		command  string // substring of expected start command
	}{
		{
			name:     "Java/Maven",
			files:    map[string]string{"pom.xml": "<project></project>"},
			expected: RuntimeJava,
			command:  "mvnw",
		},
		{
			name:     "Java/Gradle",
			files:    map[string]string{"build.gradle": "plugins {}"},
			expected: RuntimeJava,
			command:  "gradlew",
		},
		{
			name:     "Java/Gradle Kotlin",
			files:    map[string]string{"build.gradle.kts": "plugins {}"},
			expected: RuntimeJava,
			command:  "gradlew",
		},
		{
			name:     "C#/.NET csproj",
			files:    map[string]string{"MyApp.csproj": "<Project></Project>"},
			expected: RuntimeDotnet,
			command:  "dotnet",
		},
		{
			name:     "Ruby/Gemfile",
			files:    map[string]string{"Gemfile": "source 'https://rubygems.org'"},
			expected: RuntimeRuby,
			command:  "bundle",
		},
		{
			name:     "Ruby/Rails",
			files:    map[string]string{"Gemfile": "gem 'rails'", "config/routes.rb": "Rails.application.routes.draw {}"},
			expected: RuntimeRuby,
			command:  "rails server",
		},
		{
			name:     "Elixir",
			files:    map[string]string{"mix.exs": "defmodule MyApp do end"},
			expected: RuntimeElixir,
			command:  "phx.server",
		},
		{
			name:     "Dart",
			files:    map[string]string{"pubspec.yaml": "name: myapp"},
			expected: RuntimeDart,
			command:  "dart run",
		},
		{
			name:     "Swift",
			files:    map[string]string{"Package.swift": "import PackageDescription"},
			expected: RuntimeSwift,
			command:  "swift run",
		},
		{
			name:     "Scala",
			files:    map[string]string{"build.sbt": "name := \"myapp\""},
			expected: RuntimeScala,
			command:  "sbt run",
		},
		{
			name:     "Clojure/deps.edn",
			files:    map[string]string{"deps.edn": "{:deps {}}"},
			expected: RuntimeClojure,
			command:  "clj",
		},
		{
			name:     "Clojure/Leiningen",
			files:    map[string]string{"project.clj": "(defproject myapp)"},
			expected: RuntimeClojure,
			command:  "lein run",
		},
		{
			name:     "Zig",
			files:    map[string]string{"build.zig": "const std = @import(\"std\");"},
			expected: RuntimeZig,
			command:  "zig build run",
		},
		{
			name:     "Gleam",
			files:    map[string]string{"gleam.toml": "name = \"myapp\""},
			expected: RuntimeGleam,
			command:  "gleam run",
		},
		{
			name:     "Haskell/Stack",
			files:    map[string]string{"stack.yaml": "resolver: lts-21"},
			expected: RuntimeHaskell,
			command:  "stack run",
		},
		{
			name:     "Haskell/Cabal",
			files:    map[string]string{"myapp.cabal": "name: myapp"},
			expected: RuntimeHaskell,
			command:  "cabal run",
		},
		{
			name:     "Deno",
			files:    map[string]string{"deno.json": "{}"},
			expected: RuntimeDeno,
			command:  "deno task dev",
		},
		{
			name:     "Bun",
			files:    map[string]string{"bunfig.toml": "[install]"},
			expected: RuntimeBun,
			command:  "bun run dev",
		},
		{
			name:     "Just",
			files:    map[string]string{"justfile": "dev:\n\techo hello"},
			expected: RuntimeJust,
			command:  "just dev",
		},
		{
			name:     "Task",
			files:    map[string]string{"Taskfile.yml": "version: 3"},
			expected: RuntimeTask,
			command:  "task dev",
		},
		{
			name:     "Node.js/yarn",
			files:    map[string]string{"package.json": `{"scripts":{"dev":"next dev"}}`, "yarn.lock": ""},
			expected: RuntimeNPM,
			command:  "yarn",
		},
		{
			name:     "Node.js/pnpm",
			files:    map[string]string{"package.json": `{"scripts":{"dev":"vite"}}`, "pnpm-lock.yaml": ""},
			expected: RuntimeNPM,
			command:  "pnpm",
		},
		{
			name:     "Node.js/bun lockfile",
			files:    map[string]string{"package.json": `{"scripts":{"dev":"next dev"}}`, "bun.lockb": ""},
			expected: RuntimeNPM,
			command:  "bun",
		},
		// Existing runtimes (regression tests)
		{
			name:     "Go",
			files:    map[string]string{"go.mod": "module example.com/app\ngo 1.22"},
			expected: RuntimeGo,
			command:  "go run",
		},
		{
			name:     "PHP",
			files:    map[string]string{"composer.json": "{}"},
			expected: RuntimePHP,
			command:  "php",
		},
		{
			name:     "Python",
			files:    map[string]string{"pyproject.toml": "[build-system]"},
			expected: RuntimePython,
		},
		{
			name:     "Rust",
			files:    map[string]string{"Cargo.toml": "[package]"},
			expected: RuntimeRust,
			command:  "cargo run",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			// Create files
			for name, content := range tt.files {
				fullPath := filepath.Join(dir, name)
				os.MkdirAll(filepath.Dir(fullPath), 0755)
				os.WriteFile(fullPath, []byte(content), 0644)
			}

			result := Detect(dir)

			if result.Runtime != tt.expected {
				t.Errorf("expected runtime %s, got %s", tt.expected, result.Runtime)
			}
			if tt.command != "" && !contains(result.StartCommand, tt.command) &&
				!contains(result.DevCommand, tt.command) {
				t.Errorf("expected command containing %q, got start=%q dev=%q",
					tt.command, result.StartCommand, result.DevCommand)
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
