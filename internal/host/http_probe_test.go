package host

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthURL(t *testing.T) {
	cases := []struct {
		name     string
		port     int
		endpoint string
		want     string
	}{
		{"leading slash kept", 3000, "/api/health", "http://127.0.0.1:3000/api/health"},
		{"missing slash added", 8080, "health/ready", "http://127.0.0.1:8080/health/ready"},
		{"root endpoint", 9000, "/", "http://127.0.0.1:9000/"},
		{"no endpoint", 9000, "", "http://127.0.0.1:9000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := HealthURL(tc.port, tc.endpoint); got != tc.want {
				t.Errorf("HealthURL(%d, %q) = %q, want %q", tc.port, tc.endpoint, got, tc.want)
			}
		})
	}
}

func TestProbeHTTP(t *testing.T) {
	cases := []struct {
		name string
		code int
		want bool
	}{
		{"200 is up", http.StatusOK, true},
		{"204 is up", http.StatusNoContent, true},
		{"302 is up", http.StatusFound, true},
		{"404 is not", http.StatusNotFound, false},
		{"500 is not", http.StatusInternalServerError, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.code)
			}))
			defer srv.Close()
			if got := ProbeHTTP(t.Context(), srv.URL); got != tc.want {
				t.Errorf("ProbeHTTP(%d) = %v, want %v", tc.code, got, tc.want)
			}
		})
	}
}

func TestProbeHTTP_NobodyListening(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // the port is now closed

	if ProbeHTTP(t.Context(), url) {
		t.Error("ProbeHTTP answered true with nobody listening")
	}
}
