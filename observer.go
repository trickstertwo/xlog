package xlog

import (
	"time"
)

// Observer pattern

// EventData is a read-only snapshot of an emitted log event.
type EventData struct {
	Level  Level
	Msg    string
	At     time.Time
	Fields []Field // copy per emit; safe to hold
}

// ConfigChange captures logger configuration updates of interest to observers.
type ConfigChange struct {
	OldMin Level
	NewMin Level
}

// Observer receives notifications for events and config changes.
// Implementations MUST be concurrency-safe.
type Observer interface {
	OnEvent(e EventData)
	OnConfig(c ConfigChange)
}
