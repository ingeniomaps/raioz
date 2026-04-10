package upcase

import (
	"context"

	"raioz/internal/config"
	"raioz/internal/detect"
	"raioz/internal/domain/interfaces"
	"raioz/internal/logging"
	"raioz/internal/output"
)

// DetectionMap maps service/dependency names to their detected runtime results.
type DetectionMap map[string]detect.DetectResult

// detectRuntimes scans all services and dependencies to determine their runtime type.
// For services: scans the path directory.
// For dependencies: returns RuntimeImage.
func detectRuntimes(ctx context.Context, deps *config.Deps) DetectionMap {
	results := make(DetectionMap)

	// Detect services
	for name, svc := range deps.Services {
		path := svc.Source.Path
		if path == "" {
			continue
		}

		result := detect.Detect(path)
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
	detection detect.DetectResult,
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
func isYAMLMode(deps *config.Deps) bool {
	return deps.SchemaVersion == "2.0"
}
