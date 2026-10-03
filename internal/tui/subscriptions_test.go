package tui

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// A row's container is found by label. It is this project's when it
// carries the project label, or when it is a workspace-shared dependency
// of the same workspace (those carry no project label).
func TestParseContainerLine(t *testing.T) {
	m := Model{config: Config{Project: "rzb1", Workspace: "rzbench"}}

	tests := []struct {
		name        string
		line        string
		wantOK      bool
		wantService string
		wantStatus  string
		wantHealth  string
	}{
		{"own service", "rzbench-rzb1-api|rzb1|rzbench|api|running|Up 2 minutes", true, "api", "running", ""},
		{"custom container_name", "rzb-site-custom|rzb1|rzbench|site|running|Up 2 minutes", true, "site", "running", ""},
		{"shared dep of the workspace", "rzbench-redis||rzbench|redis|running|Up 5 minutes (healthy)",
			true, "redis", "running", "healthy"},
		{"exited container", "rzbench-rzb1-api|rzb1|rzbench|api|exited|Exited (1) 3 seconds ago", true, "api", "exited", ""},
		{"unhealthy", "x|rzb1|rzbench|db|running|Up 1 minute (unhealthy)", true, "db", "running", "unhealthy"},
		{"another project", "conorbi-account|account|conorbi|account|running|Up 1 hour", false, "", "", ""},
		{"shared dep of another workspace", "conorbi-pg||conorbi|postgres|running|Up", false, "", "", ""},
		{"no workspace on either side is not a match", "x|||redis|running|Up", false, "", "", ""},
		{"malformed", "garbage", false, "", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, service, st, ok := m.parseContainerLine(tc.line)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if service != tc.wantService || st.Status != tc.wantStatus || st.Health != tc.wantHealth {
				t.Errorf("got %s/%s/%s, want %s/%s/%s",
					service, st.Status, st.Health, tc.wantService, tc.wantStatus, tc.wantHealth)
			}
		})
	}
}

func TestTailLines(t *testing.T) {
	got := tailLines("a\r\n\nb\nc\n   \nd\n", 3)
	if want := []string{"b", "c", "d"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := tailLines("", 3); len(got) != 0 {
		t.Errorf("empty text gave %v", got)
	}
}

// The dashboard acts on containers only: a host service row has none.
func TestContainerOf(t *testing.T) {
	m := Model{services: []ServiceRow{
		{Name: "api", Container: "rzbench-rzb1-api"},
		{Name: "web", Host: true},
	}}
	if c, err := m.containerOf("api"); err != nil || c != "rzbench-rzb1-api" {
		t.Errorf("api → %q, %v", c, err)
	}
	for _, name := range []string{"web", "ghost"} {
		if _, err := m.containerOf(name); err == nil {
			t.Errorf("%s has no container, want an error", name)
		}
	}
}

// A host service has no container: restart and stop go through HostAction,
// everything else keeps addressing the container.
func TestHostActionRouting(t *testing.T) {
	var calls []string
	m := New(Config{
		Ctx: context.Background(),
		Services: []ServiceRow{
			{Name: "api", Container: "rzbench-rzb1-api"},
			{Name: "web", Host: true},
		},
		HostAction: func(_ context.Context, action, service string) error {
			calls = append(calls, action+" "+service)
			return errors.New("the lock already exists")
		},
	})

	for _, action := range []string{"restart", "stop"} {
		handled, err := m.hostAction(action, "web")
		if !handled || err == nil {
			t.Errorf("%s on a host service: handled=%v err=%v", action, handled, err)
		}
	}
	if handled, _ := m.hostAction("restart", "api"); handled {
		t.Error("a container row must not go through HostAction")
	}
	if got := strings.Join(calls, ","); got != "restart web,stop web" {
		t.Errorf("calls = %q", got)
	}

	msg, ok := m.restartServiceCmd("web")().(ActionResultMsg)
	if !ok || msg.Err == nil || msg.Action != "restart" || msg.Service != "web" {
		t.Errorf("restart result = %+v", msg)
	}
	msg, ok = m.stopServiceCmd("web")().(ActionResultMsg)
	if !ok || msg.Err == nil || msg.Action != "stop" {
		t.Errorf("stop result = %+v", msg)
	}

	// Without the callback a host service stays read-only.
	ro := New(Config{Ctx: context.Background(), Services: []ServiceRow{{Name: "web", Host: true}}})
	if handled, _ := ro.hostAction("restart", "web"); handled {
		t.Error("no HostAction configured: nothing to handle")
	}
}
