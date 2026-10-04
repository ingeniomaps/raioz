package app

import (
	"context"
	"strings"
	"testing"

	"raioz/internal/domain/models"
)

// approveAll stands for a user who said yes; declineAll for one who said no.
func approveAll([]string) (bool, error) { return true, nil }
func declineAll([]string) (bool, error) { return false, nil }

func stubTerminal(t *testing.T, isTerminal bool) {
	t.Helper()
	prev := stdinIsTerminalFn
	stdinIsTerminalFn = func() bool { return isTerminal }
	t.Cleanup(func() { stdinIsTerminalFn = prev })
}

// Another project is never stopped unannounced: the list and the reason
// are printed even when --yes approved it up front.
func TestApproveStopping(t *testing.T) {
	initI18nForTest(t)

	t.Run("nothing to stop needs no approval", func(t *testing.T) {
		stubTerminal(t, false)
		ok, err := approveStopping(false, reasonConflicting)(nil)
		if !ok || err != nil {
			t.Errorf("ok=%v err=%v, want approved", ok, err)
		}
	})

	t.Run("yes approves and still names the projects", func(t *testing.T) {
		stubTerminal(t, false)
		var ok bool
		var err error
		out := captureStdout(t, func() {
			ok, err = approveStopping(true, reasonExclusive)([]string{"identity", "account"})
		})
		if !ok || err != nil {
			t.Fatalf("ok=%v err=%v, want approved", ok, err)
		}
		for _, name := range []string{"identity", "account"} {
			if !strings.Contains(out, name) {
				t.Errorf("project %q not announced:\n%s", name, out)
			}
		}
	})

	t.Run("no terminal and no yes refuses", func(t *testing.T) {
		stubTerminal(t, false)
		ok, err := approveStopping(false, reasonAllProjects)([]string{"identity"})
		if ok || err == nil {
			t.Errorf("ok=%v err=%v, want a refusal with an error", ok, err)
		}
	})
}

// A declined approval stops nothing, whichever command asked.
func TestStoppingOthers_DeclinedStopsNothing(t *testing.T) {
	initI18nForTest(t)
	cwd := &models.Deps{Project: models.Project{Name: "me"}}

	t.Run("down --conflicting", func(t *testing.T) {
		stubs := &downOthersStubs{conflicts: []portConflict{{Port: "5432", Project: "other"}}}
		withDownOthersHooks(t, stubs)
		got, err := DownConflictingProjects(context.Background(), cwd, "/tmp", declineAll)
		if err != nil || len(got) != 0 || len(stubs.stopProjectCalls) != 0 {
			t.Errorf("stopped=%v calls=%v err=%v, want nothing stopped", got, stubs.stopProjectCalls, err)
		}
	})

	t.Run("down --all-projects", func(t *testing.T) {
		stubs := &downOthersStubs{active: []string{"other", "me"}}
		withDownOthersHooks(t, stubs)
		got, err := DownAllOtherProjects(context.Background(), "me", declineAll)
		if err != nil || len(got) != 0 || len(stubs.stopProjectCalls) != 0 {
			t.Errorf("stopped=%v calls=%v err=%v, want nothing stopped", got, stubs.stopProjectCalls, err)
		}
	})

	t.Run("up --exclusive without a terminal does not start", func(t *testing.T) {
		stubTerminal(t, false)
		stubs := &downOthersStubs{active: []string{"other"}}
		withDownOthersHooks(t, stubs)
		uc := NewUpUseCase(&Dependencies{ConfigLoader: nil})
		proceed, err := uc.stopOtherProjects(context.Background(), "", false)
		if proceed || err == nil {
			t.Errorf("proceed=%v err=%v, want the up refused", proceed, err)
		}
		if len(stubs.stopProjectCalls) != 0 {
			t.Errorf("stopped %v without approval", stubs.stopProjectCalls)
		}
	})
}
