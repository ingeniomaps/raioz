package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed locales/*.json
var localesFS embed.FS

const (
	defaultLang = "en"
	configFile  = "config.json"
)

var (
	mu           sync.RWMutex
	currentLang  string
	catalogs     map[string]map[string]string
	available    []string
	initialized  bool
	raiozBaseDir string
)

// Init loads all embedded locale catalogs and sets the active language.
// Detection order: explicit lang param > saved preference > RAIOZ_LANG env > LANG/LC_ALL > "en"
func Init(lang string) {
	mu.Lock()
	defer mu.Unlock()

	if catalogs == nil {
		catalogs = make(map[string]map[string]string)
	}

	// Load all embedded locales
	entries, err := localesFS.ReadDir("locales")
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			langCode := strings.TrimSuffix(entry.Name(), ".json")
			data, err := localesFS.ReadFile("locales/" + entry.Name())
			if err != nil {
				continue
			}
			var catalog map[string]string
			if err := json.Unmarshal(data, &catalog); err != nil {
				continue
			}
			catalogs[langCode] = catalog
			available = append(available, langCode)
		}
	}

	// Resolve language (use internal version to avoid deadlock — we already hold mu)
	resolved := resolveLangInternal(lang)
	if _, ok := catalogs[resolved]; !ok {
		resolved = defaultLang
	}
	currentLang = resolved
	initialized = true
}

// T returns the translated string for the given key in the current language.
// If the key is not found in the current language, falls back to English.
// If not found in English either, returns the key itself.
// Optional args are passed to fmt.Sprintf if the translated string contains format verbs.
func T(key string, args ...any) string {
	mu.RLock()
	defer mu.RUnlock()

	if !initialized {
		return key
	}

	// Try current language
	if catalog, ok := catalogs[currentLang]; ok {
		if val, ok := catalog[key]; ok {
			if len(args) > 0 {
				return fmt.Sprintf(val, args...)
			}
			return val
		}
	}

	// Fallback to English
	if currentLang != defaultLang {
		if catalog, ok := catalogs[defaultLang]; ok {
			if val, ok := catalog[key]; ok {
				if len(args) > 0 {
					return fmt.Sprintf(val, args...)
				}
				return val
			}
		}
	}

	// Key not found anywhere — return the key itself
	return key
}

// SetLang changes the active language. Returns error if language is not available.
func SetLang(lang string) error {
	mu.Lock()
	defer mu.Unlock()

	if _, ok := catalogs[lang]; !ok {
		return fmt.Errorf("language '%s' is not available", lang)
	}
	currentLang = lang
	return nil
}

// GetLang returns the current active language code.
func GetLang() string {
	mu.RLock()
	defer mu.RUnlock()
	return currentLang
}

// Available returns the list of available language codes.
func Available() []string {
	mu.RLock()
	defer mu.RUnlock()
	result := make([]string, len(available))
	copy(result, available)
	return result
}

// SetBaseDir sets the raioz base directory (for config file persistence).
func SetBaseDir(dir string) {
	mu.Lock()
	defer mu.Unlock()
	raiozBaseDir = dir
}

// SavePreference persists the language preference to ~/.raioz/config.json.
func SavePreference(lang string) error {
	mu.RLock()
	dir := raiozBaseDir
	mu.RUnlock()

	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		dir = filepath.Join(home, ".raioz")
	}

	configPath := filepath.Join(dir, configFile)

	// Load existing config to preserve other fields
	existing := make(map[string]any)
	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, &existing)
	}

	existing["language"] = lang

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// LoadPreference reads the saved language preference from ~/.raioz/config.json.
// Returns empty string if no preference is saved.
func LoadPreference() string {
	mu.RLock()
	defer mu.RUnlock()
	return loadPreferenceInternal()
}

// loadPreferenceInternal reads preference without locking (caller must hold mu).
func loadPreferenceInternal() string {
	dir := raiozBaseDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".raioz")
	}

	configPath := filepath.Join(dir, configFile)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return ""
	}

	if lang, ok := config["language"].(string); ok {
		return lang
	}
	return ""
}

// resolveLangInternal determines the language (must be called while holding mu).
func resolveLangInternal(explicit string) string {
	if explicit != "" {
		return explicit
	}

	// Saved preference (use internal version — caller holds mu)
	if saved := loadPreferenceInternal(); saved != "" {
		return saved
	}

	// RAIOZ_LANG env var
	if envLang := os.Getenv("RAIOZ_LANG"); envLang != "" {
		return normalizeLocale(envLang)
	}

	// LANG / LC_ALL
	if lc := os.Getenv("LC_ALL"); lc != "" {
		return normalizeLocale(lc)
	}
	if lang := os.Getenv("LANG"); lang != "" {
		return normalizeLocale(lang)
	}

	return defaultLang
}

// normalizeLocale extracts the 2-letter language code from a locale string.
// e.g. "es_ES.UTF-8" → "es", "en_US" → "en", "es" → "es"
func normalizeLocale(locale string) string {
	// Remove encoding (e.g., ".UTF-8")
	if idx := strings.Index(locale, "."); idx >= 0 {
		locale = locale[:idx]
	}
	// Remove region (e.g., "_ES")
	if idx := strings.Index(locale, "_"); idx >= 0 {
		locale = locale[:idx]
	}
	return strings.ToLower(locale)
}
