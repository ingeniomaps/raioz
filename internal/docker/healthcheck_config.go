package docker

import (
	"raioz/internal/config"
)

// HealthcheckToMap converts config.HealthcheckConfig to the map format used in docker-compose (same as Docker).
// Omits zero/empty fields. When Disable is true, the returned map only contains "disable": true so Compose disables the healthcheck.
func HealthcheckToMap(h *config.HealthcheckConfig) map[string]any {
	if h == nil {
		return nil
	}
	out := make(map[string]any)
	if h.Disable {
		out["disable"] = true
		return out
	}
	if h.Test != nil {
		out["test"] = h.Test
	}
	if h.Interval != "" {
		out["interval"] = h.Interval
	}
	if h.Timeout != "" {
		out["timeout"] = h.Timeout
	}
	if h.Retries != 0 {
		out["retries"] = h.Retries
	}
	if h.StartPeriod != "" {
		out["start_period"] = h.StartPeriod
	}
	if h.StartInterval != "" {
		out["start_interval"] = h.StartInterval
	}
	return out
}
