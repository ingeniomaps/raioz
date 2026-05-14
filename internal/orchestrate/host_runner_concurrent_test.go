package orchestrate

import (
	"sync"
	"testing"
)

// Issue 059 / ADR-028 regression guard for HostRunner. Run under
// `go test -race` to surface unguarded map access. Remove the mu
// in HostRunner and one of these must trip the detector.

// TestHostRunner_PIDsConcurrent fires SetPID / GetPID / Status-style
// peeks from many goroutines and asserts the final map matches the
// last-write-wins semantics expected from sync.Mutex.
func TestHostRunner_PIDsConcurrent(t *testing.T) {
	r := &HostRunner{}

	const goroutines = 16
	const perGoroutine = 64

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				name := svcName(g, i)
				r.SetPID(name, 1000+i)
				_ = r.GetPID(name) // exercise peek path
			}
		}(g)
	}
	wg.Wait()

	expected := goroutines * perGoroutine
	r.mu.Lock()
	got := len(r.processes)
	r.mu.Unlock()
	if got != expected {
		t.Errorf("processes count = %d, want %d", got, expected)
	}
}

// TestHostRunner_LauncherFlagConcurrent exercises markLauncher /
// isLauncher in parallel. The result is read with a final
// assertion that every key marked is observable.
func TestHostRunner_LauncherFlagConcurrent(t *testing.T) {
	r := &HostRunner{}

	const goroutines = 8
	const perGoroutine = 32

	names := make([]string, 0, goroutines*perGoroutine)
	for g := 0; g < goroutines; g++ {
		for i := 0; i < perGoroutine; i++ {
			names = append(names, svcName(g, i))
		}
	}

	var wg sync.WaitGroup
	for _, n := range names {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			r.markLauncher(n)
		}(n)
	}
	wg.Wait()

	for _, n := range names {
		if !r.isLauncher(n) {
			t.Errorf("isLauncher(%q) = false after marking", n)
		}
	}
}

// TestHostRunner_TakePIDConsumes asserts the read-and-clear pattern:
// takePID returns the PID once and the second call sees nothing.
func TestHostRunner_TakePIDConsumes(t *testing.T) {
	r := &HostRunner{}
	r.recordPID("api", 4242)

	pid1, ok1 := r.takePID("api")
	if !ok1 || pid1 != 4242 {
		t.Fatalf("first takePID = (%d, %v), want (4242, true)", pid1, ok1)
	}
	pid2, ok2 := r.takePID("api")
	if ok2 || pid2 != 0 {
		t.Errorf("second takePID = (%d, %v), want (0, false)", pid2, ok2)
	}
}

func svcName(g, i int) string {
	return "svc-" + itoa(g) + "-" + itoa(i)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var s string
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
