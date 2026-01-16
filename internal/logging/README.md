# Logging Package

This package provides structured logging using Go's standard `log/slog` package.

## Usage

The logging package is initialized automatically when the CLI starts. You can configure it using:

- **Command-line flags:**
  - `--log-level` (debug, info, warn, error)
  - `--log-json` (output JSON format)

- **Environment variables:**
  - `RAIOZ_LOG_LEVEL` (debug, info, warn, error)
  - `RAIOZ_LOG_JSON` (true/false)

- **Automatic detection:**
  - JSON format is automatically enabled in CI environments
  - Default level is INFO

## Examples

```go
import "raioz/internal/logging"

// Initialize if needed (usually done automatically)
logging.Default()

// Log messages
logging.Info("Processing request", "user", "john", "action", "login")
logging.Warn("Deprecated feature used", "feature", "old-api")
logging.Error("Failed to process", "error", err)
logging.Debug("Debug information", "data", someData) // Only if level is debug
```

## Integration with Sanitization

When logging environment variables or secrets, use the sanitization package:

```go
import (
    "raioz/internal/env"
    "raioz/internal/logging"
)

// Sanitize before logging
sanitized := env.SanitizeEnvValue("PASSWORD", password)
logging.Info("Setting password", "key", "PASSWORD", "value", sanitized)
```

## Gradual Migration

The logging infrastructure is ready for use. To gradually migrate from `fmt.Printf` to structured logging:

1. Use `logging.Info()` for informational messages
2. Use `logging.Warn()` for warnings
3. Use `logging.Error()` for errors
4. Use `logging.Debug()` for debug information
5. Replace `fmt.Printf("ℹ️  ...")` with `logging.Info(...)`
6. Replace `fmt.Printf("⚠️  ...")` with `logging.Warn(...)`
7. Replace `fmt.Printf("🔴 ...")` with `logging.Error(...)`
