package errors

import (
	"fmt"
	"runtime"
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
			"This is an unexpected error. Please report this issue with the stack trace. " +
				"Check logs for more details.",
		).WithContext("operation", operation).
			WithContext("panic_value", panicMsg).
			WithContext("stack_trace", stack)
	}
	return nil
}

// RecoverPanicWithError recovers from a panic and returns it as an error
// This is a simpler version that just returns a regular error
func RecoverPanicWithError(operation string) error {
	if r := recover(); r != nil {
		stack := string(debug.Stack())

		logging.Error("Panic recovered in critical operation",
			"operation", operation,
			"panic", fmt.Sprintf("%v", r),
			"stack", stack)

		var panicMsg string
		if err, ok := r.(error); ok {
			panicMsg = err.Error()
		} else {
			panicMsg = fmt.Sprintf("%v", r)
		}

		return fmt.Errorf("panic in %s: %v\nStack trace:\n%s", operation, panicMsg, stack)
	}
	return nil
}

// SafeExecute executes a function with panic recovery
// Returns the error from the function or a panic recovery error
func SafeExecute(operation string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())

			logging.Error("Panic recovered in critical operation",
				"operation", operation,
				"panic", fmt.Sprintf("%v", r),
				"stack", stack)

			var panicMsg string
			if err, ok := r.(error); ok {
				panicMsg = err.Error()
			} else {
				panicMsg = fmt.Sprintf("%v", r)
			}

			err = New(
				ErrCodeInternalError,
				fmt.Sprintf("Internal error in %s: %s", operation, panicMsg),
			).WithSuggestion(
				"This is an unexpected error. Please report this issue with the stack trace. " +
					"Check logs for more details.",
			).WithContext("operation", operation).
				WithContext("panic_value", panicMsg).
				WithContext("stack_trace", stack)
		}
	}()

	return fn()
}

// GetCallerInfo returns information about the caller for error context
func GetCallerInfo(skip int) map[string]interface{} {
	pc, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return map[string]interface{}{
			"caller": "unknown",
		}
	}

	fn := runtime.FuncForPC(pc)
	return map[string]interface{}{
		"file":     file,
		"line":     line,
		"function": fn.Name(),
	}
}
