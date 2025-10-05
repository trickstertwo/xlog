package xlog

import "time"

// Adapter is the logging backend Strategy (e.g., slog wrapper).
// Log receives the single authoritative timestamp 'at' from the Logger to avoid
// multiple time reads and ensure consistency across adapter and observers.
type Adapter interface {
	Log(level Level, msg string, at time.Time, fields []Field)
	With(fields []Field) Adapter // return a child adapter with bound fields (do not mutate receiver)
}
