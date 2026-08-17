package docker

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"raioz/internal/naming"
	rt "raioz/internal/runtime"
)

// fakeDocker installs a stand-in for the container runtime that answers
// by subcommand. The map is keyed on the first argument (inspect, ps,
// …); a value of "" plus a non-zero code simulates the daemon rejecting
// the call. Every invocation appends its argv to the returned file, so a
// test can assert what raioz asked docker for — the flags are the
// contract, and a wrong filter silently returns the wrong containers.
func fakeDocker(t *testing.T, replies map[string]fakeReply) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake binary scripts are POSIX-only")
	}
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args.txt")

	var sb strings.Builder
	sb.WriteString("#!/bin/sh\n")
	sb.WriteString("for a in \"$@\"; do echo \"$a\" >> " + argsFile + "; done\n")
	// Match the two-word key first ("volume inspect") so a test can give
	// different answers to subcommands that share a first word.
	sb.WriteString("case \"$1 $2\" in\n")
	for sub, reply := range replies {
		if !strings.Contains(sub, " ") {
			continue
		}
		sb.WriteString("  \"" + sub + "\")\n")
		if reply.stdout != "" {
			sb.WriteString("    printf '%s' '" + reply.stdout + "'\n")
		}
		sb.WriteString("    exit " + itoa(reply.exitCode) + " ;;\n")
	}
	sb.WriteString("esac\n")
	sb.WriteString("case \"$1\" in\n")
	for sub, reply := range replies {
		if strings.Contains(sub, " ") {
			continue
		}
		sb.WriteString("  " + sub + ")\n")
		if reply.stdout != "" {
			sb.WriteString("    printf '%s' '" + reply.stdout + "'\n")
		}
		sb.WriteString("    exit " + itoa(reply.exitCode) + " ;;\n")
	}
	sb.WriteString("  *) exit 0 ;;\nesac\n")

	bin := filepath.Join(dir, "fakedocker")
	if err := os.WriteFile(bin, []byte(sb.String()), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	prev := rt.Binary()
	rt.SetBinary(bin)
	t.Cleanup(func() { rt.SetBinary(prev) })
	return argsFile
}

type fakeReply struct {
	stdout   string
	exitCode int
}

func fakeArgs(t *testing.T, argsFile string) string {
	t.Helper()
	data, err := os.ReadFile(argsFile)
	if err != nil {
		return ""
	}
	return string(data)
}

// The label probe answers "does this container belong to project X?".
// Docker prints "<no value>" for an absent key, and reading that string
// as a project name would make every unlabeled container look owned.
func TestGetContainerLabel(t *testing.T) {
	cases := []struct {
		name    string
		reply   fakeReply
		want    string
		wantErr bool
	}{
		{name: "label present", reply: fakeReply{stdout: "acme\n"}, want: "acme"},
		{name: "label absent", reply: fakeReply{stdout: "<no value>\n"}},
		{name: "container missing", reply: fakeReply{exitCode: 1}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeDocker(t, map[string]fakeReply{"inspect": tc.reply})
			got, err := GetContainerLabel(context.Background(), "c1", naming.LabelProject)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("GetContainerLabel() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGetContainerLabelEmptyArgs(t *testing.T) {
	argsFile := fakeDocker(t, map[string]fakeReply{"inspect": {stdout: "x"}})
	for _, tc := range []struct{ name, key string }{
		{name: "", key: "k"},
		{name: "c", key: ""},
	} {
		got, err := GetContainerLabel(context.Background(), tc.name, tc.key)
		if got != "" || err != nil {
			t.Errorf("GetContainerLabel(%q, %q) = %q, %v; want empty", tc.name, tc.key, got, err)
		}
	}
	if fakeArgs(t, argsFile) != "" {
		t.Error("an empty name or key must not reach docker at all")
	}
}

// The reconcile and status paths branch on this string, so an unknown
// container has to come back empty rather than as an error.
func TestGetContainerStatusByName(t *testing.T) {
	cases := []struct {
		name  string
		reply fakeReply
		want  string
	}{
		{name: "running", reply: fakeReply{stdout: "running\n"}, want: "running"},
		{name: "exited", reply: fakeReply{stdout: "exited\n"}, want: "exited"},
		{name: "unknown container", reply: fakeReply{exitCode: 1}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeDocker(t, map[string]fakeReply{"inspect": tc.reply})
			got, err := GetContainerStatusByName(context.Background(), "c1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("GetContainerStatusByName() = %q, want %q", got, tc.want)
			}
		})
	}
}

// The down flow deletes state when this comes back empty, so the
// error-surfacing variant has to distinguish "no containers" from "the
// daemon is unreachable" — that is the whole reason it exists.
func TestListContainersByLabelsErr(t *testing.T) {
	t.Run("names parsed", func(t *testing.T) {
		argsFile := fakeDocker(t, map[string]fakeReply{"ps": {stdout: "a\nb\n"}})
		got, err := ListContainersByLabelsErr(context.Background(),
			map[string]string{naming.LabelManaged: "true"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 || got[0] != "a" || got[1] != "b" {
			t.Errorf("names = %v, want [a b]", got)
		}
		if args := fakeArgs(t, argsFile); !strings.Contains(args, "label="+naming.LabelManaged+"=true") {
			t.Errorf("the label filter must reach docker, got:\n%s", args)
		}
	})

	t.Run("no matches", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{"ps": {stdout: "\n"}})
		got, err := ListContainersByLabelsErr(context.Background(),
			map[string]string{naming.LabelManaged: "true"})
		if err != nil || got != nil {
			t.Errorf("got %v, %v; want nil, nil", got, err)
		}
	})

	t.Run("daemon unreachable surfaces", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{"ps": {exitCode: 1}})
		if _, err := ListContainersByLabelsErr(context.Background(),
			map[string]string{naming.LabelManaged: "true"}); err == nil {
			t.Error("a docker failure must not read as an empty result")
		}
	})

	t.Run("no labels short-circuits", func(t *testing.T) {
		argsFile := fakeDocker(t, map[string]fakeReply{"ps": {stdout: "a\n"}})
		got, err := ListContainersByLabelsErr(context.Background(), nil)
		if got != nil || err != nil {
			t.Errorf("got %v, %v; want nil, nil", got, err)
		}
		if fakeArgs(t, argsFile) != "" {
			t.Error("an unfiltered ps would match every container on the machine")
		}
	})
}

