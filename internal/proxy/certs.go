package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	defaultCertsDir = ".raioz/certs"
	certFileName    = "localhost.pem"
	keyFileName     = "localhost-key.pem"
)

// CertsDir returns the default certificate directory (~/.raioz/certs/).
func CertsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, defaultCertsDir)
}

// EnsureCerts checks if mkcert certificates exist. If not, generates them.
// Returns the certs directory path, or empty string if mkcert is not available.
func EnsureCerts() (string, error) {
	dir := CertsDir()
	if dir == "" {
		return "", fmt.Errorf("could not determine home directory")
	}

	certPath := filepath.Join(dir, certFileName)
	keyPath := filepath.Join(dir, keyFileName)

	// Check if certs already exist
	if fileExists(certPath) && fileExists(keyPath) {
		return dir, nil
	}

	// Check if mkcert is available
	if !commandExists("mkcert") {
		return "", nil // Not an error — proxy works without HTTPS
	}

	// Create certs directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create certs directory: %w", err)
	}

	// Install root CA (if not already done)
	install := exec.Command("mkcert", "-install")
	install.Run() // Ignore error — may already be installed

	// Generate wildcard cert for *.localhost
	gen := exec.Command("mkcert",
		"-cert-file", certPath,
		"-key-file", keyPath,
		"localhost", "*.localhost",
	)
	if output, err := gen.CombinedOutput(); err != nil {
		return "", fmt.Errorf("mkcert failed: %w\n%s", err, string(output))
	}

	return dir, nil
}

// HasMkcert returns true if mkcert is installed.
func HasMkcert() bool {
	return commandExists("mkcert")
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
