package upcase

import (
	"testing"
)

// --- diagnoseContainerError additional patterns --------------------------------

func TestDiagnoseContainerErrorMySQLPassword(t *testing.T) {
	logs := "You need to specify one of MYSQL_ROOT_PASSWORD, MYSQL_ALLOW_EMPTY_PASSWORD"
	suggestions := diagnoseContainerError(logs, "mysql")
	if len(suggestions) == 0 {
		t.Fatal("expected suggestions for mysql root password error")
	}
	found := false
	for _, s := range suggestions {
		if strContains2(s, "MYSQL_ROOT_PASSWORD") {
			found = true
		}
	}
	if !found {
		t.Error("expected MYSQL_ROOT_PASSWORD in suggestions")
	}
}

func TestDiagnoseContainerErrorPortAlreadyInUse(t *testing.T) {
	logs := "bind: address already in use"
	suggestions := diagnoseContainerError(logs, "api")
	if len(suggestions) == 0 {
		t.Fatal("expected suggestions for address already in use")
	}
	found := false
	for _, s := range suggestions {
		if strContains2(s, "raioz down") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'raioz down' in suggestions")
	}
}

func TestDiagnoseContainerErrorPermissionDenied(t *testing.T) {
	logs := "permission denied on /var/lib/data"
	suggestions := diagnoseContainerError(logs, "db")
	if len(suggestions) == 0 {
		t.Fatal("expected suggestions")
	}
	found := false
	for _, s := range suggestions {
		if strContains2(s, "volume") || strContains2(s, "permission") {
			found = true
		}
	}
	if !found {
		t.Error("expected volume/permission suggestion")
	}
}

func TestDiagnoseContainerErrorGenericFallback(t *testing.T) {
	logs := "something unexpected happened"
	suggestions := diagnoseContainerError(logs, "svc")
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 generic suggestion, got %d", len(suggestions))
	}
	if !strContains2(suggestions[0], "raioz logs") {
		t.Errorf("expected 'raioz logs' suggestion, got %q", suggestions[0])
	}
}

func TestDiagnoseContainerErrorMultiplePatterns(t *testing.T) {
	// Logs that match both postgres and permission
	logs := "POSTGRES_PASSWORD not set\npermission denied on /data"
	suggestions := diagnoseContainerError(logs, "db")
	if len(suggestions) < 3 {
		t.Errorf("expected at least 3 suggestions (postgres + permission), got %d", len(suggestions))
	}
}

// strContains2 is a local helper to avoid conflicts with other test files.
func strContains2(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