// The swallowing variant is for best-effort callers: a docker failure
// has to look like "nothing to do", never like an error to propagate.
func TestListContainersByLabelsSwallowsErrors(t *testing.T) {
	fakeDocker(t, map[string]fakeReply{"ps": {exitCode: 1}})
	if got := ListContainersByLabels(context.Background(),
		map[string]string{naming.LabelManaged: "true"}); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestLookupAdapter(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{"inspect": {stdout: "running\n"}})
		ok, err := NewLookup().Exists(context.Background(), "c1")
		if err != nil || !ok {
			t.Errorf("Exists() = %v, %v; want true, nil", ok, err)
		}
	})

	t.Run("absent", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{"inspect": {exitCode: 1}})
		ok, err := NewLookup().Exists(context.Background(), "c1")
		if err != nil || ok {
			t.Errorf("Exists() = %v, %v; want false, nil", ok, err)
		}
	})

	t.Run("find by labels", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{"ps": {stdout: "x\n"}})
		got := NewLookup().FindByLabels(context.Background(),
			map[string]string{naming.LabelManaged: "true"})
		if len(got) != 1 || got[0] != "x" {
			t.Errorf("FindByLabels() = %v, want [x]", got)
		}
	})
}

// `down --conflicting` stops other projects by these two calls, so a
// missing project label must not turn into a container named "" and the
// list has to come back deduplicated.
func TestListActiveProjects(t *testing.T) {
	fakeDocker(t, map[string]fakeReply{"ps": {stdout: "beta\nalpha\nbeta\n\n"}})

	got, err := ListActiveProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Errorf("ListActiveProjects() = %v, want [alpha beta]", got)
	}
}

func TestListActiveProjectsDockerFailure(t *testing.T) {
	fakeDocker(t, map[string]fakeReply{"ps": {exitCode: 1}})
	if _, err := ListActiveProjects(context.Background()); err == nil {
		t.Error("expected the docker failure to surface")
	}
}

func TestStopProjectContainers(t *testing.T) {
	t.Run("stops what it finds", func(t *testing.T) {
		argsFile := fakeDocker(t, map[string]fakeReply{
			"ps": {stdout: "acme-api\nacme-web\n"},
			"rm": {},
		})
		got, err := StopProjectContainers(context.Background(), "acme")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("stopped = %v, want both containers", got)
		}
		if args := fakeArgs(t, argsFile); !strings.Contains(args, "label="+naming.LabelProject+"=acme") {
			t.Errorf("the project filter must reach docker, got:\n%s", args)
		}
	})

	t.Run("nothing to stop", func(t *testing.T) {
		fakeDocker(t, map[string]fakeReply{"ps": {stdout: "\n"}})
		got, err := StopProjectContainers(context.Background(), "acme")
		if err != nil || got != nil {
			t.Errorf("got %v, %v; want nil, nil", got, err)
		}
	})

	t.Run("project name is required", func(t *testing.T) {
		argsFile := fakeDocker(t, map[string]fakeReply{"ps": {stdout: "x\n"}})
		if _, err := StopProjectContainers(context.Background(), ""); err == nil {
			t.Error("an empty project would match containers by an empty label")
		}
		if fakeArgs(t, argsFile) != "" {
			t.Error("the empty-name guard must run before touching docker")
		}
	})
}
