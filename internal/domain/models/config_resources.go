package models

import (
	"fmt"
	"regexp"
	"strconv"
)

// Resources caps what a container raioz creates may use. Both fields are
// optional; the zero value means "no cap", which is Docker's default and
// what raioz did before the block existed.
type Resources struct {
	// Memory is a Docker size: a number with an optional b/k/m/g unit
	// (`256m`, `1g`). It is applied as the memory limit AND the
	// memory+swap limit, so the container cannot spill into swap — a cap
	// that swap can bypass is not a cap.
	Memory string `yaml:"memory,omitempty" json:"memory,omitempty"` // since: v0.16.0
	// CPUs is how many CPUs the container may use (`0.5`, `2`).
	CPUs float64 `yaml:"cpus,omitempty" json:"cpus,omitempty"` // since: v0.16.0
}

var memorySizePattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?([bkmgBKMG][bB]?)?$`)

// IsZero reports whether r declares no cap at all.
func (r *Resources) IsZero() bool {
	return r == nil || (r.Memory == "" && r.CPUs == 0)
}

// Validate rejects a value Docker would refuse, before anything starts.
func (r *Resources) Validate() error {
	if r == nil {
		return nil
	}
	if r.Memory != "" && !memorySizePattern.MatchString(r.Memory) {
		return fmt.Errorf("memory %q is not a size like 256m or 1g", r.Memory)
	}
	if r.CPUs < 0 {
		return fmt.Errorf("cpus %s must be positive", r.CPUsString())
	}
	return nil
}

// CPUsString renders CPUs the way Docker's --cpus and compose's `cpus`
// take it.
func (r *Resources) CPUsString() string {
	return strconv.FormatFloat(r.CPUs, 'f', -1, 64)
}

// OrDefault returns r when it declares something, fallback otherwise.
func (r *Resources) OrDefault(fallback *Resources) *Resources {
	if r.IsZero() {
		return fallback
	}
	return r
}
