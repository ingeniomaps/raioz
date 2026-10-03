package naming

import (
	"context"
	"errors"
	"testing"
)

// stubLookup is a minimal in-memory ContainerLookup used to drive the
// strategy branches of ResolveContainer / ContainerTarget without going
// near a real Docker daemon.
type stubLookup struct {
	// existing is the set of container names docker would report as
	// existing in `docker inspect`.
	existing map[string]bool
	// labeled maps a serialized label set ("k1=v1,k2=v2") to the names
	// of containers that match. Match order in tests is the slice order.
	labeled map[string][]string
	// existsErr forces Exists to fail (timeout simulation).
	existsErr error
}

func (s *stubLookup) Exists(_ context.Context, name string) (bool, error) {
	if s.existsErr != nil {
		return false, s.existsErr
	}
	return s.existing[name], nil
}

func (s *stubLookup) FindByLabels(
	_ context.Context, labels map[string]string,
) []string {
	return s.labeled[encodeLabels(labels)]
}

func encodeLabels(labels map[string]string) string {
	// Tests construct the expected key in the same order ResolveContainer
	// builds the filter map. We rely on stable ordering of the small
	// label set used in this package (managed → service → project →
	// workspace → kind).
	out := ""
	for _, k := range []string{LabelManaged, LabelService, LabelProject, LabelWorkspace, LabelKind} {
		if v, ok := labels[k]; ok {
			if out != "" {
				out += ","
			}
			out += k + "=" + v
		}
	}
	return out
}

