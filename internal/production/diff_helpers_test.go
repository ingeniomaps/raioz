package production

import "testing"

func TestPortsEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want bool
	}{
		{"both nil", nil, nil, true},
		{"empty equals empty", []string{}, []string{}, true},
		{"different lengths", []string{"3000"}, []string{"3000", "3001"}, false},
		{"same order", []string{"3000:3000"}, []string{"3000:3000"}, true},
		{"different order normalized", []string{"3000", "8080"}, []string{"8080", "3000"}, true},
		{"different values", []string{"3000"}, []string{"3001"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := portsEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVolumesEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want bool
	}{
		{"both empty", nil, nil, true},
		{"./prefix normalized", []string{"./data:/data"}, []string{"data:/data"}, true},
		{"different mounts", []string{"./a:/a"}, []string{"./b:/b"}, false},
		{"different lengths", []string{"a"}, []string{"a", "b"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := volumesEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeVolume(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"./data:/data", "data:/data"},
		{"data:/data", "data:/data"},
		{"  ./x  ", "x"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := normalizeVolume(tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDependsEqual(t *testing.T) {
	if !dependsEqual([]string{"a", "b"}, []string{"b", "a"}) {
		t.Error("order shouldn't matter")
	}
	if dependsEqual([]string{"a"}, []string{"a", "b"}) {
		t.Error("different lengths must not be equal")
	}
	if dependsEqual([]string{"a"}, []string{"b"}) {
		t.Error("different values must not be equal")
	}
}

func TestIsInfraService(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"postgres", true},
		{"my-redis-instance", true},
		{"app-database-shard", true},
		{"web", false},
		{"api", false},
		{"frontend", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInfraService(tt.name); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
