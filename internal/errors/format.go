package errors

import (
	"fmt"
	"strings"
)

// FormatError formats an error for display with context and suggestions
func FormatError(err error) string {
	// Check if it's a RaiozError
	if raiozErr, ok := err.(*RaiozError); ok {
		return raiozErr.Format()
	}

	// Check if it wraps a RaiozError
	var raiozErr *RaiozError
	if As(err, &raiozErr) {
		return raiozErr.Format()
	}

	// Default formatting for regular errors
	return fmt.Sprintf("\033[31m[error]\033[0m %v\n", err)
}

// FormatMultipleErrors formats multiple errors for display
func FormatMultipleErrors(errs []error) string {
	if len(errs) == 0 {
		return ""
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("\033[31m[error]\033[0m Found %d error(s):\n\n", len(errs)))

	for i, err := range errs {
		result.WriteString(fmt.Sprintf("%d. %s", i+1, FormatError(err)))
		if i < len(errs)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// As checks if an error matches the target type (similar to errors.As)
func As(err error, target **RaiozError) bool {
	if err == nil {
		return false
	}

	if raiozErr, ok := err.(*RaiozError); ok {
		*target = raiozErr
		return true
	}

	// Check if error has Unwrap method (for wrapped errors)
	type unwrapper interface {
		Unwrap() error
	}

	if u, ok := err.(unwrapper); ok {
		return As(u.Unwrap(), target)
	}

	return false
}
