package errors

import (
	stderrs "errors"
	"fmt"
	"strings"
)

// FormatError formats an error for display with context and suggestions.
// Returns the structured RaiozError view when err is (or wraps) one;
// falls back to the default red-prefix display otherwise.
func FormatError(err error) string {
	var raiozErr *RaiozError
	if stderrs.As(err, &raiozErr) {
		return raiozErr.Format()
	}
	return fmt.Sprintf("\033[31m[error]\033[0m %v\n", err)
}

// FormatMultipleErrors formats multiple errors for display
func FormatMultipleErrors(errs []error) string {
	if len(errs) == 0 {
		return ""
	}

	var result strings.Builder
	fmt.Fprintf(&result, "\033[31m[error]\033[0m Found %d error(s):\n\n", len(errs))

	for i, err := range errs {
		fmt.Fprintf(&result, "%d. %s", i+1, FormatError(err))
		if i < len(errs)-1 {
			result.WriteString("\n")
		}
	}

	return result.String()
}
