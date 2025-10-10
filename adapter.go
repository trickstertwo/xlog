package xlog

import "time"

// Adapter is the pluggable strategy that emits logs. It must be concurrency-safe.
// Patterns: Adapter + Strategy
type Adapter interface {
	// With binds fields, returning a derived Adapter with immutable bound fields.
	With(fs []Field) Adapter
	// Log emits a single log event.
	// Implementations MUST NOT retain or mutate the fields slice after return.
	Log(level Level, msg string, at time.Time, fields []Field)
}
