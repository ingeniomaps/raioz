package workspace

import (
	"path/filepath"

	"raioz/internal/config"
	"raioz/internal/git"
)

// GetServiceDir returns the correct directory for a service based on its access mode
// For readonly services, returns ReadonlyServicesDir
// For editable services, returns LocalServicesDir
// Falls back to ServicesDir for backward compatibility
func GetServiceDir(ws *Workspace, svc config.Service) string {
	if svc.Source.Kind == "git" {
		if git.IsReadonly(svc.Source) {
			return ws.ReadonlyServicesDir
		}
		return ws.LocalServicesDir
	}
	// For image services, use the legacy ServicesDir (not used for git repos)
	return ws.ServicesDir
}

// GetServicePath returns the full path to a service directory
// serviceName is used for context (useful for logging, etc.)
// If Source.Path is an absolute path (from override), it's used as-is
// Otherwise, it's resolved relative to the workspace directory
func GetServicePath(ws *Workspace, serviceName string, svc config.Service) string {
	// If Source.Path is already absolute (from override), use it directly
	if filepath.IsAbs(svc.Source.Path) {
		return svc.Source.Path
	}

	// No override, use workspace path
	baseDir := GetServiceDir(ws, svc)
	// Use Source.Path as path component
	pathComponent := svc.Source.Path
	if pathComponent == "" {
		// Fallback to service name (shouldn't happen, but safety check)
		pathComponent = serviceName
	}
	return filepath.Join(baseDir, pathComponent)
}
