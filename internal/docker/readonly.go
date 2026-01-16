package docker

import (
	"strings"

	"raioz/internal/config"
	"raioz/internal/git"
)

// GetVolumeMountMode returns ":ro" for readonly services, empty string otherwise
func GetVolumeMountMode(svc config.Service) string {
	if svc.Source.Kind == "git" && git.IsReadonly(svc.Source) {
		return ":ro"
	}
	return ""
}

// ApplyReadonlyToVolumes adds :ro suffix to bind mount volumes for readonly services
func ApplyReadonlyToVolumes(volumes []string, svc config.Service) []string {
	if len(volumes) == 0 {
		return volumes
	}

	// Only apply to readonly git services
	if svc.Source.Kind != "git" || !git.IsReadonly(svc.Source) {
		return volumes
	}

	result := make([]string, 0, len(volumes))
	for _, vol := range volumes {
		// Parse volume to determine type
		volInfo, err := ParseVolume(vol)
		if err != nil {
			// If parsing fails, keep as is (conservative)
			result = append(result, vol)
			continue
		}

		// Only add :ro to bind mounts, not named or anonymous volumes
		if volInfo.Type != VolumeTypeBind {
			result = append(result, vol)
			continue
		}

		// It's a bind mount, check if already has :ro or :rw
		if strings.HasSuffix(vol, ":ro") {
			// Already readonly, keep as is
			result = append(result, vol)
		} else if strings.HasSuffix(vol, ":rw") {
			// Has explicit :rw, remove it and add :ro (readonly takes precedence)
			vol = strings.TrimSuffix(vol, ":rw") + ":ro"
			result = append(result, vol)
		} else {
			// Bind mount without mode, add :ro
			result = append(result, vol+":ro")
		}
	}

	return result
}
