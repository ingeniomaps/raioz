package upcase

import (
	"context"
	goerrors "errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"raioz/internal/config"
	"raioz/internal/domain/models"
	errpkg "raioz/internal/errors"
	"raioz/internal/protocol"
	rt "raioz/internal/runtime"
)

// writeFakeRuntime points runtime.SetBinary at a shell script that
// echoes `stdout` and exits 0. Used to mock docker.IsProjectActive
// without a daemon.
func writeFakeRuntime(t *testing.T, stdout string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake binary scripts are POSIX-only")
	}
	dir := t.TempDir()
	body := "#!/bin/sh\nprintf '%s' '" + stdout + "'\nexit 0\n"
	p := filepath.Join(dir, "fakedocker")
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	prev := rt.Binary()
	rt.SetBinary(p)
	t.Cleanup(func() { rt.SetBinary(prev) })
}

func writeSiblingYAML(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "raioz.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	return dir
}

// --- non-sibling deps ----------------------------------------------------

func TestDecideSibling_NilInline(t *testing.T) {
	got, err := decideSibling(context.Background(), "redis", nil, "ws")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Kind != siblingProceed {
		t.Errorf("expected siblingProceed for nil inline, got %d", got.Kind)
	}
}

func TestDecideSibling_RegularImageDep(t *testing.T) {
	inline := &models.Infra{Image: "postgres", Tag: "16"}
	got, err := decideSibling(context.Background(), "postgres", inline, "ws")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Kind != siblingProceed {
		t.Errorf("regular dep should proceed, got %d", got.Kind)
	}
}

// --- mode A (project:) ---------------------------------------------------

func TestDecideSibling_ModeA_SiblingActive_SkipsSpawn(t *testing.T) {
	siblingDir := writeSiblingYAML(t,
		"workspace: hypixo\nproject: keycloak\n"+
			"services:\n  keycloak:\n    path: .\n")
	writeFakeRuntime(t, "hypixo-keycloak\n") // sibling reported active

	inline := &models.Infra{Project: siblingDir}
	got, err := decideSibling(context.Background(), "keycloak", inline, "hypixo")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Kind != siblingSkipModeA {
		t.Errorf("expected siblingSkipModeA when sibling is up, got %d", got.Kind)
	}
	if got.SiblingInfo == nil || got.SiblingInfo.Project != "keycloak" {
		t.Errorf("SiblingInfo not populated correctly: %+v", got.SiblingInfo)
	}
}

func TestDecideSibling_ModeA_SiblingInactive_RequestsSpawn(t *testing.T) {
	siblingDir := writeSiblingYAML(t,
		"workspace: hypixo\nproject: keycloak\n"+
			"services:\n  keycloak:\n    path: .\n")
	writeFakeRuntime(t, "") // empty docker ps → not active

	inline := &models.Infra{Project: siblingDir}
	got, err := decideSibling(context.Background(), "keycloak", inline, "hypixo")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Kind != siblingSpawnModeA {
		t.Errorf("expected siblingSpawnModeA, got %d", got.Kind)
	}
	if got.SiblingInfo == nil || got.SiblingInfo.Dir != siblingDir {
		t.Errorf("SiblingInfo not populated: %+v", got.SiblingInfo)
	}
}

func TestDecideSibling_ModeA_DetectsCycle(t *testing.T) {
	siblingDir := writeSiblingYAML(t,
		"workspace: hypixo\nproject: keycloak\n"+
			"services:\n  keycloak:\n    path: .\n")
	// Pretend we're already mid-spawn for siblingDir — simulate the
	// child raioz process running in a recursive chain.
	t.Setenv(protocol.SiblingStack, siblingDir)

	inline := &models.Infra{Project: siblingDir}
	_, err := decideSibling(context.Background(), "keycloak", inline, "hypixo")
	if err == nil || !strings.Contains(err.Error(), "sibling cycle") {
		t.Errorf("expected sibling-cycle error, got %v", err)
	}
}

// --- mode B (siblingProject:) --------------------------------------------

func TestDecideSibling_ModeB_SiblingActive(t *testing.T) {
	siblingDir := writeSiblingYAML(t,
		"workspace: hypixo\nproject: keycloak\n"+
			"services:\n  keycloak:\n    path: .\n")
	writeFakeRuntime(t, "hypixo-keycloak\n") // docker ps reports the container

	inline := &models.Infra{
		Image:          "keycloak",
		Tag:            "24",
		SiblingProject: siblingDir,
	}
	got, err := decideSibling(context.Background(), "keycloak", inline, "hypixo")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Kind != siblingSkipDeferred {
		t.Errorf("expected siblingSkipDeferred, got %d (reason: %s)", got.Kind, got.Reason)
	}
	if got.SiblingName != "keycloak" {
		t.Errorf("SiblingName = %q, want keycloak", got.SiblingName)
	}
}

