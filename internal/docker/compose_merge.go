package docker

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// mergeExternalComposeFile loads a YAML file and merges its services, volumes, and networks into
// compose, applying the same normalization as inline infra: container names (workspace/project prefix),
// resolved and normalized volumes, and default network.
func mergeExternalComposeFile(
	compose map[string]any,
	projectDir, infraFilePath string,
	workspaceName, projectName, networkName string,
	hasExplicitWorkspace bool,
	infraKey string,
	infraVolumeMap map[string]string,
) ([]string, error) {
	path := infraFilePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(projectDir, infraFilePath)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read infra file %s: %w", path, err)
	}
	var external map[string]any
	if err := yaml.Unmarshal(data, &external); err != nil {
		return nil, fmt.Errorf("failed to parse infra YAML %s: %w", path, err)
	}

	// Collect named volumes from top-level volumes and from each service for normalization
	volumeNamesToNormalize := make(map[string]bool)
	if volumes, ok := external["volumes"].(map[string]any); ok {
		for name := range volumes {
			volumeNamesToNormalize[name] = true
		}
	}
	if services, ok := external["services"].(map[string]any); ok {
		for _, svc := range services {
			svcMap, _ := svc.(map[string]any)
			volStrs := volumeStringsFromServiceConfig(svcMap)
			for _, v := range volStrs {
				info, err := ParseVolume(v)
				if err != nil {
					continue
				}
				if info.Type == VolumeTypeNamed && info.Source != "" {
					volumeNamesToNormalize[info.Source] = true
				}
			}
		}
	}
	for name := range volumeNamesToNormalize {
		if _, ok := infraVolumeMap[name]; ok {
			continue
		}
		normalized, err := NormalizeVolumeName(workspaceName, name)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize volume name %q from infra file: %w", name, err)
		}
		infraVolumeMap[name] = normalized
	}

	// Merge volumes with normalized keys
	if volumes, ok := external["volumes"].(map[string]any); ok {
		dst, _ := compose["volumes"].(map[string]any)
		if dst == nil {
			dst = make(map[string]any)
			compose["volumes"] = dst
		}
		for name, vol := range volumes {
			normName := infraVolumeMap[name]
			if normName == "" {
				normName, _ = NormalizeVolumeName(workspaceName, name)
				infraVolumeMap[name] = normName
			}
			dst[normName] = vol
		}
	}

	var externalNames []string
	if services, ok := external["services"].(map[string]any); ok {
		dst := compose["services"].(map[string]any)
		for svcName, svc := range services {
			svcMap, ok := svc.(map[string]any)
			if !ok {
				dst[svcName] = svc
				externalNames = append(externalNames, svcName)
				continue
			}
			// Clone so we don't mutate the original
			config := make(map[string]any)
			for k, v := range svcMap {
				config[k] = v
			}

			// Normalize container name (same as inline: workspace/project + infra key + service)
			infraID := infraKey + "_" + svcName
			containerName, err := NormalizeInfraName(workspaceName, infraID, projectName, hasExplicitWorkspace)
			if err != nil {
				return nil, fmt.Errorf("failed to normalize container name for infra %s/%s: %w", infraKey, svcName, err)
			}
			config["container_name"] = containerName

			// Resolve and normalize volumes
			volStrs := volumeStringsFromServiceConfig(svcMap)
			if len(volStrs) > 0 {
				resolved, err := ResolveRelativeVolumes(volStrs, projectDir)
				if err != nil {
					return nil, fmt.Errorf("failed to resolve relative volumes for infra %s/%s: %w", infraKey, svcName, err)
				}
				normalized, err := NormalizeVolumeNamesInStrings(resolved, workspaceName, infraVolumeMap)
				if err != nil {
					return nil, fmt.Errorf("failed to normalize volume names for infra %s/%s: %w", infraKey, svcName, err)
				}
				config["volumes"] = normalized
			}

			// Use same default network as inline infra
			config["networks"] = []string{networkName}

			dst[svcName] = config
			externalNames = append(externalNames, svcName)
		}
	}

	if networks, ok := external["networks"].(map[string]any); ok {
		dst := compose["networks"].(map[string]any)
		if dst == nil {
			dst = make(map[string]any)
			compose["networks"] = dst
		}
		for name, net := range networks {
			dst[name] = net
		}
	}
	return externalNames, nil
}

// volumeStringsFromServiceConfig extracts volume mount strings from a service config (YAML-decoded).
// Supports "volumes" as []interface{} of strings. Long-form volumes are ignored.
func volumeStringsFromServiceConfig(svc map[string]any) []string {
	if svc == nil {
		return nil
	}
	raw, ok := svc["volumes"]
	if !ok {
		return nil
	}
	list, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	var out []string
	for _, item := range list {
		s, _ := item.(string)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
