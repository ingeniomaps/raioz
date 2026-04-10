package upcase

import (
	"testing"
)

func TestDiagnoseContainerError_Postgres(t *testing.T) {
	logs := "Error: Database is uninitialized and superuser password is not specified.\nYou must specify POSTGRES_PASSWORD"
	suggestions := diagnoseContainerError(logs, "postgres")
	if len(suggestions) == 0 {
		t.Fatal("expected suggestions for postgres password error")
	}
	found := false
	for _, s := range suggestions {
		if strContains(s, "POSTGRES_PASSWORD") {
			found = true
		}
	}
	if !found {
		t.Error("expected POSTGRES_PASSWORD in suggestions")
	}
}

func TestDiagnoseContainerError_MySQL(t *testing.T) {
	logs := "error: database is uninitialized and password option is not specified\nYou need to specify one of MYSQL_ROOT_PASSWORD"
	suggestions := diagnoseContainerError(logs, "mysql")
	if len(suggestions) == 0 {
		t.Fatal("expected suggestions for mysql password error")
	}
}

func TestDiagnoseContainerError_PortInUse(t *testing.T) {
	logs := "Error: bind: address already in use"
	suggestions := diagnoseContainerError(logs, "api")
	if len(suggestions) == 0 {
		t.Fatal("expected suggestions for port in use")
	}
}

func TestDiagnoseContainerError_Generic(t *testing.T) {
	logs := "some unknown error"
	suggestions := diagnoseContainerError(logs, "myservice")
	if len(suggestions) == 0 {
		t.Fatal("expected generic suggestion")
	}
	found := false
	for _, s := range suggestions {
		if strContains(s, "raioz logs") {
			found = true
		}
	}
	if !found {
		t.Error("expected 'raioz logs' in generic suggestion")
	}
}

func strContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
