package docker

import (
	"strings"
	"testing"
)

func TestValidateComposePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid absolute", "/tmp/docker-compose.yml", false},
		{"valid relative", "docker-compose.yml", false},
		{"with subdir", "dir/docker-compose.yml", false},
		{"empty", "", true},
		{"semicolon injection", "/tmp/docker-compose.yml;rm -rf /", true},
		{"pipe injection", "/tmp/file|cat", true},
		{"backtick injection", "/tmp/`whoami`", true},
		{"dollar injection", "/tmp/$PWD", true},
		{"ampersand injection", "/tmp/file&echo", true},
		{"newline", "/tmp/file\nls", true},
		{"carriage return", "/tmp/file\rls", true},
		{"tab", "/tmp/file\tls", true},
		{"null byte", "/tmp/file\x00ls", true},
		{"too long", "/tmp/" + strings.Repeat("a", 5000), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateComposePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateComposePath(%q) err = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}
