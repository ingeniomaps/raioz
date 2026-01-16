package workspace

import (
	"fmt"
	"os"
	"path/filepath"

	"raioz/internal/config"
	"raioz/internal/logging"
	pathvalidate "raioz/internal/path"
)

// MigrateLegacyServices migrates services from the old structure to the new structure
// Old structure: {base}/services/{path}
// New structure: {base}/workspaces/{project}/local/{path} or readonly/{path}
//
// This function:
// 1. Checks if services exist in the old location
// 2. Moves them to the appropriate new location based on access mode
// 3. Only moves if the service doesn't already exist in the new location
func MigrateLegacyServices(ws *Workspace, deps *config.Deps) error {
	if deps == nil {
		return nil
	}

	var migrated []string
	var errors []string

	for name, svc := range deps.Services {
		if svc.Source.Kind != "git" {
			continue // Only migrate git services
		}

		// Determine target path based on access mode
		targetPath := GetServicePath(ws, name, svc)

		// Old location - validate path to prevent path traversal
		oldPath, err := pathvalidate.EnsurePathInBase(ws.ServicesDir, svc.Source.Path)
		if err != nil {
			errors = append(errors, fmt.Sprintf("invalid path for %s: %v", name, err))
			continue
		}

		// Check if service exists in old location
		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			continue // Service doesn't exist in old location, skip
		}

		// Check if service already exists in new location
		if _, err := os.Stat(targetPath); err == nil {
			// Service already exists in new location, skip migration
			continue
		}

		// Create target directory if it doesn't exist (use 0700 for security - owner only)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
			errors = append(errors, fmt.Sprintf("failed to create target directory for %s: %v", name, err))
			continue
		}

		// Move service from old location to new location
		if err := os.Rename(oldPath, targetPath); err != nil {
			errors = append(errors, fmt.Sprintf("failed to migrate %s: %v", name, err))
			continue
		}

		migrated = append(migrated, name)
	}

	// Report results
	if len(migrated) > 0 {
		logging.Info("Migrated services to new structure", "count", len(migrated), "services", migrated)
	}

	if len(errors) > 0 {
		return fmt.Errorf("migration errors: %v", errors)
	}

	return nil
}

// CheckAndMigrateLegacyServices checks if migration is needed and performs it
// This is called automatically during workspace resolution
func CheckAndMigrateLegacyServices(ws *Workspace, deps *config.Deps) error {
	// Only migrate if we have services and the old ServicesDir exists
	if deps == nil || len(deps.Services) == 0 {
		return nil
	}

	// Check if old ServicesDir has any git services
	hasLegacyServices := false
	for _, svc := range deps.Services {
		if svc.Source.Kind == "git" {
			// Validate path to prevent path traversal
			oldPath, err := pathvalidate.EnsurePathInBase(ws.ServicesDir, svc.Source.Path)
			if err != nil {
				// Invalid path, skip (will be caught during migration)
				continue
			}
			if _, err := os.Stat(oldPath); err == nil {
				hasLegacyServices = true
				break
			}
		}
	}

	if !hasLegacyServices {
		return nil // No legacy services to migrate
	}

	// Perform migration
	return MigrateLegacyServices(ws, deps)
}
