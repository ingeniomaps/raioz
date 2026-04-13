package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"raioz/internal/detect"
	"raioz/internal/output"
)

// AutoDetect scans a directory and generates a Deps config in memory
// without requiring a raioz.yaml file. This enables zero-config `raioz up`.
func AutoDetect(dir string) (*Deps, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve directory: %w", err)
	}

	projectName := filepath.Base(absDir)

	output.PrintInfo("No config file found — auto-detecting project structure...")
	fmt.Println()

	services := make(map[string]Service)
	infra := make(map[string]InfraEntry)

	// Scan subdirectories for services
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || isAutoDetectIgnored(entry.Name()) {
			continue
		}

		subPath := filepath.Join(absDir, entry.Name())
		result := detect.Detect(subPath)
		if result.Runtime == detect.RuntimeUnknown {
			continue
		}

		name := entry.Name()
		svc := Service{
			Source: SourceConfig{
				Kind: "local",
				Path: subPath,
			},
		}
		if result.HasHotReload {
			svc.Watch = YAMLWatch{Enabled: true, Mode: "native"}
		}

		services[name] = svc
		output.PrintInfo(fmt.Sprintf("  %s → %s (%s)", name, result.Runtime, result.StartCommand))
	}

	// If no subdirectory services, check root
	if len(services) == 0 {
		rootResult := detect.Detect(absDir)
		if rootResult.Runtime != detect.RuntimeUnknown {
			services[projectName] = Service{
				Source: SourceConfig{
					Kind: "local",
					Path: absDir,
				},
			}
			output.PrintInfo(fmt.Sprintf("  . → %s (%s)", rootResult.Runtime, rootResult.StartCommand))
		}
	}

	// Infer dependencies from .env files
	inferredDeps, inferredLinks := detect.InferDepsFromEnv(absDir)
	for _, dep := range inferredDeps {
		infraEntry := &Infra{
			Image: extractAutoImage(dep.Image),
			Tag:   extractAutoTag(dep.Image),
			Ports: []string{dep.Port},
		}
		// Auto-detect .env.{name} file (e.g., .env.postgres)
		envFile := ".env." + dep.Name
		if _, err := os.Stat(filepath.Join(absDir, envFile)); err == nil {
			infraEntry.Env = &EnvValue{Files: []string{envFile}}
		}
		infra[dep.Name] = InfraEntry{Inline: infraEntry}
		output.PrintInfo(fmt.Sprintf("  %s → %s (from %s)", dep.Name, dep.Image, dep.Source))
	}

	// Wire dependsOn from inferred links
	for _, link := range inferredLinks {
		if svc, ok := services[link.From]; ok {
			if !containsStr(svc.DependsOn, link.To) {
				svc.DependsOn = append(svc.DependsOn, link.To)
				services[link.From] = svc
			}
		}
	}

	if len(services) == 0 && len(infra) == 0 {
		return nil, fmt.Errorf(
			"no services or dependencies detected in %s.\n\n"+
				"  Raioz looks for: docker-compose.yml, Dockerfile, "+
				"package.json, go.mod, Makefile\n"+
				"  Create a raioz.yaml manually or add one of these files",
			absDir,
		)
	}

	fmt.Println()
	output.PrintInfo(fmt.Sprintf("Auto-detected %d services, %d dependencies", len(services), len(infra)))
	fmt.Println()

	return &Deps{
		SchemaVersion: "2.0",
		Project:       Project{Name: projectName},
		Network:       NetworkConfig{Name: projectName + "-net"},
		Services:      services,
		Infra:         infra,
		ProjectRoot:   absDir,
	}, nil
}

func isAutoDetectIgnored(name string) bool {
	ignored := map[string]bool{
		"node_modules": true, "vendor": true, ".git": true,
		"dist": true, "build": true, ".next": true, ".nuxt": true,
		"target": true, "bin": true, "__pycache__": true,
		"docs": true, "scripts": true, "examples": true,
	}
	return ignored[name]
}

func extractAutoImage(imageRef string) string {
	parts := strings.SplitN(imageRef, ":", 2)
	return parts[0]
}

func extractAutoTag(imageRef string) string {
	parts := strings.SplitN(imageRef, ":", 2)
	if len(parts) > 1 {
		return parts[1]
	}
	return "latest"
}

func containsStr(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
