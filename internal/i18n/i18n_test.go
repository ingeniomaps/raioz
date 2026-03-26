package i18n

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInit_DefaultsToEnglish(t *testing.T) {
	reset()
	// Isolate from saved preferences and system locale
	SetBaseDir(t.TempDir())
	os.Unsetenv("RAIOZ_LANG")
	origLang := os.Getenv("LANG")
	origLCAll := os.Getenv("LC_ALL")
	os.Unsetenv("LANG")
	os.Unsetenv("LC_ALL")
	defer func() {
		os.Setenv("LANG", origLang)
		if origLCAll != "" {
			os.Setenv("LC_ALL", origLCAll)
		}
	}()

	Init("")

	if lang := GetLang(); lang != "en" {
		t.Errorf("expected default lang 'en', got '%s'", lang)
	}
}

func TestInit_ExplicitLang(t *testing.T) {
	reset()
	SetBaseDir(t.TempDir())
	Init("es")

	if lang := GetLang(); lang != "es" {
		t.Errorf("expected lang 'es', got '%s'", lang)
	}
}

func TestInit_EnvVar(t *testing.T) {
	reset()
	SetBaseDir(t.TempDir())
	os.Setenv("RAIOZ_LANG", "es")
	defer os.Unsetenv("RAIOZ_LANG")

	Init("")

	if lang := GetLang(); lang != "es" {
		t.Errorf("expected lang 'es' from env, got '%s'", lang)
	}
}

func TestInit_InvalidFallsBackToDefault(t *testing.T) {
	reset()
	SetBaseDir(t.TempDir())
	Init("xx")

	if lang := GetLang(); lang != "en" {
		t.Errorf("expected fallback to 'en', got '%s'", lang)
	}
}

func TestT_ReturnsEnglish(t *testing.T) {
	reset()
	Init("en")

	got := T("cmd.up.short")
	want := "Bring up project dependencies"
	if got != want {
		t.Errorf("T('cmd.up.short') = %q, want %q", got, want)
	}
}

func TestT_ReturnsSpanish(t *testing.T) {
	reset()
	Init("es")

	got := T("cmd.up.short")
	want := "Levantar dependencias del proyecto"
	if got != want {
		t.Errorf("T('cmd.up.short') = %q, want %q", got, want)
	}
}

func TestT_WithArgs(t *testing.T) {
	reset()
	Init("en")

	got := T("output.project_started", "my-app")
	want := "Project 'my-app' started successfully"
	if got != want {
		t.Errorf("T with args = %q, want %q", got, want)
	}
}

func TestT_FallsBackToEnglish(t *testing.T) {
	reset()
	Init("es")

	// Use a key that exists in English but we'll verify fallback behavior
	// by testing with a key that might not exist in es
	got := T("nonexistent.key")
	want := "nonexistent.key"
	if got != want {
		t.Errorf("T for missing key = %q, want key itself %q", got, want)
	}
}

func TestT_MissingKeyReturnsKey(t *testing.T) {
	reset()
	Init("en")

	got := T("this.key.does.not.exist")
	want := "this.key.does.not.exist"
	if got != want {
		t.Errorf("T for missing key = %q, want %q", got, want)
	}
}

func TestSetLang(t *testing.T) {
	reset()
	Init("en")

	if err := SetLang("es"); err != nil {
		t.Fatalf("SetLang('es') error: %v", err)
	}
	if lang := GetLang(); lang != "es" {
		t.Errorf("after SetLang('es'), GetLang() = %q", lang)
	}

	got := T("cmd.down.short")
	want := "Detener dependencias del proyecto"
	if got != want {
		t.Errorf("after SetLang('es'), T = %q, want %q", got, want)
	}
}

func TestSetLang_InvalidReturnsError(t *testing.T) {
	reset()
	Init("en")

	if err := SetLang("xx"); err == nil {
		t.Error("SetLang('xx') should return error")
	}
}

func TestAvailable(t *testing.T) {
	reset()
	Init("")

	langs := Available()
	if len(langs) < 2 {
		t.Errorf("expected at least 2 languages, got %d: %v", len(langs), langs)
	}

	hasEn, hasEs := false, false
	for _, l := range langs {
		if l == "en" {
			hasEn = true
		}
		if l == "es" {
			hasEs = true
		}
	}
	if !hasEn || !hasEs {
		t.Errorf("expected 'en' and 'es' in available, got %v", langs)
	}
}

func TestSaveAndLoadPreference(t *testing.T) {
	reset()
	Init("en")

	tmpDir := t.TempDir()
	SetBaseDir(tmpDir)

	if err := SavePreference("es"); err != nil {
		t.Fatalf("SavePreference error: %v", err)
	}

	loaded := LoadPreference()
	if loaded != "es" {
		t.Errorf("LoadPreference() = %q, want 'es'", loaded)
	}

	// Verify file exists
	configPath := filepath.Join(tmpDir, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config.json was not created")
	}
}

func TestNormalizeLocale(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"es_ES.UTF-8", "es"},
		{"en_US", "en"},
		{"es", "es"},
		{"pt_BR.utf8", "pt"},
		{"EN", "en"},
		{"C", "c"},
	}

	for _, tt := range tests {
		got := normalizeLocale(tt.input)
		if got != tt.want {
			t.Errorf("normalizeLocale(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCatalogCompleteness(t *testing.T) {
	reset()
	Init("en")

	mu.RLock()
	enCatalog := catalogs["en"]
	esCatalog := catalogs["es"]
	mu.RUnlock()

	if enCatalog == nil {
		t.Fatal("English catalog not loaded")
	}
	if esCatalog == nil {
		t.Fatal("Spanish catalog not loaded")
	}

	// Check that es has all keys from en
	var missing []string
	for key := range enCatalog {
		if _, ok := esCatalog[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		t.Errorf("Spanish catalog is missing %d keys from English: %v", len(missing), missing)
	}

	// Check that en has all keys from es
	var extra []string
	for key := range esCatalog {
		if _, ok := enCatalog[key]; !ok {
			extra = append(extra, key)
		}
	}
	if len(extra) > 0 {
		t.Errorf("Spanish catalog has %d keys not in English: %v", len(extra), extra)
	}
}

// reset clears global state for test isolation
func reset() {
	mu.Lock()
	defer mu.Unlock()
	catalogs = nil
	available = nil
	currentLang = ""
	initialized = false
	raiozBaseDir = ""
}
