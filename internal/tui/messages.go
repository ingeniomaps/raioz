package tui

import "time"

// StatsMsg carries updated resource stats for all services.
type StatsMsg struct {
	Stats map[string]ServiceStats
}

// ServiceStats holds CPU and memory for one service.
type ServiceStats struct {
	CPU    string
	Memory string
	Status string
	Health string
	Uptime string
}

// LogMsg carries a single log line for a service.
type LogMsg struct {
	Service string
	Line    string
}

// ActionResultMsg reports the result of a user action (restart, stop).
type ActionResultMsg struct {
	Service string
	Action  string
	Err     error
}

// TickMsg triggers periodic stats polling.
type TickMsg time.Time

// LogStreamStartedMsg signals that log streaming has begun for a service.
type LogStreamStartedMsg struct {
	Service string
}
