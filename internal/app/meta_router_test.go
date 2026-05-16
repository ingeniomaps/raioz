package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"raioz/internal/config"
)

// stageRecordingBinary writes a shell script that appends one line per
// invocation to outFile: `<subCmd>\t<cwd>\t<routerActive>` where
// routerActive is "1" / "" depending on whether RAIOZ_ROUTER_ACTIVE
// reached the sub-process. Returns the binary path and the output
// file path so callers can replay invocation history.
func stageRecordingBinary(t *testing.T) (binPath, outFile string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("recording binary uses /bin/sh")
	}
	dir := t.TempDir()
	binPath = filepath.Join(dir, "fake-raioz")
	outFile = filepath.Join(dir, "calls.log")
	body := "#!/bin/sh\n" +
		"echo \"$1\\t$PWD\\t${RAIOZ_ROUTER_ACTIVE}\" >> " + outFile + "\n" +
		"exit 0\n"
	if err := os.WriteFile(binPath, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	return binPath, outFile
}

// readCalls returns one entry per invocation recorded by the fake
// binary. Each entry is a 3-tuple {subCmd, cwd, routerActive}.
func readCalls(t *testing.T, file string) [][3]string {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read calls log: %v", err)
	}
	var out [][3]string
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		// Pad to 3 fields for missing env (empty router-active).
		for len(parts) < 3 {
			parts = append(parts, "")
		}
		out = append(out, [3]string{parts[0], parts[1], parts[2]})
	}
	return out
}

