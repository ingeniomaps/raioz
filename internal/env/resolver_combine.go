package env

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	raiozErr "raioz/internal/errors"
	"raioz/internal/workspace"
)

// createCombinedEnvFile merges multiple env files into a single combined file.
func createCombinedEnvFile(
	ws *workspace.Workspace,
	serviceName string,
	resolvedPaths []string,
	hasDotEnv bool,
	servicePath, projectDir string,
) (string, error) {
	var combinedPath string
	if hasDotEnv {
		if servicePath != "" {
			combinedPath = filepath.Join(servicePath, ".env")
		} else if projectDir != "" {
			combinedPath = filepath.Join(projectDir, ".env")
		} else {
			combinedPath = filepath.Join(ws.Root, fmt.Sprintf(".env.%s", serviceName))
		}
	} else {
		combinedPath = filepath.Join(ws.Root, fmt.Sprintf(".env.%s", serviceName))
	}

	if err := os.MkdirAll(filepath.Dir(combinedPath), 0700); err != nil {
		return "", raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to create directory for combined env file").
			WithContext("directory", filepath.Dir(combinedPath)).
			WithContext("service", serviceName).
			WithSuggestion("Check that the parent directory exists and you have write permissions").
			WithError(err)
	}

	mergedEnv, err := LoadFiles(resolvedPaths)
	if err != nil {
		return "", err
	}
	file, err := os.OpenFile(combinedPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to create combined env file").
			WithContext("file", combinedPath).
			WithContext("service", serviceName).
			WithSuggestion("Check file permissions and disk space").
			WithError(err)
	}
	defer file.Close()

	for key, value := range mergedEnv {
		escapedValue := value
		if strings.Contains(value, " ") || strings.Contains(value, "$") {
			escapedValue = fmt.Sprintf("\"%s\"", value)
		}
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, escapedValue); err != nil {
			return "", raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to write to combined env file").
				WithContext("file", combinedPath).
				WithContext("service", serviceName).
				WithSuggestion("Check disk space and file permissions").
				WithError(err)
		}
	}

	return combinedPath, nil
}
