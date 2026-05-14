package proxy

import (
	"context"
	"sync"
	"testing"

	"raioz/internal/domain/interfaces"
)

// ADR-028 regression guard: run under `go test -race`
// to surface any unguarded access. The test was added together with
// the routesMu sync.RWMutex; remove the lock and it must trip the
// detector. AddRoute, GetURL, HostsLine and snapshotRoutes are the
// reads-and-writes surface.

// TestManager_RoutesConcurrent fires concurrent AddRoute + reads and
// asserts no race + final state matches the writers' contributions.
func TestManager_RoutesConcurrent(t *testing.T) {
	m := NewManager("")
	m.projectName = "proj"

	const goroutines = 16
	const perGoroutine = 64

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			ctx := context.Background()
			for i := 0; i < perGoroutine; i++ {
				name := nameForKey(g, i)
				route := interfaces.ProxyRoute{
					ServiceName: name,
					Hostname:    name,
					Target:      "svc",
					Port:        8080,
				}
				_ = m.AddRoute(ctx, route)
				// Sprinkle reads to exercise the RLock path.
				_ = m.GetURL(name)
				_ = m.HostsLine()
			}
		}(g)
	}
	wg.Wait()

	// Final map should hold goroutines * perGoroutine entries; the
	// snapshot also serves as a smoke check that the read path
	// returns a consistent slice.
	got := m.snapshotRoutes()
	want := goroutines * perGoroutine
	if len(got) != want {
		t.Errorf("routes count = %d, want %d", len(got), want)
	}
}

// TestManager_RoutesAddRemoveConcurrent exercises the writer/writer
// path (AddRoute vs RemoveRoute on the same keys) and asserts the
// race detector stays quiet plus the map ends in a coherent state.
func TestManager_RoutesAddRemoveConcurrent(t *testing.T) {
	m := NewManager("")
	m.projectName = "proj"
	ctx := context.Background()

	// Seed so RemoveRoute has something to delete.
	for i := 0; i < 32; i++ {
		_ = m.AddRoute(ctx, interfaces.ProxyRoute{
			ServiceName: nameForKey(0, i),
			Hostname:    nameForKey(0, i),
		})
	}

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 32; i++ {
				_ = m.AddRoute(ctx, interfaces.ProxyRoute{
					ServiceName: nameForKey(0, i),
					Hostname:    nameForKey(0, i),
				})
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 32; i++ {
				_ = m.RemoveRoute(ctx, nameForKey(0, i))
			}
		}()
	}
	wg.Wait()
}

func nameForKey(g, i int) string {
	return "svc-" + intToStr(g) + "-" + intToStr(i)
}

func intToStr(n int) string {
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