// makeMetaWithRouter stages two consumer dirs + a gateway dir, returns
// a MetaConfig whose Router points at gateway.
func makeMetaWithRouter(t *testing.T) *config.MetaConfig {
	t.Helper()
	base := t.TempDir()
	mk := func(n string) string {
		p := filepath.Join(base, n)
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	gateway := mk("gateway")
	api := mk("api")
	web := mk("web")
	return &config.MetaConfig{
		BaseDir: base,
		Projects: []config.MetaProject{
			{Name: "api", Path: api},
			{Name: "web", Path: web},
			{Name: "gateway", Path: gateway}, // also in projects: per ADR-037 example
		},
		Router: &config.MetaProject{Name: "gateway", Path: gateway},
	}
}

// Router declared → first up is the router, then consumers; consumers
// receive RAIOZ_ROUTER_ACTIVE=1; router itself does NOT.
func TestMetaRunner_RouterUpFirstAndPropagates(t *testing.T) {
	bin, log := stageRecordingBinary(t)
	cfg := makeMetaWithRouter(t)
	r := &MetaRunner{Binary: bin}

	summary := r.Up(context.Background(), cfg, nil, nil, MetaUpOptions{})

	calls := readCalls(t, log)
	if len(calls) != 3 {
		t.Fatalf("expected 3 invocations (router + 2 consumers), got %d: %+v",
			len(calls), calls)
	}
	// Order: router first.
	if !strings.HasSuffix(calls[0][1], "gateway") {
		t.Errorf("call[0] cwd = %q, want trailing 'gateway'", calls[0][1])
	}
	// Router itself runs without RAIOZ_ROUTER_ACTIVE.
	if calls[0][2] != "" {
		t.Errorf("router invocation got RAIOZ_ROUTER_ACTIVE=%q, want empty",
			calls[0][2])
	}
	// Consumers get the env var.
	for i := 1; i < len(calls); i++ {
		if calls[i][2] != "1" {
			t.Errorf("consumer call[%d] RAIOZ_ROUTER_ACTIVE=%q, want 1",
				i, calls[i][2])
		}
	}
	if summary.HasFailures() {
		t.Errorf("expected no failures, got %+v", summary)
	}
}

// Router declared → gateway entry in projects: must not be invoked a
// second time in the consumer loop.
func TestMetaRunner_RouterSkippedFromConsumerLoop(t *testing.T) {
	bin, log := stageRecordingBinary(t)
	cfg := makeMetaWithRouter(t) // gateway is in both router: and projects:
	r := &MetaRunner{Binary: bin}

	r.Up(context.Background(), cfg, nil, nil, MetaUpOptions{})

	calls := readCalls(t, log)
	gatewayHits := 0
	for _, c := range calls {
		if strings.HasSuffix(c[1], "gateway") {
			gatewayHits++
		}
	}
	if gatewayHits != 1 {
		t.Errorf("gateway invoked %d times, want exactly 1 (router phase only)",
			gatewayHits)
	}
}

// --router-off bypasses the router phase entirely. Gateway, if listed in
// projects:, gets up'd as a normal consumer with no router env var.
func TestMetaRunner_RouterOffBypasses(t *testing.T) {
	bin, log := stageRecordingBinary(t)
	cfg := makeMetaWithRouter(t)
	r := &MetaRunner{Binary: bin}

	r.Up(context.Background(), cfg, nil, nil, MetaUpOptions{RouterOff: true})

	calls := readCalls(t, log)
	// 3 projects, no special router phase → 3 consumer-style invocations,
	// none with the router env var.
	if len(calls) != 3 {
		t.Fatalf("expected 3 invocations, got %d", len(calls))
	}
	for i, c := range calls {
		if c[2] != "" {
			t.Errorf("call[%d] RAIOZ_ROUTER_ACTIVE=%q under --router-off, want empty",
				i, c[2])
		}
	}
}

// Router up failure aborts before any consumer runs.
func TestMetaRunner_RouterUpFailureAborts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("/bin/sh")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-raioz")
	log := filepath.Join(dir, "calls.log")
	// Script: fail when cwd ends with "gateway", succeed elsewhere.
	body := "#!/bin/sh\n" +
		"echo \"$1\\t$PWD\" >> " + log + "\n" +
		"case \"$PWD\" in *gateway) exit 1 ;; esac\n" +
		"exit 0\n"
	if err := os.WriteFile(bin, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	cfg := makeMetaWithRouter(t)
	r := &MetaRunner{Binary: bin}

	summary := r.Up(context.Background(), cfg, nil, nil, MetaUpOptions{})

	if !summary.HasFailures() {
		t.Errorf("expected failures when router up fails, got %+v", summary)
	}
	data, _ := os.ReadFile(log)
	if strings.Count(string(data), "\n") != 1 {
		t.Errorf("expected only the router invocation to be recorded, got log:\n%s",
			string(data))
	}
}

// Down: router goes LAST; consumers first in reverse order. Gateway
// must appear exactly once.
func TestMetaRunner_RouterDownsLast(t *testing.T) {
	bin, log := stageRecordingBinary(t)
	cfg := makeMetaWithRouter(t)
	r := &MetaRunner{Binary: bin}

	r.Down(context.Background(), cfg, nil)

	calls := readCalls(t, log)
	if len(calls) != 3 {
		t.Fatalf("expected 3 invocations, got %d: %+v", len(calls), calls)
	}
	if !strings.HasSuffix(calls[len(calls)-1][1], "gateway") {
		t.Errorf("last down call = %q, want trailing 'gateway'",
			calls[len(calls)-1][1])
	}
	gatewayHits := 0
	for _, c := range calls {
		if strings.HasSuffix(c[1], "gateway") {
			gatewayHits++
		}
	}
	if gatewayHits != 1 {
		t.Errorf("gateway visited %d times in down, want exactly 1", gatewayHits)
	}
}

// stageConditionalBinary writes a fake raioz that fails (exit 1) when
// invoked from any cwd whose basename matches one of `failNames`, and
// succeeds otherwise. Used to exercise per-sub partial-failure paths
// without scaffolding multiple binaries.
func stageConditionalBinary(t *testing.T, failNames ...string) (binPath, logFile string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("recording binary uses /bin/sh")
	}
	dir := t.TempDir()
	binPath = filepath.Join(dir, "fake-raioz")
	logFile = filepath.Join(dir, "calls.log")
	body := "#!/bin/sh\n" +
		"echo \"$1\\t$PWD\\t${RAIOZ_ROUTER_ACTIVE}\" >> " + logFile + "\n" +
		"name=$(basename \"$PWD\")\n"
	body += "case \"$name\" in\n"
	for _, n := range failNames {
		body += "  " + n + ") exit 1 ;;\n"
	}
	body += "esac\nexit 0\n"
	if err := os.WriteFile(binPath, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	return binPath, logFile
}

// When the router is up and an OPTIONAL consumer fails, the meta run
// must continue (the rest of the consumers come up) and the failure
// must surface as Skipped, not HasFailures.
func TestMetaRunner_RouterStaysUpWhenOptionalConsumerFails(t *testing.T) {
	bin, _ := stageConditionalBinary(t, "api")
	cfg := makeMetaWithRouter(t)
	// makeMetaWithRouter declares projects in order: api, web, gateway.
	// Mark api as optional so its exit-1 doesn't abort the meta up.
	cfg.Projects[0].Optional = true

	r := &MetaRunner{Binary: bin}
	summary := r.Up(context.Background(), cfg, nil, nil, MetaUpOptions{})

	if summary.HasFailures() {
		t.Errorf("optional failure must not mark HasFailures(); summary=%+v", summary)
	}
	// Find the api entry — should be Skipped with the err recorded.
	var apiEntry *MetaSummary
	for i := range summary {
		if summary[i].Project == "api" {
			apiEntry = &summary[i]
		}
	}
	if apiEntry == nil {
		t.Fatalf("api entry missing from summary: %+v", summary)
	}
	if !apiEntry.Skipped {
		t.Errorf("api entry must be Skipped, got %+v", apiEntry)
	}
	if apiEntry.Err == nil {
		t.Errorf("api entry must record the underlying error")
	}
}

// --audit-siblings flags a meta sub-project whose yaml trips H3
// (un-pinned image). The whole meta up aborts before any spawn, and
// the summary names the audit phase as the failure.
func TestMetaRunner_AuditSiblingsRejectsUnpinnedImage(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX path joins")
	}
	bin, _ := stageRecordingBinary(t)

	base := t.TempDir()
	mk := func(n, yaml string) string {
		p := filepath.Join(base, n)
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "raioz.yaml"),
			[]byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	good := mk("good", "project: good\nservices:\n  api:\n    path: .\n")
	bad := mk("bad", "project: bad\nservices:\n  api:\n    path: .\n"+
		"dependencies:\n  redis:\n    image: redis:latest\n")

	cfg := &config.MetaConfig{
		BaseDir: base,
		Projects: []config.MetaProject{
			{Name: "good", Path: good},
			{Name: "bad", Path: bad},
		},
	}
	r := &MetaRunner{Binary: bin}
	summary := r.Up(context.Background(), cfg, nil, nil,
		MetaUpOptions{AuditSiblings: true})

	if !summary.HasFailures() {
		t.Errorf("audit-siblings must abort meta up on H3 trip, summary=%+v",
			summary)
	}
	if len(summary) != 1 || summary[0].Project != "audit-siblings" {
		t.Errorf("expected single audit-siblings failure entry, got %+v",
			summary)
	}
}