func TestResolveContainer_CanonicalHit(t *testing.T) {
	SetPrefix("")
	t.Cleanup(func() { SetPrefix("") })
	lookup := &stubLookup{
		existing: map[string]bool{"raioz-acme-postgres": true},
	}
	got, err := ResolveContainer(context.Background(), lookup, "acme", "postgres", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "raioz-acme-postgres" {
		t.Errorf("expected canonical, got %q", got)
	}
}

func TestResolveContainer_FallbackToLabels(t *testing.T) {
	SetPrefix("")
	t.Cleanup(func() { SetPrefix("") })
	// Canonical not in docker; a user-supplied compose set its own name.
	lookup := &stubLookup{
		existing: map[string]bool{},
		labeled: map[string][]string{
			"com.raioz.managed=true,com.raioz.service=postgres,com.raioz.project=acme": {
				"pg-master",
			},
		},
	}
	got, err := ResolveContainer(context.Background(), lookup, "acme", "postgres", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "pg-master" {
		t.Errorf("expected label-match 'pg-master', got %q", got)
	}
}

func TestResolveContainer_NothingFound(t *testing.T) {
	SetPrefix("")
	t.Cleanup(func() { SetPrefix("") })
	lookup := &stubLookup{}
	got, err := ResolveContainer(context.Background(), lookup, "acme", "redis", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty when no match, got %q", got)
	}
}

func TestResolveContainer_NameOverrideHitsCanonicalDirectly(t *testing.T) {
	SetPrefix("")
	t.Cleanup(func() { SetPrefix("") })
	lookup := &stubLookup{
		existing: map[string]bool{"my-pg": true},
	}
	got, err := ResolveContainer(context.Background(), lookup, "acme", "postgres", "my-pg")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "my-pg" {
		t.Errorf("expected override 'my-pg', got %q", got)
	}
}

func TestResolveContainer_NilLookupReturnsCanonical(t *testing.T) {
	SetPrefix("")
	t.Cleanup(func() { SetPrefix("") })
	got, err := ResolveContainer(context.Background(), nil, "acme", "postgres", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "raioz-acme-postgres" {
		t.Errorf("expected canonical when lookup nil, got %q", got)
	}
}

func TestResolveContainer_ExistsErrorPropagates(t *testing.T) {
	SetPrefix("")
	t.Cleanup(func() { SetPrefix("") })
	lookup := &stubLookup{existsErr: errors.New("docker timeout")}
	_, err := ResolveContainer(context.Background(), lookup, "acme", "postgres", "")
	if err == nil {
		t.Fatal("expected error from underlying lookup, got nil")
	}
}

func TestContainerTarget_FallsBackToCanonical(t *testing.T) {
	SetPrefix("")
	t.Cleanup(func() { SetPrefix("") })
	// No match anywhere — ContainerTarget must still return a usable
	// name (the canonical) so the proxy/discovery callers can wire it.
	lookup := &stubLookup{}
	got := ContainerTarget(context.Background(), lookup, "acme", "postgres", "")
	if got != "raioz-acme-postgres" {
		t.Errorf("expected canonical fallback, got %q", got)
	}
}

func TestContainerTarget_FallsBackOnLookupError(t *testing.T) {
	SetPrefix("")
	t.Cleanup(func() { SetPrefix("") })
	// A Docker probe failure must NOT prevent us from returning a usable
	// name — the caller is building a target string, not asserting
	// container health.
	lookup := &stubLookup{existsErr: errors.New("docker outage")}
	got := ContainerTarget(context.Background(), lookup, "acme", "postgres", "")
	if got != "raioz-acme-postgres" {
		t.Errorf("expected canonical on lookup error, got %q", got)
	}
}

func TestResolveContainer_WorkspaceMode(t *testing.T) {
	SetPrefix("acme")
	t.Cleanup(func() { SetPrefix("") })
	// Workspace mode: canonical is the shared name {workspace}-{dep}.
	lookup := &stubLookup{
		existing: map[string]bool{"acme-postgres": true},
	}
	got, err := ResolveContainer(context.Background(), lookup,
		"projectA", "postgres", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "acme-postgres" {
		t.Errorf("expected shared canonical 'acme-postgres', got %q", got)
	}
}

func TestResolveDepContainer(t *testing.T) {
	const wsFilter = "com.raioz.managed=true,com.raioz.service=rabbitmq," +
		"com.raioz.workspace=dropi,com.raioz.kind=dependency"
	const projFilter = "com.raioz.managed=true,com.raioz.service=rabbitmq,com.raioz.project=dropi"

	tests := []struct {
		name      string
		workspace string
		lookup    *stubLookup
		want      string
		wantErr   bool
	}{
		{
			// The reported bug: workspace dep from a compose with its own
			// container_name. The overlay stamped no project label, so only
			// the workspace-scoped retry can find it.
			name:      "workspace dep with own container_name",
			workspace: "dropi",
			lookup:    &stubLookup{labeled: map[string][]string{wsFilter: {"rabbitmq"}}},
			want:      "rabbitmq",
		},
		{
			name:      "canonical still wins in workspace mode",
			workspace: "dropi",
			lookup: &stubLookup{
				existing: map[string]bool{"dropi-rabbitmq": true},
				labeled:  map[string][]string{wsFilter: {"rabbitmq"}},
			},
			want: "dropi-rabbitmq",
		},
		{
			name:      "project label match wins over workspace retry",
			workspace: "dropi",
			lookup: &stubLookup{labeled: map[string][]string{
				projFilter: {"by-project"},
				wsFilter:   {"by-workspace"},
			}},
			want: "by-project",
		},
		{
			// Without a workspace the overlay keeps the project label, so
			// there is nothing to retry — and nothing to widen into.
			name:   "no workspace skips the retry",
			lookup: &stubLookup{labeled: map[string][]string{wsFilter: {"rabbitmq"}}},
			want:   "",
		},
		{
			name:      "nothing found in workspace mode",
			workspace: "dropi",
			lookup:    &stubLookup{},
			want:      "",
		},
		{
			name:      "lookup error propagates without retry",
			workspace: "dropi",
			lookup: &stubLookup{
				existsErr: errors.New("docker timeout"),
				labeled:   map[string][]string{wsFilter: {"rabbitmq"}},
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			SetPrefix(tc.workspace)
			t.Cleanup(func() { SetPrefix("") })
			got, err := ResolveDepContainer(context.Background(), tc.lookup, "dropi", "rabbitmq", "")
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveDepContainer_NilLookupReturnsCanonical(t *testing.T) {
	SetPrefix("dropi")
	t.Cleanup(func() { SetPrefix("") })
	got, err := ResolveDepContainer(context.Background(), nil, "dropi", "rabbitmq", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "dropi-rabbitmq" {
		t.Errorf("expected canonical when lookup nil, got %q", got)
	}
}

// ContainerTarget addresses a dependency (Caddy upstream, discovery host).
// A workspace dep whose compose sets its own container_name carries no
// project label, so only the workspace-scoped lookup finds the name that
// actually resolves on the network.
func TestContainerTarget_WorkspaceComposeDep(t *testing.T) {
	SetPrefix("dropi")
	t.Cleanup(func() { SetPrefix("") })

	const wsFilter = "com.raioz.managed=true,com.raioz.service=rabbitmq," +
		"com.raioz.workspace=dropi,com.raioz.kind=dependency"
	lookup := &stubLookup{labeled: map[string][]string{wsFilter: {"rabbitmq"}}}

	if got := ContainerTarget(context.Background(), lookup, "dropi", "rabbitmq", ""); got != "rabbitmq" {
		t.Errorf("ContainerTarget = %q, want the live container %q", got, "rabbitmq")
	}
}
