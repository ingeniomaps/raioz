package errors

import (
	"fmt"
	"runtime/debug"

	"raioz/internal/logging"
)

// RecoverPanic recovers from a panic and converts it to a RaiozError
// This should be used with defer in critical operations
func RecoverPanic(operation string) *RaiozError {
	if r := recover(); r != nil {
		// Get stack trace
		stack := string(debug.Stack())

		// Log the panic for debugging
		logging.Error("Panic recovered in critical operation",
			"operation", operation,
			"panic", fmt.Sprintf("%v", r),
			"stack", stack)

		// Convert panic to error
		var panicMsg string
		if err, ok := r.(error); ok {
			panicMsg = err.Error()
		} else {
			panicMsg = fmt.Sprintf("%v", r)
		}

		return New(
			ErrCodeInternalError,
			fmt.Sprintf("Internal error in %s: %s", operation, panicMsg),
		).WithSuggestion(
			"This is an unexpected error. Please report this issue with the stack trace. "+
				"Check logs for more details.",
		).WithContext("operation", operation).
			WithContext("panic_value", panicMsg).
			WithContext("stack_trace", stack)
	}
	return nil
}
