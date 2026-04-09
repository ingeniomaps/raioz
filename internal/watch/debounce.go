package watch

import (
	"sync"
	"time"
)

// Debouncer groups rapid file change events per service into a single callback.
// This prevents multiple restarts when an editor writes temp files + rename.
type Debouncer struct {
	delay    time.Duration
	mu       sync.Mutex
	timers   map[string]*time.Timer
	callback func(serviceName string)
}

// NewDebouncer creates a Debouncer with the given delay and callback.
func NewDebouncer(delay time.Duration, callback func(serviceName string)) *Debouncer {
	return &Debouncer{
		delay:    delay,
		timers:   make(map[string]*time.Timer),
		callback: callback,
	}
}

// Trigger schedules a callback for the given service.
// If called again before the delay expires, the timer resets.
func (d *Debouncer) Trigger(serviceName string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if timer, ok := d.timers[serviceName]; ok {
		timer.Stop()
	}

	d.timers[serviceName] = time.AfterFunc(d.delay, func() {
		d.callback(serviceName)
		d.mu.Lock()
		delete(d.timers, serviceName)
		d.mu.Unlock()
	})
}

// Stop cancels all pending timers.
func (d *Debouncer) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, timer := range d.timers {
		timer.Stop()
	}
	d.timers = make(map[string]*time.Timer)
}