func TestDecideSibling_ModeB_SiblingInactive(t *testing.T) {
	siblingDir := writeSiblingYAML(t,
		"workspace: hypixo\nproject: keycloak\n"+
			"services:\n  keycloak:\n    path: .\n")
	writeFakeRuntime(t, "") // docker ps returns nothing → not active

	inline := &models.Infra{
		Image:          "keycloak",
		SiblingProject: siblingDir,
	}
	got, err := decideSibling(context.Background(), "keycloak", inline, "hypixo")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Kind != siblingProceed {
		t.Errorf("inactive sibling should fall back to image, got %d", got.Kind)
	}
}

func TestDecideSibling_ModeB_SiblingMissing(t *testing.T) {
	inline := &models.Infra{
		Image:          "keycloak",
		SiblingProject: "/does/not/exist",
	}
	_, err := decideSibling(context.Background(), "keycloak", inline, "hypixo")
	if err == nil || !strings.Contains(err.Error(), "resolve sibling") {
		t.Errorf("expected resolve-sibling error, got %v", err)
	}
}

// --- resolveSiblingVerdicts ----------------------------------------------

func TestResolveSiblingVerdicts_DispatchCountSkipsSiblingDeferred(t *testing.T) {
	siblingDir := writeSiblingYAML(t,
		"workspace: hypixo\nproject: keycloak\n"+
			"services:\n  keycloak:\n    path: .\n")
	writeFakeRuntime(t, "hypixo-keycloak\n") // sibling reported active

	deps := &models.Deps{
		Workspace: "hypixo",
		Infra: map[string]models.InfraEntry{
			"keycloak": {Inline: &models.Infra{
				Image: "keycloak", Tag: "24", SiblingProject: siblingDir,
			}},
			"postgres": {Inline: &models.Infra{Image: "postgres", Tag: "16"}},
			"redis":    {Inline: &models.Infra{Image: "redis", Tag: "7"}},
		},
	}
	names := []string{"keycloak", "postgres", "redis"}

	verdicts, toDispatch, err := resolveSiblingVerdicts(
		context.Background(), names, deps)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if toDispatch != 2 {
		t.Errorf("expected toDispatch=2 (postgres + redis), got %d", toDispatch)
	}
	if verdicts["keycloak"].Kind != siblingSkipDeferred {
		t.Errorf("keycloak should be deferred, got %d", verdicts["keycloak"].Kind)
	}
	if verdicts["postgres"].Kind != siblingProceed {
		t.Errorf("postgres should proceed, got %d", verdicts["postgres"].Kind)
	}
	if verdicts["redis"].Kind != siblingProceed {
		t.Errorf("redis should proceed, got %d", verdicts["redis"].Kind)
	}
}

func TestResolveSiblingVerdicts_AllRegular(t *testing.T) {
	deps := &models.Deps{
		Infra: map[string]models.InfraEntry{
			"postgres": {Inline: &models.Infra{Image: "postgres", Tag: "16"}},
			"redis":    {Inline: &models.Infra{Image: "redis", Tag: "7"}},
		},
	}
	_, toDispatch, err := resolveSiblingVerdicts(
		context.Background(), []string{"postgres", "redis"}, deps)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if toDispatch != 2 {
		t.Errorf("toDispatch should equal infraNames when no siblings, got %d",
			toDispatch)
	}
}

func TestResolveSiblingVerdicts_PropagatesError(t *testing.T) {
	deps := &models.Deps{
		Workspace: "hypixo",
		Infra: map[string]models.InfraEntry{
			"keycloak": {Inline: &models.Infra{
				SiblingProject: "/does/not/exist",
				Image:          "keycloak",
			}},
		},
	}
	_, _, err := resolveSiblingVerdicts(
		context.Background(), []string{"keycloak"}, deps)
	if err == nil || !strings.Contains(err.Error(), "resolve sibling") {
		t.Errorf("expected resolve-sibling error, got %v", err)
	}
}

// --- applySiblingVerdict --------------------------------------------------

