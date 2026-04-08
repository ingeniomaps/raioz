package app

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"raioz/internal/config"
	"raioz/internal/i18n"
	"raioz/internal/mocks"
)

func initI18nIgnore(t *testing.T) {
	t.Helper()
	os.Setenv("RAIOZ_LANG", "en")
	tmpHome := t.TempDir()
	os.Setenv("RAIOZ_HOME", tmpHome)
	t.Cleanup(func() {
		os.Unsetenv("RAIOZ_LANG")
		os.Unsetenv("RAIOZ_HOME")
	})
	i18n.Init("en")
}

func TestIgnoreAddEmpty(t *testing.T) {
	initI18nIgnore(t)
	uc := NewIgnoreUseCase(&Dependencies{})
	err := uc.Add("", "config.json")
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestIgnoreAddNew(t *testing.T) {
	initI18nIgnore(t)

	uc := NewIgnoreUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return nil, nil, nil
			},
		},
	})
	var buf bytes.Buffer
	uc.Out = &buf

	err := uc.Add("test-svc", "config.json")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(buf.String(), "test-svc") {
		t.Errorf("should mention service\ngot: %s", buf.String())
	}
}

func TestIgnoreAddDuplicate(t *testing.T) {
	initI18nIgnore(t)

	uc := NewIgnoreUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return nil, nil, nil
			},
		},
	})
	var buf bytes.Buffer
	uc.Out = &buf

	uc.Add("dup-svc", "")
	buf.Reset()
	uc.Add("dup-svc", "")

	if !strings.Contains(strings.ToLower(buf.String()), "already ignored") {
		t.Errorf("should say already ignored\ngot: %s", buf.String())
	}
}

func TestIgnoreAddWithDependents(t *testing.T) {
	initI18nIgnore(t)

	cfgDeps := &config.Deps{
		Project: config.Project{Name: "test"},
		Services: map[string]config.Service{
			"db": {},
			"api": {
				DependsOn: []string{"db"},
			},
		},
	}

	uc := NewIgnoreUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return cfgDeps, nil, nil
			},
		},
	})
	var buf bytes.Buffer
	uc.Out = &buf

	err := uc.Add("db", "config.json")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Should warn about api depending on db
	if !strings.Contains(buf.String(), "api") {
		t.Errorf("should warn about dependent service 'api'\ngot: %s", buf.String())
	}
}

func TestIgnoreRemoveNotIgnored(t *testing.T) {
	initI18nIgnore(t)

	uc := NewIgnoreUseCase(&Dependencies{})
	var buf bytes.Buffer
	uc.Out = &buf

	err := uc.Remove("not-ignored")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(strings.ToLower(buf.String()), "not in") {
		t.Errorf("should say not in list\ngot: %s", buf.String())
	}
}

func TestIgnoreListEmpty(t *testing.T) {
	initI18nIgnore(t)

	uc := NewIgnoreUseCase(&Dependencies{})
	var buf bytes.Buffer
	uc.Out = &buf

	err := uc.List()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(strings.ToLower(buf.String()), "no service") {
		t.Errorf("should show empty message\ngot: %s", buf.String())
	}
}

func TestIgnoreFullCycle(t *testing.T) {
	initI18nIgnore(t)

	uc := NewIgnoreUseCase(&Dependencies{
		ConfigLoader: &mocks.MockConfigLoader{
			LoadDepsFunc: func(path string) (*config.Deps, []string, error) {
				return nil, nil, nil
			},
		},
	})
	var buf bytes.Buffer
	uc.Out = &buf

	// Add
	uc.Add("svc1", "")
	uc.Add("svc2", "")

	// List
	buf.Reset()
	uc.List()
	output := buf.String()
	if !strings.Contains(output, "svc1") || !strings.Contains(output, "svc2") {
		t.Errorf("list should show both services\ngot: %s", output)
	}

	// Remove
	buf.Reset()
	uc.Remove("svc1")
	if !strings.Contains(buf.String(), "svc1") {
		t.Errorf("remove should mention service\ngot: %s", buf.String())
	}

	// List again
	buf.Reset()
	uc.List()
	output = buf.String()
	if strings.Contains(output, "svc1") {
		t.Errorf("svc1 should be removed\ngot: %s", output)
	}
	if !strings.Contains(output, "svc2") {
		t.Errorf("svc2 should still be there\ngot: %s", output)
	}
}
