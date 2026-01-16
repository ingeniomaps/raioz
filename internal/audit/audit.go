package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"time"
)

const auditLogFileName = "audit.log"

// EventType represents the type of audit event
type EventType string

const (
	EventTypeDependencyAdded    EventType = "dependency_added"
	EventTypeOverrideApplied    EventType = "override_applied"
	EventTypeOverrideReverted   EventType = "override_reverted"
	EventTypeConfigChanged      EventType = "config_changed"
	EventTypeConflictResolved   EventType = "conflict_resolved"
	EventTypeWorkspaceChanged   EventType = "workspace_changed"
	EventTypeServiceAssisted    EventType = "service_assisted"
	EventTypeDriftDetected      EventType = "drift_detected"
)

// Event represents an audit log entry
type Event struct {
	Timestamp time.Time              `json:"timestamp"`
	Type      EventType              `json:"type"`
	Details   map[string]interface{} `json:"details"`
	Message   string                 `json:"message,omitempty"`
}

// getBaseDirForAuditLog returns the base directory for storing audit log
// Uses same logic as workspace.GetBaseDir but specifically for config files
func getBaseDirForAuditLog() (string, error) {
	// Check for override environment variable
	if home := os.Getenv("RAIOZ_HOME"); home != "" {
		if err := os.MkdirAll(home, 0755); err != nil {
			return "", fmt.Errorf("failed to create RAIOZ_HOME directory '%s': %w", home, err)
		}
		return home, nil
	}

	// Try /opt/raioz-proyecto first (preferred location)
	optBase := "/opt/raioz-proyecto"
	if err := os.MkdirAll(optBase, 0755); err == nil {
		return optBase, nil
	}

	// Failed to create in /opt, use fallback
	usr, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get current user: %w", err)
	}

	homeDir := usr.HomeDir
	if homeDir == "" {
		return "", fmt.Errorf("home directory is empty")
	}

	fallbackBase := filepath.Join(homeDir, ".raioz")
	if runtime.GOOS == "windows" {
		fallbackBase = filepath.Join(homeDir, ".raioz")
	}

	if err := os.MkdirAll(fallbackBase, 0755); err != nil {
		return "", fmt.Errorf("failed to create fallback directory '%s': %w", fallbackBase, err)
	}

	return fallbackBase, nil
}

// GetAuditLogPath returns the path to the audit log file
func GetAuditLogPath() (string, error) {
	baseDir, err := getBaseDirForAuditLog()
	if err != nil {
		return "", fmt.Errorf("failed to get base directory for audit log: %w", err)
	}
	return filepath.Join(baseDir, auditLogFileName), nil
}

// Log writes an audit log entry
func Log(eventType EventType, details map[string]interface{}, message string) error {
	path, err := GetAuditLogPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory for audit log: %w", err)
	}

	// Create event
	event := Event{
		Timestamp: time.Now().UTC(),
		Type:      eventType,
		Details:   details,
		Message:   message,
	}

	// Marshal event to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	// Append to file (create if not exists, append mode)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit log file: %w", err)
	}
	defer file.Close()

	// Write JSON line (one event per line)
	if _, err := file.WriteString(string(eventJSON) + "\n"); err != nil {
		return fmt.Errorf("failed to write audit log: %w", err)
	}

	return nil
}

// LogDependencyAdded logs when a dependency is added via dependency assist
func LogDependencyAdded(serviceName string, source string, reason string) error {
	details := map[string]interface{}{
		"service": serviceName,
		"source":  source,
		"reason":  reason,
	}
	message := fmt.Sprintf("Dependency added: %s (source: %s)", serviceName, source)
	return Log(EventTypeDependencyAdded, details, message)
}

// LogOverrideApplied logs when an override is applied
func LogOverrideApplied(serviceName string, overridePath string) error {
	details := map[string]interface{}{
		"service":      serviceName,
		"override_path": overridePath,
	}
	message := fmt.Sprintf("Override applied: %s -> %s", serviceName, overridePath)
	return Log(EventTypeOverrideApplied, details, message)
}

// LogOverrideReverted logs when an override is reverted
func LogOverrideReverted(serviceName string, reason string) error {
	details := map[string]interface{}{
		"service": serviceName,
		"reason":  reason,
	}
	message := fmt.Sprintf("Override reverted: %s (reason: %s)", serviceName, reason)
	return Log(EventTypeOverrideReverted, details, message)
}

// LogConfigChanged logs when configuration root is changed
func LogConfigChanged(workspaceName string, changes []string) error {
	details := map[string]interface{}{
		"workspace": workspaceName,
		"changes":   changes,
	}
	message := fmt.Sprintf("Configuration changed in workspace: %s (%d changes)", workspaceName, len(changes))
	return Log(EventTypeConfigChanged, details, message)
}

// LogConflictResolved logs when a conflict is resolved
func LogConflictResolved(serviceName string, resolution string, reason string) error {
	details := map[string]interface{}{
		"service":    serviceName,
		"resolution": resolution,
		"reason":     reason,
	}
	message := fmt.Sprintf("Conflict resolved: %s (resolution: %s)", serviceName, resolution)
	return Log(EventTypeConflictResolved, details, message)
}

// LogWorkspaceChanged logs when workspace is changed
func LogWorkspaceChanged(oldWorkspace string, newWorkspace string) error {
	details := map[string]interface{}{
		"old_workspace": oldWorkspace,
		"new_workspace": newWorkspace,
	}
	message := fmt.Sprintf("Workspace changed: %s -> %s", oldWorkspace, newWorkspace)
	return Log(EventTypeWorkspaceChanged, details, message)
}

// LogServiceAssisted logs when a service is added via dependency assist
func LogServiceAssisted(serviceName string, addedBy string, reason string) error {
	details := map[string]interface{}{
		"service": serviceName,
		"added_by": addedBy,
		"reason":   reason,
	}
	message := fmt.Sprintf("Service assisted: %s (added by: %s)", serviceName, addedBy)
	return Log(EventTypeServiceAssisted, details, message)
}

// LogDriftDetected logs when configuration drift is detected in a service
func LogDriftDetected(serviceName string, servicePath string, differences []string) error {
	details := map[string]interface{}{
		"service":     serviceName,
		"config_path": servicePath,
		"differences": differences,
		"count":       len(differences),
	}
	message := fmt.Sprintf("Drift detected in service: %s (%d differences)", serviceName, len(differences))
	return Log(EventTypeDriftDetected, details, message)
}
