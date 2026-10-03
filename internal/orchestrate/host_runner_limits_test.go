package orchestrate

import (
	"context"
	"strings"
	"testing"

	"raioz/internal/domain/interfaces"
	"raioz/internal/domain/models"
)

func stubHostLimits(t *testing.T, reason string) {
	t.Helper()
	orig := hostLimitsUnavailableFn
	hostLimitsUnavailableFn = func(*models.Resources) string { return reason }
	t.Cleanup(func() { hostLimitsUnavailableFn = orig })
}

func TestScopeProperties(t *testing.T) {
	tests := []struct {
		name string
		res  *models.Resources
		want string
	}{
		{"nothing declared", nil, ""},
		{"memory pins swap to zero", &models.Resources{Memory: "256m"}, "MemoryMax=268435456 MemorySwapMax=0"},
		{"fractional cpu", &models.Resources{CPUs: 0.5}, "CPUQuota=50%"},
		{"both", &models.Resources{Memory: "1g", CPUs: 2}, "MemoryMax=1073741824 MemorySwapMax=0 CPUQuota=200%"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := strings.Join(scopeProperties(tt.res), " "); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHostLimitPrefix(t *testing.T) {
	capped := interfaces.ServiceContext{Name: "web", Resources: &models.Resources{Memory: "64m", CPUs: 1}}

	t.Run("no cap, no wrapper", func(t *testing.T) {
		stubHostLimits(t, "")
		if got := hostLimitPrefix(context.Background(), interfaces.ServiceContext{Name: "web"}); got != nil {
			t.Errorf("got %v", got)
		}
	})
	t.Run("a cap runs the command in a user scope", func(t *testing.T) {
		stubHostLimits(t, "")
		got := strings.Join(hostLimitPrefix(context.Background(), capped), " ")
		want := "systemd-run --user --scope --quiet --collect " +
			"-p MemoryMax=67108864 -p MemorySwapMax=0 -p CPUQuota=100% --"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
	t.Run("a host that cannot enforce it starts the service uncapped", func(t *testing.T) {
		stubHostLimits(t, "no systemd")
		if got := hostLimitPrefix(context.Background(), capped); got != nil {
			t.Errorf("got %v", got)
		}
	})
}

func TestMissingControllers(t *testing.T) {
	both := &models.Resources{Memory: "64m", CPUs: 1}
	tests := []struct {
		name, delegated, want string
		res                   *models.Resources
	}{
		{"all delegated", "cpu memory pids\n", "", both},
		{"memory missing", "cpu pids", "memory", both},
		{"cpu missing", "memory pids", "cpu", both},
		{"cpu not needed", "memory pids", "", &models.Resources{Memory: "64m"}},
		{"nothing delegated", "", "memory", both},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := missingControllers(tt.res, tt.delegated); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
