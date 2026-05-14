package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"raioz/internal/logging"
	"raioz/internal/naming"
)

const (
	auditLogFileName = "audit.log"

	// maxAuditSize is the soft cap on the audit log. Past this size,
	// the next Log call rotates the current file to .1 (overwriting
	// the previous .1) before writing. Picked at ~one month of heavy
	// raioz use; ADR-020 documents the trade-off.
	maxAuditSize = 10 * 1024 * 1024 // 10 MiB
)

// EventType represents the type of audit event
type EventType string

const (
	EventTypeDependencyAdded  EventType = "dependency_added"
	EventTypeConfigChanged    EventType = "config_changed"
	EventTypeConflictResolved EventType = "conflict_resolved"
	EventTypeServiceAssisted  EventType = "service_assisted"
	EventTypeDriftDetected    EventType = "drift_detected"
	EventTypeDevPromoted      EventType = "dev_promoted"
	EventTypeDevReverted      EventType = "dev_reverted"

	// Lifecycle events for up/down/restart. Phase identifies start vs
	// complete; status distinguishes success from failure on complete.
	// See OBSERVABILITY.md and issue 048.
	EventTypeLifecycle EventType = "lifecycle"
)

// LifecyclePhase enumerates the phase value carried in a lifecycle
// event's Details. Kept as a typed string so callers can't fat-finger
// the convention.
type LifecyclePhase string

const (
	LifecycleStart    LifecyclePhase = "start"
	LifecycleComplete LifecyclePhase = "complete"
)

// Event represents an audit log entry.
type Event struct {
	Timestamp time.Time `json:"timestamp"`
	// CorrelationID groups records emitted by the same logical
	// operation, including recursive sibling spawns. Sourced from
	// logging.GetRequestID(ctx) so audit and slog share the value.
	// Omitted from JSON when empty for backwards compat with readers
	// that predate the field.
	CorrelationID string                 `json:"correlation_id,omitempty"`
	Type          EventType              `json:"type"`
	Details       map[string]interface{} `json:"details"`
	Message       string                 `json:"message,omitempty"`
}

// GetAuditLogPath returns the path to the audit log file.
// Location delegated to naming.RaiozStateDir() — ADR-022.
func GetAuditLogPath() (string, error) {
	baseDir := naming.RaiozStateDir()
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create audit state dir %q: %w", baseDir, err)
	}
	return filepath.Join(baseDir, auditLogFileName), nil
}

// Log writes an audit log entry. Convenience for call sites without
// a ctx; LogWithContext is preferred — it stamps the correlation ID.
func Log(eventType EventType, details map[string]interface{}, message string) error {
	return LogWithContext(context.Background(), eventType, details, message)
}

// LogWithContext writes an audit log entry, sourcing the correlation
// ID from the ctx (via logging.GetRequestID). Recursive raioz
// invocations inherit the same ID from RAIOZ_CORRELATION_ID, so
// grep on correlation_id reconstructs a parent+children spawn tree.
func LogWithContext(
	ctx context.Context,
	eventType EventType,
	details map[string]interface{},
	message string,
) error {
	path, err := GetAuditLogPath()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory for audit log: %w", err)
	}

	// Rotate before append when the file has crossed the size cap.
	// Best-effort: rotation failures fall through and the event is
	// still appended (better to lose one rotation than one event).
	rotateIfOverCap(path, maxAuditSize)

	event := Event{
		Timestamp:     time.Now().UTC(),
		CorrelationID: logging.GetRequestID(ctx),
		Type:          eventType,
		Details:       details,
		Message:       message,
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

// rotateIfOverCap renames `path` → `path + ".1"` (overwriting an older
// .1 if present) when the file is larger than `cap`. Any other state
// — missing file, file under cap, permission error — is a no-op. The
// next Log call will create the file fresh.
//
// We pre-check the size with a Stat rather than reading-then-writing
// to avoid the round-trip on every event. The 10 MiB threshold is
// large enough that this stat is the dominant cost only in
// rotation-pressure scenarios — fine.
func rotateIfOverCap(path string, capBytes int64) {
	info, err := os.Stat(path)
	if err != nil || info.Size() <= capBytes {
		return
	}
	rotated := path + ".1"
	// Best-effort. os.Rename overwrites the destination on Linux.
	// Errors here only mean "the .1 from a previous rotation
	// persists" — acceptable; the next rotation tries again.
	_ = os.Rename(path, rotated)
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

// LogDevPromoted logs when a dependency is promoted to local development.
func LogDevPromoted(depName string, localPath string, originalImage string) error {
	details := map[string]interface{}{
		"dependency":     depName,
		"local_path":     localPath,
		"original_image": originalImage,
	}
	message := fmt.Sprintf("Dev promoted: %s -> %s (was: %s)", depName, localPath, originalImage)
	return Log(EventTypeDevPromoted, details, message)
}

// LogDevReverted logs when a dependency is reverted from local to image.
func LogDevReverted(depName string, originalImage string) error {
	details := map[string]interface{}{
		"dependency":     depName,
		"original_image": originalImage,
	}
	message := fmt.Sprintf("Dev reverted: %s -> %s", depName, originalImage)
	return Log(EventTypeDevReverted, details, message)
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

// LogServiceAssisted logs when a service is added via dependency assist
func LogServiceAssisted(serviceName string, addedBy string, reason string) error {
	details := map[string]interface{}{
		"service":  serviceName,
		"added_by": addedBy,
		"reason":   reason,
	}
	message := fmt.Sprintf("Service assisted: %s (added by: %s)", serviceName, addedBy)
	return Log(EventTypeServiceAssisted, details, message)
}

// lifecycleDetails packs the fields common to every lifecycle event
// (start and complete) and is the only shape the helpers below
// construct, so the field names stay stable for downstream JSONL
// readers.
func lifecycleDetails(
	operation string, phase LifecyclePhase, project, workspace string,
) map[string]interface{} {
	d := map[string]interface{}{
		"operation": operation,
		"phase":     string(phase),
		"project":   project,
	}
	if workspace != "" {
		d["workspace"] = workspace
	}
	return d
}

// LogLifecycleStart records that `raioz <operation>` has begun. The
// matching LogLifecycleComplete (with status + duration + err) closes
// the pair. operation is "up" | "down" | "restart" — convention, not
// enforced; downstream queries pivot on the Details.operation value.
func LogLifecycleStart(
	ctx context.Context, operation, project, workspace string,
) error {
	d := lifecycleDetails(operation, LifecycleStart, project, workspace)
	msg := fmt.Sprintf("raioz %s started: %s", operation, project)
	return LogWithContext(ctx, EventTypeLifecycle, d, msg)
}

// LogLifecycleComplete records the outcome of `raioz <operation>`.
// status is "success" | "failure"; err is nil on success. duration is
// elapsed wall-clock from the matching start.
func LogLifecycleComplete(
	ctx context.Context,
	operation, project, workspace, status string,
	duration time.Duration,
	err error,
) error {
	d := lifecycleDetails(operation, LifecycleComplete, project, workspace)
	d["status"] = status
	d["duration_ms"] = duration.Milliseconds()
	if err != nil {
		d["error"] = err.Error()
	}
	msg := fmt.Sprintf(
		"raioz %s %s: %s (%dms)",
		operation, status, project, duration.Milliseconds(),
	)
	return LogWithContext(ctx, EventTypeLifecycle, d, msg)
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
