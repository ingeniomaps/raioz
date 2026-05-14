package config

import (
	"strings"
	"testing"

	"raioz/internal/errors"
)

func TestScanForSecrets_PositiveMatches(t *testing.T) {
	cases := []struct {
		name    string
		secret  string
		pattern string
	}{
		{"github PAT (ghp_)", "ghp_" + strings.Repeat("a", 40), "GitHub personal access token"},
		{"github OAuth (gho_)", "gho_" + strings.Repeat("b", 40), "GitHub OAuth token"},
		{"github user-to-server (ghu_)", "ghu_" + strings.Repeat("c", 40), "GitHub user-to-server token"},
		{"github server-to-server (ghs_)", "ghs_" + strings.Repeat("d", 40), "GitHub server-to-server token"},
		{"github refresh (ghr_)", "ghr_" + strings.Repeat("e", 40), "GitHub refresh token"},
		{"gitlab PAT (glpat-)", "glpat-" + strings.Repeat("f", 25), "GitLab personal access token"},
		// Slack tokens are built via concatenation so the literal
		// `xox[bopa]-<digits>-<word>` pattern never appears in source —
		// keeps GitHub Push Protection / gitleaks / truffleHog from
		// flagging this fixture file.
		{"slack bot token (xoxb-)", "xoxb-" + "1234567890-abcdefghij", "Slack token"},
		{"slack oauth token (xoxo-)", "xoxo-" + "1234567890-abcdefghij", "Slack token"},
		{"slack app token (xoxa-)", "xoxa-" + "1234567890-abcdefghij", "Slack token"},
		{"slack platform token (xoxp-)", "xoxp-" + "1234567890-abcdefghij", "Slack token"},
		{"aws access key (AKIA*)", "AKIAIOSFODNN7EXAMPLE", "AWS access key ID"},
		{"PEM RSA", "-----BEGIN RSA PRIVATE KEY-----", "PEM private key"},
		{"PEM OpenSSH", "-----BEGIN OPENSSH PRIVATE KEY-----", "PEM private key"},
		{"PEM EC", "-----BEGIN EC PRIVATE KEY-----", "PEM private key"},
		{"PEM generic", "-----BEGIN PRIVATE KEY-----", "PEM private key"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			yamlSrc := "project: test\nservices:\n  api:\n    leaked: " + tc.secret + "\n"
			err := ScanForSecrets([]byte(yamlSrc))
			if err == nil {
				t.Fatalf("expected match for %s but got nil", tc.name)
			}
			rerr, ok := err.(*errors.RaiozError)
			if !ok {
				t.Fatalf("expected *RaiozError, got %T", err)
			}
			if rerr.Code != errors.ErrCodeSecretInYAML {
				t.Errorf("expected code %s, got %s", errors.ErrCodeSecretInYAML, rerr.Code)
			}
			if rerr.Context["pattern"] != tc.pattern {
				t.Errorf("expected pattern %q in context, got %v", tc.pattern, rerr.Context["pattern"])
			}
		})
	}
}

// TestScanForSecrets_NeverLeaksSecret is load-bearing: the scanner's
// reason to exist is to keep credentials out of error output. If this
// test ever fails, the implementation has a critical bug — the very
// thing we promised the user.
func TestScanForSecrets_NeverLeaksSecret(t *testing.T) {
	secrets := []string{
		"ghp_" + strings.Repeat("a", 40),
		"glpat-" + strings.Repeat("b", 25),
		"AKIAIOSFODNN7EXAMPLE",
		// Built via concatenation so the literal `xoxb-<digits>-<word>`
		// pattern never appears in the source — keeps GitHub Push
		// Protection (and gitleaks/truffleHog) from flagging this fixture.
		"xoxb-" + "1234567890-superSecretToken",
		"-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----",
	}
	for _, secret := range secrets {
		t.Run(secret[:10]+"...", func(t *testing.T) {
			err := ScanForSecrets([]byte("project: test\nfoo: " + secret + "\n"))
			if err == nil {
				t.Fatal("expected error")
			}
			msg := err.Error()
			if strings.Contains(msg, secret) {
				t.Errorf("error message leaks the secret literal: %q", msg)
			}
			// Also check that the context map values don't contain it.
			if rerr, ok := err.(*errors.RaiozError); ok {
				for k, v := range rerr.Context {
					if s, ok := v.(string); ok && strings.Contains(s, secret) {
						t.Errorf("context[%q] leaks the secret: %q", k, s)
					}
				}
				if strings.Contains(rerr.Suggestion, secret) {
					t.Errorf("suggestion leaks the secret: %q", rerr.Suggestion)
				}
			}
		})
	}
}

