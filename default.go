package xlog

import (
	"sync/atomic"

	"github.com/trickstertwo/xclock"
)

// Default returns the process-wide logger (global singleton).
// If the global logger hasn't been initialized yet, it lazily installs and
// returns a minimal, safe logger that logs nothing (nop adapter) at Info level,
// bound to the current process clock (xclock.Default()).
//
// This mirrors xclock.Default() semantics and gives a clear, discoverable
// entrypoint for retrieving the active logger.
func Default() *Logger {
	if v := global.Load(); v != nil {
		return v.(*Logger)
	}
	// Fallback: install a minimal, safe logger.
	l := &Logger{
		ad:    nopAdapter{},
		min:   new(atomic.Int32),
		clock: xclock.Default(),
	}
	l.min.Store(int32(LevelInfo))
	global.Store(l)
	return l
}

// SetDefault sets the provided logger as the global singleton.
// This is a convenience alias for SetGlobal for semantic symmetry with xclock.
//
// Recommended for adapter setup. Thread-safe; replaces the global logger atomically.
func SetDefault(l *Logger) *Logger {
	return SetGlobal(l)
}
