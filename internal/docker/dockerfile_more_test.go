package docker

import (
	"strings"
	"testing"
)

func TestGetInstallCommand(t *testing.T) {
	tests := []struct {
		runtime string
		want    string
	}{
		{"node", "npm install"},
		{"nodejs", "npm install"},
		{"javascript", "npm install"},
		{"js", "npm install"},
		{"NODE", "npm install"},
		{"go", "go mod download"},
		{"golang", "go mod download"},
		{"python", "pip install"},
		{"py", "pip install"},
		{"java", "mvn"},
		{"rust", "cargo fetch"},
		{"unknown", "npm install"},
		{"", "npm install"},
	}

	for _, tt := range tests {
		t.Run(tt.runtime, func(t *testing.T) {
			got := getInstallCommand(tt.runtime)
			if !strings.Contains(got, tt.want) {
				t.Errorf("getInstallCommand(%q) = %q, should contain %q",
					tt.runtime, got, tt.want)
			}
		})
	}
}
