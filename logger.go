package xlog

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/trickstertwo/xclock"
)

type Logger struct {
	adapter    Adapter
	minLevel   Level
	baseFields []Field

	observersMu sync.RWMutex
	observers   []Observer
}

// Factory: internal constructor.
func newLogger(cfg Config) *Logger {
	l := &Logger{
		adapter:  cfg.Adapter,
		minLevel: cfg.MinLevel,
	}
	if len(cfg.Observers) > 0 {
		l.observers = append(l.observers, cfg.Observers...)
	}
	return l
}

// Facade: global access (Singleton + Facade).
var global atomic.Pointer[Logger]

// SetGlobal sets the global Logger (Singleton setter).
func SetGlobal(l *Logger) { global.Store(l) }

// L returns the global Logger; panic if unset to surface misconfig early.
func L() *Logger {
	l := global.Load()
	if l == nil {
		panic("xlog: global logger not set. Build one and call xlog.SetGlobal(...)")
	}
	return l
}

// Level entry points returning fluent builders.

func (l *Logger) Trace() *Event { return getEvent(l, LevelTrace) }
func (l *Logger) Debug() *Event { return getEvent(l, LevelDebug) }
func (l *Logger) Info() *Event  { return getEvent(l, LevelInfo) }
func (l *Logger) Warn() *Event  { return getEvent(l, LevelWarn) }
func (l *Logger) Error() *Event { return getEvent(l, LevelError) }
func (l *Logger) Fatal() *Event { return getEvent(l, LevelFatal) }

// With returns a child logger with bound fields.
func (l *Logger) With(fs ...Field) *Logger {
	return &Logger{
		adapter:    l.adapter.With(fs),
		minLevel:   l.minLevel,
		baseFields: append(copyFields(nil, l.baseFields), fs...),
		observers:  l.copyObservers(),
	}
}

func (l *Logger) copyObservers() []Observer {
	l.observersMu.RLock()
	defer l.observersMu.RUnlock()
	if len(l.observers) == 0 {
		return nil
	}
	out := make([]Observer, len(l.observers))
	copy(out, l.observers)
	return out
}

func (l *Logger) AddObserver(o Observer) {
	l.observersMu.Lock()
	l.observers = append(l.observers, o)
	l.observersMu.Unlock()
}

func (l *Logger) emit(level Level, msg string, evFields []Field) {
	if level < l.minLevel {
		return
	}
	// Single authoritative timestamp from xclock
	at := xclock.Now()

	// Fast path: adapter handles bound fields internally; pass only event fields.
	l.adapter.Log(level, msg, at, evFields)

	// Observers see combined fields: base + event.
	if len(l.observers) > 0 {
		merged := make([]Field, 0, len(l.baseFields)+len(evFields))
		if len(l.baseFields) > 0 {
			merged = append(merged, l.baseFields...)
		}
		if len(evFields) > 0 {
			merged = append(merged, evFields...)
		}
		entry := Entry{
			At:      at,
			Level:   level,
			Message: msg,
			Fields:  merged,
		}
		l.observersMu.RLock()
		obs := make([]Observer, len(l.observers))
		copy(obs, l.observers)
		l.observersMu.RUnlock()
		for _, o := range obs {
			o.OnLog(entry)
		}
	}
}

func copyFields(dst, src []Field) []Field {
	if len(src) == 0 {
		return dst
	}
	return append(dst, src...)
}

// Ensure time referenced; documents that adapters receive 'at'.
var _ time.Time
