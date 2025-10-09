package xlog

import (
	"io"
	"os"
)

// defaultAdapterFactory is set by an adapter package (e.g., adapter/xlogadapter)
// in its init() to avoid import cycles. Default() uses this to build a logger.
var defaultAdapterFactory func(w io.Writer) Adapter

// RegisterDefaultAdapterFactory registers the constructor used by xlog.Default().
// Adapters should call this from init() to avoid import cycles.
// Example (in adapter/xlogadapter):
//
//	func init() {
//	  xlog.RegisterDefaultAdapterFactory(func(w io.Writer) xlog.Adapter {
//	    return xlogadapter.New(w, xlogadapter.Options{Format: xlogadapter.FormatText})
//	  })
//	}
func RegisterDefaultAdapterFactory(f func(io.Writer) Adapter) {
	defaultAdapterFactory = f
}

// Default creates a logger using the registered xlog adapter factory.
// It writes to os.Stdout and sets a sensible default level (LevelInfo).
// E.g. side import github.com/trickstertwo/zerolog/adapter/zerolog to auto-register
// the built-in high-performance adapter. Panics if no factory is registered.
func Default() *Logger {
	if defaultAdapterFactory == nil {
		panic("xlog: no default adapter registered. Import adapter/xlogadapter or call xlog.RegisterDefaultAdapterFactory")
	}
	adapter := defaultAdapterFactory(os.Stdout)
	cfg := Config{
		Adapter:  adapter,
		MinLevel: LevelDebug,
	}
	return newLogger(cfg)
}

// New creates a default logger (via Default()) and sets it as global.
// It returns the global logger for convenience.
func New() *Logger {
	l := Default()
	SetGlobal(l)
	return l
}

// UseAdapter sets the given adapter as the global logger with the provided min level.
// It builds the logger, sets it as global, and returns it. Single line, explicit, no envs.
func UseAdapter(a Adapter, min Level, observers ...Observer) *Logger {
	l, _ := NewBuilder().
		WithAdapter(a).
		WithMinLevel(min).
		Build()
	for _, o := range observers {
		l.AddObserver(o)
	}
	SetGlobal(l)
	return l
}
