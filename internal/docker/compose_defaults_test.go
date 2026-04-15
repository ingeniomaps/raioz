package docker

import (
	"testing"
)

func TestAddDefaultInfraEnv(t *testing.T) {
	tests := []struct {
		name     string
		svcName  string
		image    string
		wantKeys []string
	}{
		{
			name:     "postgres image",
			svcName:  "db",
			image:    "postgres",
			wantKeys: []string{"POSTGRES_PASSWORD", "POSTGRES_USER", "POSTGRES_DB"},
		},
		{
			name:     "database name",
			svcName:  "database",
			image:    "some/image",
			wantKeys: []string{"POSTGRES_PASSWORD", "POSTGRES_USER", "POSTGRES_DB"},
		},
		{
			name:     "redis",
			svcName:  "cache",
			image:    "redis",
			wantKeys: []string{},
		},
		{
			name:     "unknown",
			svcName:  "random",
			image:    "something",
			wantKeys: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addDefaultInfraEnv(tt.svcName, tt.image)
			if len(got) != len(tt.wantKeys) {
				t.Errorf("addDefaultInfraEnv(%s, %s) len = %d, want %d",
					tt.svcName, tt.image, len(got), len(tt.wantKeys))
			}
			for _, k := range tt.wantKeys {
				if _, ok := got[k]; !ok {
					t.Errorf("addDefaultInfraEnv(%s, %s) missing key %q", tt.svcName, tt.image, k)
				}
			}
		})
	}
}

func TestAddDefaultInfraHealthcheck(t *testing.T) {
	tests := []struct {
		name    string
		svcName string
		image   string
		wantNil bool
	}{
		{"postgres image", "db", "postgres", false},
		{"postgres name", "postgres", "something", false},
		{"postgresql name", "postgresql", "custom-image", false},
		{"pgadmin image", "ui", "dpage/pgadmin4", false},
		{"pgadmin name", "pgadmin", "dpage/pgadmin4", false},
		{"redis image", "cache", "redis", false},
		{"redis name", "redis", "image", false},
		{"mongo image", "m", "mongo", false},
		{"mongo name", "mongo", "some-mongo", false},
		{"mongodb name", "mongodb", "foo", false},
		{"mysql image", "db", "mysql", false},
		{"mariadb image", "db", "mariadb", false},
		{"mysql name", "mysql", "custom", false},
		{"mariadb name", "mariadb", "custom", false},
		{"unknown", "whatever", "unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := addDefaultInfraHealthcheck(tt.svcName, tt.image)
			if tt.wantNil {
				if got != nil {
					t.Errorf("addDefaultInfraHealthcheck(%s, %s) = %v, want nil",
						tt.svcName, tt.image, got)
				}
				return
			}
			if got == nil {
				t.Errorf("addDefaultInfraHealthcheck(%s, %s) = nil, want config",
					tt.svcName, tt.image)
				return
			}
			// Check required keys
			if _, ok := got["test"]; !ok {
				t.Errorf("addDefaultInfraHealthcheck(%s, %s) missing 'test'",
					tt.svcName, tt.image)
			}
			if _, ok := got["interval"]; !ok {
				t.Errorf("addDefaultInfraHealthcheck(%s, %s) missing 'interval'",
					tt.svcName, tt.image)
			}
		})
	}
}

func TestGetInitDir(t *testing.T) {
	tests := []struct {
		image string
		want  string
	}{
		{"postgres", "/docker-entrypoint-initdb.d"},
		{"postgres:15", "/docker-entrypoint-initdb.d"},
		{"mysql", "/docker-entrypoint-initdb.d"},
		{"mariadb:10", "/docker-entrypoint-initdb.d"},
		{"mongo:6", "/docker-entrypoint-initdb.d"},
		{"unknown", "/docker-entrypoint-initdb.d"},
		{"POSTGRES:15", "/docker-entrypoint-initdb.d"},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			got := getInitDir(tt.image)
			if got != tt.want {
				t.Errorf("getInitDir(%q) = %q, want %q", tt.image, got, tt.want)
			}
		})
	}
}
