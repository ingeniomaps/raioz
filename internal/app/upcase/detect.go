package upcase

import (
	"context"

	"raioz/internal/config"
	"raioz/internal/detect"
	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// DetectionMap maps service/dependency names to their detected runtime results.
type DetectionMap map[string]models.DetectResult

// BuildDetectionMap is the read-only equivalent of detectRuntimes exported for
// callers outside the upcase package (notably `raioz check`) that need the
// runtime classification to feed into the port allocator without paying for
// the progress output and logging the full up flow emits.
func BuildDetectionMap(deps *models.Deps) DetectionMap {
	results := make(DetectionMap)

	for name, svc := range deps.Services {
		// Same precedence as the up flow: yaml `command:`/`compose:` override
		// directory auto-detection. Without this the check would report a
		// different runtime than what up actually uses.
		path := svc.Source.Path
		if path == "" && svc.Source.Command == "" && len(svc.Source.ComposeFiles) == 0 {
			continue
		}
		results[name] = config.ResolveServiceDetection(svc, path)
	}

	for name, entry := range deps.Infra {
		var imageRef string
		if entry.Inline != nil {
			imageRef = entry.Inline.Image
			if entry.Inline.Tag != "" {
				imageRef += ":" + entry.Inline.Tag
			}
		}
		results[name] = detect.ForImage(imageRef)
	}

	return results
}

// detectRuntimes scans all services and dependencies to determine their runtime type.
// For services: scans the path directory OR honors explicit overrides from raioz.yaml
// (source.command, source.composeFiles).
// For dependencies: returns RuntimeImage.
func detectRuntimes(ctx context.Context, deps *models.Deps) DetectionMap {
	results := make(DetectionMap)

	// Detect services
	for name, svc := range deps.Services {
		path := svc.Source.Path
		if path == "" && svc.Source.Command == "" && len(svc.Source.ComposeFiles) == 0 {
			continue
		}

		result := config.ResolveServiceDetection(svc, path)
		results[name] = result

		logging.DebugWithContext(ctx, "Detected runtime",
			"service", name, "runtime", string(result.Runtime),
			"path", path, "command", result.StartCommand)

		output.PrintInfo(name + " -> " + string(result.Runtime))
	}

	// Dependencies are always images
	for name, entry := range deps.Infra {
		var imageRef string
		if entry.Inline != nil {
			imageRef = entry.Inline.Image
			if entry.Inline.Tag != "" {
				imageRef += ":" + entry.Inline.Tag
			}
		}

		result := detect.ForImage(imageRef)
		results[name] = result

		logging.DebugWithContext(ctx, "Dependency runtime",
			"name", name, "runtime", string(result.Runtime), "image", imageRef)
	}

	return results
}

// buildServiceContext creates a ServiceContext for orchestration from config and detection.
func buildServiceContext(
	name string,
	detection models.DetectResult,
	networkName string,
	envVars map[string]string,
	ports []string,
	dependsOn []string,
	containerName string,
	path string,
	projectName string,
) interfaces.ServiceContext {
	return interfaces.ServiceContext{
		Name:          name,
		Path:          path,
		Detection:     detection,
		NetworkName:   networkName,
		EnvVars:       envVars,
		Ports:         ports,
		DependsOn:     dependsOn,
		ContainerName: containerName,
		ProjectName:   projectName,
	}
}

// isYAMLMode returns true if the current config was loaded from a raioz.yaml file.
// Reads the canonical SourceFormat discriminator (ADR-039); inline
// `deps.SchemaVersion == "2.0"` reads elsewhere are kept until issue 069
// collapses them through SelectFlow.
func isYAMLMode(deps *models.Deps) bool {
	return deps != nil && deps.SourceFormat == models.SourceFormatYAML
}
