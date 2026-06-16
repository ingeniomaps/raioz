package netutil

import "testing"

func TestSchemeForImage(t *testing.T) {
	tests := []struct {
		image string
		want  string
	}{
		{"redis:8-alpine", "redis"},
		{"redis", "redis"},
		{"bitnami/valkey:7", "redis"},
		{"postgres:16", "postgresql"},
		{"bitnami/postgresql:15", "postgresql"},
		{"mysql:8", "mysql"},
		{"mariadb:11", "mysql"},
		{"mongo:7", "mongodb"},
		{"rabbitmq:3-management", "amqp"},
		{"nats:2", "nats"},
		// HTTP UIs that merely share a substring must stay http.
		{"redis/redisinsight:latest", "http"},
		{"dpage/pgadmin4:latest", "http"},
		{"nginx:1.27", "http"},
		{"", "http"},
	}
	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			if got := SchemeForImage(tt.image); got != tt.want {
				t.Errorf("SchemeForImage(%q) = %q, want %q", tt.image, got, tt.want)
			}
		})
	}
}
