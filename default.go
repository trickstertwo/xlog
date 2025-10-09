package xlog

import (
	"io"
	"os"
)

// defaultAdapterFactory is set by an adapter package in its init().
var defaultAdapterFactory func(w io.Writer) Adapter

func RegisterDefaultAdapterFactory(f func(io.Writer) Adapter) {
	defaultAdapterFactory = f
}

func Default() *Logger {
	if defaultAdapterFactory == nil {
		panic("xlog: no default adapter registered. Import an adapter (e.g., adapter/zerolog) or call xlog.RegisterDefaultAdapterFactory")
	}
	adapter := defaultAdapterFactory(os.Stdout)
	cfg := Config{
		Adapter:  adapter,
		MinLevel: LevelInfo, // align with docs and common defaults
	}
	return newLogger(cfg)
}

// UseDefault creates a default logger and sets it global.
func UseDefault() *Logger {
	l := Default()
	SetGlobal(l)
	return l
}

// UseAdapter sets the given adapter as the global logger with the provided min level.
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
