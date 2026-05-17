package app

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintSpawnEnv_RedactsSecretShapedKeys(t *testing.T) {
	t.Setenv("AWS_SECRET_ACCESS_KEY", "real-secret-value-do-not-leak")
	t.Setenv("GITHUB_TOKEN", "ghp_realtoken")
	t.Setenv("MY_API_KEY", "abcd1234")
	t.Setenv("HARMLESS_VAR", "plaintext-ok-to-show")

	var buf bytes.Buffer
	PrintSpawnEnv(&buf)
	out := buf.String()

	// Secret-shaped: name present, value redacted, value string NOT in output.
	for _, key := range []string{"AWS_SECRET_ACCESS_KEY", "GITHUB_TOKEN", "MY_API_KEY"} {
		if !strings.Contains(out, key+"=<redacted> [SECRET-SHAPED]") {
			t.Errorf("expected %q listed with [SECRET-SHAPED] redaction; output:\n%s",
				key, out)
		}
	}
	if strings.Contains(out, "real-secret-value-do-not-leak") ||
		strings.Contains(out, "ghp_realtoken") {
		t.Errorf("PrintSpawnEnv leaked a secret value:\n%s", out)
	}

	// Non-secret: full KEY=VALUE shown.
	if !strings.Contains(out, "HARMLESS_VAR=plaintext-ok-to-show") {
		t.Errorf("expected non-secret KEY=VALUE present; output:\n%s", out)
	}
}

func TestPrintSpawnEnv_ListsRaiozReadsSection(t *testing.T) {
	var buf bytes.Buffer
	PrintSpawnEnv(&buf)
	out := buf.String()

	for _, key := range []string{
		"RAIOZ_HOME",
		"RAIOZ_RUNTIME",
		"RAIOZ_LANG",
		"RAIOZ_SIBLING_TIMEOUT",
		"RAIOZ_LOCK_STALE_AGE",
		"RAIOZ_META_SUB_TIMEOUT",
		"RAIOZ_SIBLING_STACK",
		"RAIOZ_CORRELATION_ID",
		"RAIOZ_ROUTER_ACTIVE",
	} {
		if !strings.Contains(out, key) {
			t.Errorf("expected env var %q in raioz-reads section; output:\n%s",
				key, out)
		}
	}
	if !strings.Contains(out, "env -i HOME=$HOME") {
		t.Errorf("expected sandbox recommendation in output:\n%s", out)
	}
}