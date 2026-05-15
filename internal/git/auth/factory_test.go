package auth

import (
	"strings"
	"testing"
)

func TestProviderFor_DefaultMapsToStrict(t *testing.T) {
	p, err := ProviderFor("")
	if err != nil {
		t.Fatalf("unexpected error for empty name: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil provider")
	}
	if p.Name() != "" {
		t.Errorf("expected Name() == \"\" for default, got %q", p.Name())
	}
}

func TestProviderFor_UnknownReturnsError(t *testing.T) {
	// Names we deliberately reject:
	//   - obvious typos
	//   - aliases that look reasonable but aren't supported
	//   - case variants (raioz.yaml fields are case-sensitive)
	//   - leading/trailing whitespace (yaml unmarshaling should
	//     strip, but defense-in-depth)
	//   - future names that haven't been wired yet
	cases := []string{
		"unknown",
		"github",
		"GH",
		"INHERIT",
		"gh ",
		" gh",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			p, err := ProviderFor(name)
			if err == nil {
				t.Errorf("expected error for %q, got nil", name)
			}
			if p != nil {
				t.Errorf("expected nil provider for unknown name, got %v", p)
			}
			// Error message should at least mention the offending name
			// so the dev knows what to fix.
			if err != nil && !strings.Contains(err.Error(), name) {
				t.Errorf("error %q should reference name %q", err.Error(), name)
			}
		})
	}
}
