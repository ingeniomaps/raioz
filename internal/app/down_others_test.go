package app

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"

	"raioz/internal/docker"
	"raioz/internal/domain/models"
)

func TestUniqueConflictingProjects(t *testing.T) {
	cases := []struct {
		name      string
		conflicts []docker.PortConflict
		current   string
		want      []string
	}{
		{
			name: "deduplicates and sorts",
			conflicts: []docker.PortConflict{
				{Port: "9001:8080", Project: "hypixo-keycloak", Service: "keycloak"},
				{Port: "5540:5540", Project: "hypixo-keycloak", Service: "redisinsight"},
				{Port: "8025:8025", Project: "alpha-mail", Service: "mailpit"},
			},
			current: "gouduet-keycloak",
			want:    []string{"alpha-mail", "hypixo-keycloak"},
		},
		{
			name: "skips own project",
			conflicts: []docker.PortConflict{
				{Port: "9001:8080", Project: "self", Service: "x"},
				{Port: "5432:5432", Project: "other", Service: "y"},
			},
			current: "self",
			want:    []string{"other"},
		},
		{
			name: "skips conflicts without project label",
			conflicts: []docker.PortConflict{
				{Port: "80:80", Project: "", Service: "?"},
				{Port: "443:443", Project: "real", Service: "z"},
			},
			current: "self",
			want:    []string{"real"},
		},
		{
			name:      "empty input → empty output",
			conflicts: nil,
			current:   "self",
			want:      []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := uniqueConflictingProjects(tc.conflicts, tc.current)
			if got == nil {
				got = []string{}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("uniqueConflictingProjects() = %v, want %v", got, tc.want)
			}
		})
	}
}

// withDownOthersHooks replaces the three docker-touching package vars with
// stubs and restores them on cleanup. Returns a struct exposing what each
// stub was called with so assertions can verify the cwd is never targeted.
type downOthersStubs struct {
	conflicts        []docker.PortConflict
	validateErr      error
	active           []string
	listErr          error
	stopped          map[string][]string // project → containers it pretends to have killed
	stopErrors       map[string]error
	stopProjectCalls []string
}

func withDownOthersHooks(t *testing.T, s *downOthersStubs) {
	t.Helper()
	prevVP, prevLA, prevSP := validatePortsFn, listActiveProjectsFn, stopProjectContainersFn
	validatePortsFn = func(_ *models.Deps, _ string, _ string) ([]docker.PortConflict, error) {
		return s.conflicts, s.validateErr
	}
	listActiveProjectsFn = func(_ context.Context) ([]string, error) {
		return s.active, s.listErr
	}
	stopProjectContainersFn = func(_ context.Context, project string) ([]string, error) {
		s.stopProjectCalls = append(s.stopProjectCalls, project)
		if err, ok := s.stopErrors[project]; ok {
			return nil, err
		}
		if c, ok := s.stopped[project]; ok {
			return c, nil
		}
		return nil, nil
	}
	t.Cleanup(func() {
		validatePortsFn = prevVP
		listActiveProjectsFn = prevLA
		stopProjectContainersFn = prevSP
	})
}

func TestDownConflictingProjects_NilDepsNoop(t *testing.T) {
	initI18nForTest(t)
	got, err := DownConflictingProjects(context.Background(), nil, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil result for nil deps, got %v", got)
	}
}

