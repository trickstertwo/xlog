package xlog

import (
	"io"
	"sync/atomic"
	"time"

	"github.com/trickstertwo/xclock"
)

// Logger is a small facade that delegates to an Adapter, with a min level filter.
// Patterns: Facade, Strategy (Adapter), Observer, Singleton (global)
type Logger struct {
	ad     Adapter
	min    *atomic.Int32 // stores Level in int32; pointer to avoid copying atomic values
	clock  xclock.Clock
	obs    []Observer // immutable slice set at construction
	closed atomic.Bool
}

// New creates a new logger with the provided adapter and min level.
// For advanced construction (clock, observers, factory), prefer Builder.
func New(ad Adapter, min Level) *Logger {
	if ad == nil {
		ad = nopAdapter{}
	}
	l := &Logger{
		ad:    ad,
		min:   new(atomic.Int32),
		clock: xclock.System(),
	}
	l.min.Store(int32(min))
	return l
}

func newLogger(cfg Config) *Logger {
	clk := cfg.Clock
	if clk == nil {
		clk = xclock.System()
	}
	l := &Logger{
		ad:    cfg.Adapter,
		min:   new(atomic.Int32),
		clock: clk,
	}
	l.min.Store(int32(cfg.MinLevel))
	if len(cfg.Observers) > 0 {
		l.obs = append([]Observer(nil), cfg.Observers...)
	}
	return l
}

func (l *Logger) MinLevel() Level { return Level(l.min.Load()) }

func (l *Logger) SetMinLevel(min Level) {
	old := l.MinLevel()
	if old == min {
		return
	}
	l.min.Store(int32(min))
	// Optional propagation to adapter (if constructed via New, not Builder)
	if ls, ok := l.ad.(adapterLevelSetter); ok {
		ls.SetMinLevel(min)
	}
	l.notifyConfig(old, min)
}

// With returns a derived logger with bound fields.
func (l *Logger) With(fs ...Field) *Logger {
	return &Logger{
		ad:    l.ad.With(fs),
		min:   l.min,   // share the same atomic.Int32 pointer; do NOT copy atomic by value
		clock: l.clock, // share the same clock reference
		obs:   l.obs,   // observers slice is immutable
	}
}

// Event builder API (zerolog-style).

func (l *Logger) Trace() *Event { return getEvent(l, LevelTrace) }
func (l *Logger) Debug() *Event { return getEvent(l, LevelDebug) }
func (l *Logger) Info() *Event  { return getEvent(l, LevelInfo) }
func (l *Logger) Warn() *Event  { return getEvent(l, LevelWarn) }
func (l *Logger) Error() *Event { return getEvent(l, LevelError) }
func (l *Logger) Fatal() *Event { return getEvent(l, LevelFatal) }

// LogAt logs at the specified level (immediate form).
func (l *Logger) LogAt(level Level, msg string, fs ...Field) {
	l.emit(level, msg, fs)
}

// emit is the single emission path for both builder and immediate APIs.
func (l *Logger) emit(level Level, msg string, fs []Field) {
	if l.closed.Load() {
		return
	}
	if level < l.MinLevel() {
		return
	}
	// Snapshot time via platform abstraction.
	at := l.clock.Now()

	// Defensive copy to avoid adapter misuse and caller aliasing.
	var fields []Field
	if len(fs) > 0 {
		fields = append(make([]Field, 0, len(fs)), fs...)
	}

	l.ad.Log(level, msg, at, fields)
	l.notifyEvent(level, msg, at, fields)
}

// Close asks the adapter to release resources if supported.
func (l *Logger) Close() {
	if !l.closed.CompareAndSwap(false, true) {
		return
	}
	if c, ok := l.ad.(io.Closer); ok {
		_ = c.Close()
	}
}

// Observer notifications (best-effort, never panic).
func (l *Logger) notifyEvent(level Level, msg string, at time.Time, fields []Field) {
	if len(l.obs) == 0 {
		return
	}
	e := EventData{Level: level, Msg: msg, At: at}
	if len(fields) > 0 {
		e.Fields = append(make([]Field, 0, len(fields)), fields...)
	}
	for _, o := range l.obs {
		func(o Observer, e EventData) {
			defer func() { _ = recover() }()
			o.OnEvent(e)
		}(o, e)
	}
}

func (l *Logger) notifyConfig(old, new Level) {
	if len(l.obs) == 0 {
		return
	}
	c := ConfigChange{OldMin: old, NewMin: new}
	for _, o := range l.obs {
		func(o Observer, c ConfigChange) {
			defer func() { _ = recover() }()
			o.OnConfig(c)
		}(o, c)
	}
}

// Global singleton (Singleton pattern)

var global atomic.Value // *Logger

func init() {
	global.Store(New(nopAdapter{}, LevelInfo))
}

func SetGlobal(l *Logger) *Logger {
	if l == nil {
		panic("xlog: SetGlobal called with nil Logger")
	}
	global.Store(l)
	return l
}

// L returns the global logger.
func L() *Logger { return global.Load().(*Logger) }

// nopAdapter is a safe no-op adapter.
type nopAdapter struct{}

func (nopAdapter) With(_ []Field) Adapter                        { return nopAdapter{} }
func (nopAdapter) Log(_ Level, _ string, _ time.Time, _ []Field) {}