func TestScanForSecrets_LineNumber(t *testing.T) {
	yamlSrc := "line1\nline2\nleak: ghp_" + strings.Repeat("a", 40) + "\nline4\n"
	err := ScanForSecrets([]byte(yamlSrc))
	if err == nil {
		t.Fatal("expected match")
	}
	rerr := err.(*errors.RaiozError)
	if got := rerr.Context["line"]; got != 3 {
		t.Errorf("expected line 3, got %v", got)
	}
}

func TestScanForSecrets_LineNumberFirstLine(t *testing.T) {
	yamlSrc := "ghp_" + strings.Repeat("a", 40) + "\nproject: test\n"
	err := ScanForSecrets([]byte(yamlSrc))
	if err == nil {
		t.Fatal("expected match")
	}
	rerr := err.(*errors.RaiozError)
	if got := rerr.Context["line"]; got != 1 {
		t.Errorf("expected line 1, got %v", got)
	}
}

func TestScanForSecrets_NoMatchOnCleanYAML(t *testing.T) {
	yamlSrc := `project: e-commerce
workspace: acme-corp
services:
  api:
    path: ./api
    dependsOn: [postgres]
    env: [.env.api]
  web:
    git: github.com/acme/web
    branch: develop
dependencies:
  postgres:
    image: postgres:16
    ports: ["5432"]
`
	if err := ScanForSecrets([]byte(yamlSrc)); err != nil {
		t.Errorf("expected nil for clean yaml, got: %v", err)
	}
}

func TestScanForSecrets_NoMatchOnShortPrefixes(t *testing.T) {
	// Strings that resemble token prefixes but are too short to satisfy
	// the regex length requirement — must NOT trigger.
	cases := []string{
		"project: test\n# fake: ghp_short\n",
		"project: test\n# fake: glpat-short\n",
		"project: test\n# fake: AKIAfoo\n",
		"project: test\n# example: xox-not-a-token\n",
		"project: test\nname: pem-utils\n", // mention of "pem" but no -----BEGIN
	}
	for i, src := range cases {
		t.Run(strings.Split(src, "\n")[1], func(t *testing.T) {
			if err := ScanForSecrets([]byte(src)); err != nil {
				t.Errorf("case %d: expected nil, got: %v", i, err)
			}
		})
	}
}

func TestScanForSecrets_EmptyInput(t *testing.T) {
	if err := ScanForSecrets(nil); err != nil {
		t.Errorf("expected nil for nil input, got: %v", err)
	}
	if err := ScanForSecrets([]byte{}); err != nil {
		t.Errorf("expected nil for empty input, got: %v", err)
	}
}

// TestScanForSecrets_PEMInCommentStillMatches documents the intentional
// behavior: a private key anywhere in the file is suspicious, even
// inside a comment. We do NOT exempt comments from the scan because
// the policy is structural ("yaml never carries credentials"), not
// contextual.
func TestScanForSecrets_PEMInCommentStillMatches(t *testing.T) {
	yamlSrc := "project: test\n# DO NOT commit:\n# -----BEGIN RSA PRIVATE KEY-----\n# end\n"
	err := ScanForSecrets([]byte(yamlSrc))
	if err == nil {
		t.Fatal("PEM marker in comment must still trigger detection")
	}
	rerr := err.(*errors.RaiozError)
	if rerr.Context["pattern"] != "PEM private key" {
		t.Errorf("expected PEM private key, got %v", rerr.Context["pattern"])
	}
}

func TestScanForSecrets_ReturnsFirstMatch(t *testing.T) {
	// Two distinct secrets — we don't care which one wins (pattern order
	// is an internal detail), just that exactly one error is returned
	// and it carries a known pattern name.
	yamlSrc := "AKIAIOSFODNN7EXAMPLE\nglpat-" + strings.Repeat("z", 25) + "\n"
	err := ScanForSecrets([]byte(yamlSrc))
	if err == nil {
		t.Fatal("expected match")
	}
	rerr := err.(*errors.RaiozError)
	if rerr.Code != errors.ErrCodeSecretInYAML {
		t.Errorf("expected SECRET_IN_YAML, got %s", rerr.Code)
	}
}

func TestLineOfByte(t *testing.T) {
	data := []byte("aaa\nbbb\nccc\nddd")
	cases := []struct {
		off  int
		want int
	}{
		{0, 1},
		{2, 1},
		{3, 1},  // the \n itself
		{4, 2},  // start of bbb
		{8, 3},  // start of ccc
		{12, 4}, // start of ddd
		{14, 4}, // inside ddd
		{99, 4}, // past end clamps to last line
	}
	for _, tc := range cases {
		if got := lineOfByte(data, tc.off); got != tc.want {
			t.Errorf("offset %d: want line %d, got %d", tc.off, tc.want, got)
		}
	}
}