func TestApplySiblingVerdict_RegularDepProceeds(t *testing.T) {
	det := DetectionMap{"redis": {}}
	deferred := []string{}
	skip, err := applySiblingVerdict(
		context.Background(), "redis",
		siblingDecision{Kind: siblingProceed},
		"/consumer", det, &deferred,
	)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if skip {
		t.Error("regular dep should NOT be skipped")
	}
	if _, ok := det["redis"]; !ok {
		t.Error("regular dep should remain in detections")
	}
	if len(deferred) != 0 {
		t.Errorf("deferred should be empty, got %v", deferred)
	}
}

func TestApplySiblingVerdict_ModeBDeferred_RemovesFromDetectionsAndStamps(t *testing.T) {
	det := DetectionMap{"redis": {}, "keycloak": {}}
	deferred := []string{}
	skip, err := applySiblingVerdict(
		context.Background(), "keycloak",
		siblingDecision{
			Kind:        siblingSkipDeferred,
			SiblingName: "keycloak",
			Reason:      "sibling active",
		},
		"/consumer", det, &deferred,
	)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !skip {
		t.Error("deferred dep should be skipped")
	}
	if _, ok := det["keycloak"]; ok {
		t.Error("deferred dep MUST be removed from detections so downstream consumers don't generate routes/env vars for it")
	}
	if _, ok := det["redis"]; !ok {
		t.Error("non-sibling dep should remain in detections")
	}
	if len(deferred) != 1 || deferred[0] != "keycloak" {
		t.Errorf("deferred = %v, want [keycloak]", deferred)
	}
}

func TestApplySiblingVerdict_SkipModeA_RemovesFromDetections_NoDefer(t *testing.T) {
	det := DetectionMap{"keycloak": {}}
	deferred := []string{}
	skip, err := applySiblingVerdict(
		context.Background(), "keycloak",
		siblingDecision{
			Kind:        siblingSkipModeA,
			SiblingName: "keycloak",
		},
		"/consumer", det, &deferred,
	)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !skip {
		t.Error("mode A skip should be skipped")
	}
	if _, ok := det["keycloak"]; ok {
		t.Error("mode A dep MUST be removed from detections")
	}
	// Mode A is NEVER deferred — sibling has its own lifecycle, down
	// is governed by `entry.Inline.Project != ""`, not the deferred
	// stamp.
	if len(deferred) != 0 {
		t.Errorf("mode A skip should NOT defer, got %v", deferred)
	}
}

// --- requiredHostname ----------------------------------------------------

func TestValidateRequiredHostname_Empty(t *testing.T) {
	sib := &config.SiblingInfo{Hostnames: []string{}}
	if err := validateRequiredHostname("kc", "", sib); err != nil {
		t.Errorf("empty requirement should be a no-op, got %v", err)
	}
}

func TestValidateRequiredHostname_Match(t *testing.T) {
	sib := &config.SiblingInfo{Hostnames: []string{"sso", "admin"}}
	if err := validateRequiredHostname("kc", "sso", sib); err != nil {
		t.Errorf("expected nil on match, got %v", err)
	}
}

