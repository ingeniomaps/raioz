package app

import (
	"os"
	"path/filepath"
	"testing"

	"raioz/internal/config"
)

func TestRouterHandoffEnv_ComputesIPFromFirstConsumerSubnet(t *testing.T) {
	base := t.TempDir()
	consumer := filepath.Join(base, "api")
	if err := os.MkdirAll(consumer, 0755); err != nil {
		t.Fatal(err)
	}
	yaml := `project: api
workspace: acme
network:
  subnet: 172.28.0.0/16
services:
  api:
    path: .
`
	if err := os.WriteFile(filepath.Join(consumer, "raioz.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.MetaConfig{
		Workspace: "acme",
		Projects:  []config.MetaProject{{Name: "api", Path: consumer}},
	}
	env := routerHandoffEnv(cfg)

	want := "RAIOZ_ROUTER_ASSIGNED_IP=172.28.1.1"
	if len(env) != 1 || env[0] != want {
		t.Errorf("env = %v, want [%q]", env, want)
	}
}

// No consumer declares a subnet → handoff env is empty; router falls
// back to Docker auto-IP. /etc/hosts stays the operator's problem in
// that mode (existing behaviour).
func TestRouterHandoffEnv_EmptyWhenNoSubnet(t *testing.T) {
	base := t.TempDir()
	consumer := filepath.Join(base, "api")
	_ = os.MkdirAll(consumer, 0755)
	_ = os.WriteFile(filepath.Join(consumer, "raioz.yaml"),
		[]byte("project: api\nservices:\n  api:\n    path: .\n"), 0644)

	cfg := &config.MetaConfig{
		Projects: []config.MetaProject{{Name: "api", Path: consumer}},
	}
	if env := routerHandoffEnv(cfg); len(env) != 0 {
		t.Errorf("expected empty env when no subnet, got %v", env)
	}
}
