package olog

import (
	"io"

	root "github.com/trickstertwo/xlog"
)

// Format defines the output format for log entries
type Format uint8

// RawJSON is a sentinel type that tells the adapter to splice the provided bytes
// directly into the JSON output (no quoting, no escaping). The content MUST be
// valid JSON. Use for maximum performance when you already have pre-encoded data.
type RawJSON []byte

const (
	FormatText Format = iota + 1
	FormatJSON
)

// ErrorHandler defines how logging errors are handled
type ErrorHandler func(error)

// JSONTimeEncoding controls how the "ts" field is encoded in JSON.
type JSONTimeEncoding uint8

const (
	JSONTimeRFC3339Nano JSONTimeEncoding = iota + 1 // default (backward compatible)
	JSONTimeUnixMillis                              // numeric, t.UnixMilli()
	JSONTimeUnixNanos                               // numeric, t.UnixNano()
)

// JSONDurationEncoding controls how time.Duration fields are encoded in JSON.
type JSONDurationEncoding uint8

const (
	JSONDurationString JSONDurationEncoding = iota + 1 // default (e.g., "1ms")
	JSONDurationMillis                                 // numeric milliseconds
	JSONDurationNanos                                  // numeric nanoseconds
)

// AsyncDropPolicy controls behavior when async queue is full.
type AsyncDropPolicy uint8

const (
	DropNewest AsyncDropPolicy = iota // fast, no producer stall (default)
	DropOldest                        // discard oldest entry to make room
	Block                             // producer blocks until space available
)

// Options configures the adapter behavior
type Options struct {
	Format         Format
	MinLevel       root.Level
	ErrorHandler   ErrorHandler
	Async          bool
	AsyncQueueSize int
	AsyncPolicy    AsyncDropPolicy
	DisableCaller  bool // reserved
	TimeFormat     string

	// JSON-specific performance toggles (opt-in)
	JSONTime     JSONTimeEncoding     // default JSONTimeRFC3339Nano
	JSONDuration JSONDurationEncoding // default JSONDurationString

	// Buffer tuning: initial capacity of format buffer
	// Defaults to 2048 when <= 0
	BufferSize int
}

// WriterFactory allows custom writers per log level
type WriterFactory interface {
	GetWriter(level root.Level) io.Writer
}

type DefaultWriterFactory struct{ Writer io.Writer }

func (f *DefaultWriterFactory) GetWriter(level root.Level) io.Writer { return f.Writer }

type LevelWriterFactory struct {
	Default     io.Writer
	LevelWriter map[root.Level]io.Writer
}

func (f *LevelWriterFactory) GetWriter(level root.Level) io.Writer {
	if w, ok := f.LevelWriter[level]; ok {
		return w
	}
	return f.Default
}