func TestValidateRequiredHostname_Missing_HelpfulError(t *testing.T) {
	sib := &config.SiblingInfo{
		Path:      "/sibling/raioz.yaml",
		Project:   "keycloak",
		Hostnames: []string{"keycloak", "admin"},
	}
	err := validateRequiredHostname("kc", "sso", sib)
	if err == nil {
		t.Fatal("expected error for missing hostname")
	}
	msg := err.Error()
	for _, want := range []string{"sso", "keycloak", "/sibling/raioz.yaml", "hostname:", "requiredHostname"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
}

func TestDecideSibling_ModeA_RequiredHostnameMissing_FailsBeforeSpawn(t *testing.T) {
	siblingDir := writeSiblingYAML(t,
		"workspace: hypixo\nproject: keycloak\n"+
			"services:\n  keycloak:\n    path: .\n")
	// No fake runtime needed — the hostname check fires before active probe.

	inline := &models.Infra{
		Project:          siblingDir,
		RequiredHostname: "sso", // sibling only declares "keycloak"
	}
	_, err := decideSibling(context.Background(), "kc", inline, "hypixo")
	if err == nil || !strings.Contains(err.Error(), "does not declare hostname") {
		t.Errorf("expected hostname-missing error, got %v", err)
	}
}

func TestDecideSibling_ModeB_RequiredHostnameMissing_OnlyWhenSiblingActive(t *testing.T) {
	siblingDir := writeSiblingYAML(t,
		"workspace: hypixo\nproject: keycloak\n"+
			"services:\n  keycloak:\n    path: .\n")

	inline := &models.Infra{
		Image:            "keycloak",
		SiblingProject:   siblingDir,
		RequiredHostname: "sso",
	}

	// Sibling NOT active → fall through to image runner; hostname check
	// must not fire (we never defer to the sibling).
	writeFakeRuntime(t, "")
	got, err := decideSibling(context.Background(), "kc", inline, "hypixo")
	if err != nil {
		t.Fatalf("expected no error when falling back to image, got %v", err)
	}
	if got.Kind != siblingProceed {
		t.Errorf("expected proceed, got %d", got.Kind)
	}

	// Sibling active → hostname check fires and rejects.
	writeFakeRuntime(t, "hypixo-keycloak\n")
	_, err = decideSibling(context.Background(), "kc", inline, "hypixo")
	if err == nil || !strings.Contains(err.Error(), "does not declare hostname") {
		t.Errorf("expected hostname-missing error, got %v", err)
	}
}

func TestDecideSibling_ModeB_WorkspaceMismatch(t *testing.T) {
	siblingDir := writeSiblingYAML(t,
		"workspace: acme\nproject: keycloak\n"+
			"services:\n  keycloak:\n    path: .\n")
	// Fake runtime not needed — workspace check fails before probing.

	inline := &models.Infra{
		Image:          "keycloak",
		SiblingProject: siblingDir,
	}
	_, err := decideSibling(context.Background(), "keycloak", inline, "hypixo")
	if err == nil || !strings.Contains(err.Error(), "workspace") {
		t.Errorf("expected workspace mismatch error, got %v", err)
	}
}

// --- verifySiblingsStillUp -----------------------------------------------

func TestVerifySiblingsStillUp_EmptyAndProceedVerdicts(t *testing.T) {
	// No docker call should happen for proceed/spawn verdicts. We don't
	// install a fake runtime — if the code probes docker accidentally,
	// the real binary lookup will surface an error from t.Fatal below.
	verdicts := map[string]siblingDecision{
		"empty": {},                        // SiblingInfo == nil → skip
		"reg":   {Kind: siblingProceed},    // not a sibling defer → skip
		"spawn": {Kind: siblingSpawnModeA}, // spawn handled elsewhere → skip
	}
	if err := verifySiblingsStillUp(context.Background(), verdicts); err != nil {
		t.Errorf("expected nil for non-deferring verdicts, got %v", err)
	}
}

func TestVerifySiblingsStillUp_SiblingStillActive(t *testing.T) {
	// Non-empty stdout from docker ps means "containers found", i.e. sibling
	// is still up. Both deferred kinds should pass.
	writeFakeRuntime(t, "acme-keycloak\n")
	verdicts := map[string]siblingDecision{
		"keycloak": {
			Kind:        siblingSkipDeferred,
			SiblingInfo: &config.SiblingInfo{Project: "keycloak", Workspace: "acme", Dir: "/x"},
		},
		"sso": {
			Kind:        siblingSkipModeA,
			SiblingInfo: &config.SiblingInfo{Project: "sso", Workspace: "acme", Dir: "/y"},
		},
	}
	if err := verifySiblingsStillUp(context.Background(), verdicts); err != nil {
		t.Errorf("expected nil when sibling still active, got %v", err)
	}
}

func TestVerifySiblingsStillUp_SiblingDownReturnsActionableError(t *testing.T) {
	// Empty stdout from docker ps means "no containers running" — sibling
	// died between decideSibling and now. Must surface as ESIBLING_DOWN
	// with a suggestion that points the user at the sibling's directory.
	writeFakeRuntime(t, "")
	verdicts := map[string]siblingDecision{
		"keycloak": {
			Kind: siblingSkipDeferred,
			SiblingInfo: &config.SiblingInfo{
				Project:   "keycloak",
				Workspace: "acme",
				Dir:       "/path/to/keycloak",
			},
		},
	}
	err := verifySiblingsStillUp(context.Background(), verdicts)
	if err == nil {
		t.Fatal("expected ESIBLING_DOWN error, got nil")
	}
	// i18n is not initialized in unit tests, so Error() returns the key.
	// Inspect the structured RaiozError to confirm code + context.
	var re *errpkg.RaiozError
	if !goerrors.As(err, &re) {
		t.Fatalf("expected *errors.RaiozError, got %T: %v", err, err)
	}
	if re.Code != errpkg.ErrCodeSiblingDown {
		t.Errorf("expected code %q, got %q", errpkg.ErrCodeSiblingDown, re.Code)
	}
	if re.Context["sibling"] != "keycloak" || re.Context["dep"] != "keycloak" {
		t.Errorf("context missing expected keys, got: %v", re.Context)
	}
	if re.Suggestion == "" {
		t.Error("error must carry an actionable suggestion")
	}
}