// The router runs regardless of `--meta-profile` (it owns the edge;
// profiles only filter consumers). Guards against a future "profile
// filter applied to router" regression.
func TestMetaRunner_RouterRunsRegardlessOfProfile(t *testing.T) {
	bin, log := stageRecordingBinary(t)
	cfg := makeMetaWithRouter(t)
	// Tag every consumer with an opt-in profile that the caller does
	// NOT activate. The only project that must still run is the
	// router (declared via cfg.Router, not via Projects[].Profiles).
	for i := range cfg.Projects {
		cfg.Projects[i].Profiles = []string{"opt-in"}
	}

	r := &MetaRunner{Binary: bin}
	r.Up(context.Background(), cfg, nil, nil /* activeProfiles */, MetaUpOptions{})

	calls := readCalls(t, log)
	// At least one call, and the FIRST one is the router (gateway).
	if len(calls) == 0 {
		t.Fatalf("expected at least the router invocation, got none")
	}
	if !strings.HasSuffix(calls[0][1], "gateway") {
		t.Errorf("router phase must run even when profiles exclude all "+
			"consumers; first call cwd = %q", calls[0][1])
	}
	// And it must run without RAIOZ_ROUTER_ACTIVE (the router itself
	// is the proxy — it owns the edge).
	if calls[0][2] != "" {
		t.Errorf("router invocation must not see RAIOZ_ROUTER_ACTIVE, got %q",
			calls[0][2])
	}
}
