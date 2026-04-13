package cli

import (
	"testing"
)

func TestProxyCmd(t *testing.T) {
	if proxyCmd == nil {
		t.Fatal("proxyCmd should be initialized")
	}
	if proxyCmd.Use != "proxy" {
		t.Errorf("Use = %s, want proxy", proxyCmd.Use)
	}
	if proxyCmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if !proxyCmd.SilenceUsage {
		t.Error("SilenceUsage should be true")
	}
}

func TestProxyCmdRegisteredOnRoot(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "proxy" {
			found = true
			break
		}
	}
	if !found {
		t.Error("proxyCmd not registered on rootCmd")
	}
}

func TestProxySubcommandsRegistered(t *testing.T) {
	expected := []string{"status", "stop"}
	registered := make(map[string]bool)
	for _, sub := range proxyCmd.Commands() {
		registered[sub.Name()] = true
	}
	for _, name := range expected {
		if !registered[name] {
			t.Errorf("proxy subcommand %q not registered", name)
		}
	}
}

func TestProxyStatusCmdMetadata(t *testing.T) {
	if proxyStatusCmd.Use != "status" {
		t.Errorf("Use = %s, want status", proxyStatusCmd.Use)
	}
	if proxyStatusCmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestProxyStopCmdMetadata(t *testing.T) {
	if proxyStopCmd.Use != "stop" {
		t.Errorf("Use = %s, want stop", proxyStopCmd.Use)
	}
	if proxyStopCmd.Short == "" {
		t.Error("Short should not be empty")
	}
}
