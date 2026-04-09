package errors

import "fmt"

// ErrorCode represents a specific error code for better error handling
type ErrorCode string

const (
	// Configuration errors
	ErrCodeInvalidConfig      ErrorCode = "INVALID_CONFIG"
	ErrCodeSchemaValidation   ErrorCode = "SCHEMA_VALIDATION"
	ErrCodeMissingField       ErrorCode = "MISSING_FIELD"
	ErrCodeInvalidField       ErrorCode = "INVALID_FIELD"

	// Docker errors
	ErrCodeDockerNotInstalled ErrorCode = "DOCKER_NOT_INSTALLED"
	ErrCodeDockerNotRunning   ErrorCode = "DOCKER_NOT_RUNNING"
	ErrCodePortConflict       ErrorCode = "PORT_CONFLICT"
	ErrCodeImagePullFailed    ErrorCode = "IMAGE_PULL_FAILED"
	ErrCodeNetworkError       ErrorCode = "NETWORK_ERROR"
	ErrCodeVolumeError        ErrorCode = "VOLUME_ERROR"

	// Git errors
	ErrCodeGitNotInstalled    ErrorCode = "GIT_NOT_INSTALLED"
	ErrCodeGitCloneFailed     ErrorCode = "GIT_CLONE_FAILED"
	ErrCodeGitBranchNotFound  ErrorCode = "GIT_BRANCH_NOT_FOUND"
	ErrCodeGitConflict        ErrorCode = "GIT_CONFLICT"
	ErrCodeNetworkUnavailable ErrorCode = "NETWORK_UNAVAILABLE"

	// Workspace errors
	ErrCodeWorkspaceError     ErrorCode = "WORKSPACE_ERROR"
	ErrCodePermissionDenied   ErrorCode = "PERMISSION_DENIED"
	ErrCodeDiskSpaceLow       ErrorCode = "DISK_SPACE_LOW"

	// State errors
	ErrCodeLockError          ErrorCode = "LOCK_ERROR"
	ErrCodeStateLoadError     ErrorCode = "STATE_LOAD_ERROR"
	ErrCodeStateSaveError     ErrorCode = "STATE_SAVE_ERROR"

	// Validation errors
	ErrCodeDependencyCycle    ErrorCode = "DEPENDENCY_CYCLE"
	ErrCodeCompatibilityError ErrorCode = "COMPATIBILITY_ERROR"

	// Internal errors
	ErrCodeInternalError ErrorCode = "INTERNAL_ERROR"
)

// RaiozError represents a structured error with context and suggestions
type RaiozError struct {
	Code        ErrorCode
	Message     string
	Context     map[string]interface{}
	Suggestion  string
	OriginalErr error
}

// Error implements the error interface
func (e *RaiozError) Error() string {
	return e.Message
}

// Unwrap returns the original error if any
func (e *RaiozError) Unwrap() error {
	return e.OriginalErr
}

// New creates a new RaiozError
func New(code ErrorCode, message string) *RaiozError {
	return &RaiozError{
		Code:    code,
		Message: message,
		Context: make(map[string]interface{}),
	}
}

// WithContext adds context to the error
func (e *RaiozError) WithContext(key string, value interface{}) *RaiozError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithSuggestion adds a suggestion to the error
func (e *RaiozError) WithSuggestion(suggestion string) *RaiozError {
	e.Suggestion = suggestion
	return e
}

// WithError wraps an original error
func (e *RaiozError) WithError(err error) *RaiozError {
	e.OriginalErr = err
	return e
}

// Format formats the error for display
func (e *RaiozError) Format() string {
	var result string
	result += fmt.Sprintf("\033[31m[error]\033[0m [%s] %s\n", e.Code, e.Message)

	if len(e.Context) > 0 {
		result += "\n  Context:\n"
		for key, value := range e.Context {
			result += fmt.Sprintf("    %s: %v\n", key, value)
		}
	}

	if e.Suggestion != "" {
		result += fmt.Sprintf("\n  \033[33mSuggestion:\033[0m %s\n", e.Suggestion)
	}

	if e.OriginalErr != nil {
		result += fmt.Sprintf("\nOriginal error: %v\n", e.OriginalErr)
	}

	return result
}
