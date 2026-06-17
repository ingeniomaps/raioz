package docker

import "testing"

func TestParsePublishedHostPort(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want int
	}{
		{"ipv4 only", "0.0.0.0:6379\n", 6379},
		{"ipv4 and ipv6", "0.0.0.0:6379\n[::]:6379\n", 6379},
		{"ipv6 first", "[::]:5432\n0.0.0.0:5432\n", 5432},
		{"remapped port", "0.0.0.0:6380\n", 6380},
		{"empty", "", 0},
		{"whitespace only", "   \n\t\n", 0},
		{"no port suffix", "0.0.0.0:\n", 0},
		{"garbage", "not-a-binding\n", 0},
		{"skips garbage then parses", "garbage\n0.0.0.0:7000\n", 7000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parsePublishedHostPort(tt.out); got != tt.want {
				t.Errorf("parsePublishedHostPort(%q) = %d, want %d", tt.out, got, tt.want)
			}
		})
	}
}

func TestGetPublishedHostPortGuards(t *testing.T) {
	// No docker call should happen for these — they short-circuit on args.
	if got, err := GetPublishedHostPort(t.Context(), "", 6379); got != 0 || err != nil {
		t.Errorf("empty name: got (%d, %v), want (0, nil)", got, err)
	}
	if got, err := GetPublishedHostPort(t.Context(), "some-container", 0); got != 0 || err != nil {
		t.Errorf("zero port: got (%d, %v), want (0, nil)", got, err)
	}
}
