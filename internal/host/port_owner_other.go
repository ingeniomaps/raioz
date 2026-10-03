//go:build !linux

package host

func processGroupListensOn(_, _ int) (held, known bool) { return false, false }

func groupListeningPorts(_ int) (ports []int, known bool) { return nil, false }

func processRunsIn(_ int, _ string) (within, known bool) { return false, false }
