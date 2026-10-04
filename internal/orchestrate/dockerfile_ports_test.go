package orchestrate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"raioz/internal/domain/interfaces"
)

func TestDeclaredPortArgs(t *testing.T) {
	stub := func(port int, err error) {
		prev := imageExposedPort
		imageExposedPort = func(context.Context, string) (int, error) { return port, err }
		t.Cleanup(func() { imageExposedPort = prev })
	}
	tests := []struct {
		name    string
		svc     interfaces.ServiceContext
		exposed int
		err     error
		want    string
	}{
		{"no port declared", interfaces.ServiceContext{}, 3000, nil, ""},
		{"maps to the port the image exposes", interfaces.ServiceContext{HostPort: 8080}, 3000, nil,
			"-p 127.0.0.1:8080:3000"},
		{"image exposes nothing: same number", interfaces.ServiceContext{HostPort: 8080}, 0, nil,
			"-p 127.0.0.1:8080:8080"},
		{"image cannot be read: same number", interfaces.ServiceContext{HostPort: 8080}, 0, errors.New("x"),
			"-p 127.0.0.1:8080:8080"},
		{"explicit ports: are the user's own", interfaces.ServiceContext{HostPort: 8080, Ports: []string{"9000:3000"}},
			3000, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub(tt.exposed, tt.err)
			if got := strings.Join(declaredPortArgs(context.Background(), tt.svc, "img"), " "); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