func TestDownConflictingProjects_StopsOnlyOthers(t *testing.T) {
	initI18nForTest(t)
	stubs := &downOthersStubs{
		conflicts: []docker.PortConflict{
			{Port: "5432:5432", Project: "siblingA", Service: "postgres"},
			{Port: "6379:6379", Project: "siblingB", Service: "redis"},
			// own-project conflict must be filtered out by
			// uniqueConflictingProjects — never reach stopProjects.
			{Port: "80:80", Project: "myproj", Service: "self"},
		},
		stopped: map[string][]string{
			"siblingA": {"siblingA-postgres"},
			"siblingB": {"siblingB-redis"},
		},
	}
	withDownOthersHooks(t, stubs)

	cwd := &models.Deps{Project: models.Project{Name: "myproj"}}
	got, err := DownConflictingProjects(context.Background(), cwd, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Strings(got)
	want := []string{"siblingA", "siblingB"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stopped projects = %v, want %v", got, want)
	}
	for _, p := range stubs.stopProjectCalls {
		if p == "myproj" {
			t.Error("must never target the cwd project")
		}
	}
}

func TestDownConflictingProjects_NoConflicts(t *testing.T) {
	initI18nForTest(t)
	withDownOthersHooks(t, &downOthersStubs{conflicts: nil})

	cwd := &models.Deps{Project: models.Project{Name: "myproj"}}
	got, err := DownConflictingProjects(context.Background(), cwd, "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

func TestDownConflictingProjects_ValidatePortsError(t *testing.T) {
	initI18nForTest(t)
	want := errors.New("docker offline")
	withDownOthersHooks(t, &downOthersStubs{validateErr: want})

	cwd := &models.Deps{Project: models.Project{Name: "myproj"}}
	_, err := DownConflictingProjects(context.Background(), cwd, "/tmp")
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

func TestDownAllOtherProjects_StopsAllButCwd(t *testing.T) {
	initI18nForTest(t)
	stubs := &downOthersStubs{
		active: []string{"alpha", "myproj", "beta"},
		stopped: map[string][]string{
			"alpha": {"alpha-api"},
			"beta":  {"beta-api", "beta-db"},
		},
	}
	withDownOthersHooks(t, stubs)

	got, err := DownAllOtherProjects(context.Background(), "myproj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sort.Strings(got)
	want := []string{"alpha", "beta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stopped projects = %v, want %v", got, want)
	}
	for _, p := range stubs.stopProjectCalls {
		if p == "myproj" {
			t.Error("must never target the cwd project")
		}
	}
}

func TestDownAllOtherProjects_NoOthers(t *testing.T) {
	initI18nForTest(t)
	withDownOthersHooks(t, &downOthersStubs{active: []string{"myproj"}})

	got, err := DownAllOtherProjects(context.Background(), "myproj")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestDownAllOtherProjects_ListError(t *testing.T) {
	initI18nForTest(t)
	want := errors.New("ls failed")
	withDownOthersHooks(t, &downOthersStubs{listErr: want})

	_, err := DownAllOtherProjects(context.Background(), "myproj")
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

func TestStopProjects_SkipsFailures(t *testing.T) {
	initI18nForTest(t)
	stubs := &downOthersStubs{
		stopped: map[string][]string{
			"healthy": {"healthy-x"},
			// "broken" intentionally absent → error in stopErrors
		},
		stopErrors: map[string]error{
			"broken": errors.New("docker stop failed"),
		},
	}
	withDownOthersHooks(t, stubs)

	got := stopProjects(context.Background(), []string{"broken", "healthy"})
	if !reflect.DeepEqual(got, []string{"healthy"}) {
		t.Errorf("stopped = %v, want [healthy] only (broken should be skipped silently)", got)
	}
}

func TestStopProjects_EmptyContainerListDoesNotReportStopped(t *testing.T) {
	initI18nForTest(t)
	// stopProjectContainersFn returns ([]string{}, nil) — no containers
	// were actually killed. stopProjects must NOT include this project
	// in its result.
	stubs := &downOthersStubs{
		stopped: map[string][]string{"ghost": {}},
	}
	withDownOthersHooks(t, stubs)

	got := stopProjects(context.Background(), []string{"ghost"})
	if len(got) != 0 {
		t.Errorf("expected empty result for project with no containers, got %v", got)
	}
}

func TestFilterOtherActiveProjects(t *testing.T) {
	cases := []struct {
		name    string
		active  []string
		current string
		want    []string
	}{
		{
			name:    "removes self and dedupes",
			active:  []string{"a", "self", "b", "a"},
			current: "self",
			want:    []string{"a", "b"},
		},
		{
			name:    "empty current keeps everything",
			active:  []string{"a", "b"},
			current: "",
			want:    []string{"a", "b"},
		},
		{
			name:    "skips empty entries",
			active:  []string{"", "x", ""},
			current: "self",
			want:    []string{"x"},
		},
		{
			name:    "all filtered → empty",
			active:  []string{"self", "self"},
			current: "self",
			want:    []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := filterOtherActiveProjects(tc.active, tc.current)
			if got == nil {
				got = []string{}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("filterOtherActiveProjects() = %v, want %v", got, tc.want)
			}
		})
	}
}
