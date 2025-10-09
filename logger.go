package xlog

import (
	"sync"
	"sync/atomic"

	"github.com/trickstertwo/xclock"
)

type Logger struct {
	adapter    Adapter
	minLevel   Level
	baseFields []Field

	// Observers: lock-free reads via atomic.Value; synchronized updates via obsMu.
	// Stored value is []Observer and MUST be treated as immutable by readers.
	observers atomic.Value // holds []Observer
	obsMu     sync.Mutex
}

// Factory: internal constructor.
func newLogger(cfg Config) *Logger {
	l := &Logger{
		adapter:  cfg.Adapter,
		minLevel: cfg.MinLevel,
	}
	if len(cfg.Observers) > 0 {
		obs := make([]Observer, len(cfg.Observers))
		copy(obs, cfg.Observers)
		l.observers.Store(obs)
	} else {
		l.observers.Store(([]Observer)(nil))
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

// Enabled reports whether logs at 'level' would be emitted by this logger.
// Use to avoid building fields in hot paths when disabled.
func (l *Logger) Enabled(level Level) bool {
	return level >= l.minLevel
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
	child := &Logger{
		adapter:    l.adapter.With(fs),
		minLevel:   l.minLevel,
		baseFields: append(copyFields(nil, l.baseFields), fs...),
	}
	// Inherit a snapshot of observers (same semantics as before).
	child.observers.Store(l.snapshotObservers())
	return child
}

func (l *Logger) snapshotObservers() []Observer {
	v := l.observers.Load()
	if v == nil {
		return nil
	}
	cur := v.([]Observer)
	if len(cur) == 0 {
		return nil
	}
	out := make([]Observer, len(cur))
	copy(out, cur)
	return out
}

func (l *Logger) AddObserver(o Observer) {
	l.obsMu.Lock()
	defer l.obsMu.Unlock()
	cur := l.snapshotObservers()
	cur = append(cur, o)
	l.observers.Store(cur)
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
	v := l.observers.Load()
	if v == nil {
		return
	}
	obs := v.([]Observer)
	if len(obs) == 0 {
		return
	}

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

	for _, o := range obs {
		o.OnLog(entry)
	}
}

func copyFields(dst, src []Field) []Field {
	if len(src) == 0 {
		return dst
	}
	return append(dst, src...)
}
