package env

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	raiozErr "raioz/internal/errors"
)

// LoadFiles loads and merges environment variables from multiple files
// Later files override earlier ones (order of precedence)
func LoadFiles(filePaths []string) (map[string]string, error) {
	env := make(map[string]string)

	for _, filePath := range filePaths {
		fileEnv, err := loadSingleFile(filePath)
		if err != nil {
			return nil, raiozErr.New(raiozErr.ErrCodeInvalidConfig, "failed to load env file").
				WithContext("file", filePath).
				WithSuggestion("Check that the env file exists and has valid KEY=VALUE format").
				WithError(err)
		}

		// Merge: later files override earlier ones
		for k, v := range fileEnv {
			env[k] = v
		}
	}

	return env, nil
}

// loadSingleFile loads environment variables from a single .env file
func loadSingleFile(filePath string) (map[string]string, error) {
	env := make(map[string]string)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, raiozErr.New(raiozErr.ErrCodeInvalidField, "invalid format in env file: expected KEY=VALUE").
				WithContext("file", filePath).
				WithContext("line", lineNum).
				WithSuggestion("Each line must follow the KEY=VALUE format, or be a comment starting with #")
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove quotes if present and unescape the value
		if len(value) >= 2 {
			if value[0] == '"' && value[len(value)-1] == '"' {
				// Double-quoted value: unescape it properly
				quoted := value
				unquoted, err := strconv.Unquote(quoted)
				if err == nil {
					value = unquoted
				} else {
					// Fallback: just remove quotes without unescaping
					value = value[1 : len(value)-1]
				}
			} else if value[0] == '\'' && value[len(value)-1] == '\'' {
				// Single-quoted value: just remove quotes (no escaping in single quotes)
				value = value[1 : len(value)-1]
			}
		}

		env[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, raiozErr.New(raiozErr.ErrCodeInvalidConfig, "error reading env file").
			WithContext("file", filePath).
			WithSuggestion("Check that the file is not corrupted and is readable").
			WithError(err)
	}

	return env, nil
}
