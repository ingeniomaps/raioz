package app

import (
	"reflect"
	"testing"

	"raioz/internal/docker"
)

// sliceEqualLoose treats nil and empty slices as equivalent so table tests
// don't need to fork "want: nil" vs "want: []string{}" — filterKeep returns
// an empty (but non-nil) slice and SplitKeepList returns nil for empty
// input. Both are "no elements" results.
func sliceEqualLoose(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

func TestFilterKeep(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		keep  []string
		want  []string
	}{
		{
			name:  "empty keep returns input unchanged",
			names: []string{"a", "b", "c"},
			keep:  nil,
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "drops listed names",
			names: []string{"a", "b", "c"},
			keep:  []string{"b"},
			want:  []string{"a", "c"},
		},
		{
			name:  "trims whitespace on keep entries",
			names: []string{"foo", "bar"},
			keep:  []string{" foo ", "  "},
			want:  []string{"bar"},
		},
		{
			name:  "keep entries not in names is harmless",
			names: []string{"a"},
			keep:  []string{"b", "c"},
			want:  []string{"a"},
		},
		{
			name:  "all kept",
			names: []string{"a", "b"},
			keep:  []string{"a", "b"},
			want:  []string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := filterKeep(tc.names, tc.keep)
			if !sliceEqualLoose(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPortsForProject(t *testing.T) {
	conflicts := []docker.PortConflict{
		{Port: "9001", Project: "alpha"},
		{Port: "8025", Project: "alpha"},
		{Port: "9001", Project: "alpha"}, // duplicate within alpha
		{Port: "5432", Project: "beta"},
		{Port: "", Project: "alpha"}, // empty port skipped
		{Port: "5540", Project: ""},  // empty project ignored
	}

	got := portsForProject(conflicts, "alpha")
	want := []string{"8025", "9001"} // sorted, deduped, non-empty
	if !reflect.DeepEqual(got, want) {
		t.Errorf("portsForProject(alpha) = %v, want %v", got, want)
	}

	if got := portsForProject(conflicts, "gamma"); len(got) != 0 {
		t.Errorf("portsForProject(missing project) = %v, want empty", got)
	}
}

func TestSplitKeepList(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "empty string", raw: "", want: nil},
		{name: "single", raw: "alpha", want: []string{"alpha"}},
		{name: "csv", raw: "alpha,beta,gamma",
			want: []string{"alpha", "beta", "gamma"}},
		{name: "trims whitespace", raw: " alpha , beta ",
			want: []string{"alpha", "beta"}},
		{name: "drops empties", raw: "alpha,,beta,",
			want: []string{"alpha", "beta"}},
		{name: "only separators", raw: ", , ,", want: []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SplitKeepList(tc.raw)
			if !sliceEqualLoose(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
