package xlog

import (
	"sync"
	"time"
)

// Event is a fluent builder (Builder pattern) for a single log entry.
// API: Logger().Info().Str("from", ...).Dur("to", dur).Int("to", v).Msg("state changed")

type Event struct {
	l      *Logger
	level  Level
	fields []Field
}

var eventPool = sync.Pool{
	New: func() any { return &Event{fields: make([]Field, 0, 8)} },
}

func getEvent(l *Logger, level Level) *Event {
	ev := eventPool.Get().(*Event)
	ev.l = l
	ev.level = level
	ev.fields = ev.fields[:0]
	return ev
}

func (e *Event) putBack() {
	// allow GC of large backing arrays by capping
	if cap(e.fields) > 128 {
		e.fields = make([]Field, 0, 8)
	}
	e.l = nil
	e.level = 0
	eventPool.Put(e)
}

// Field builders (zerolog-style)

func (e *Event) Str(k, v string) *Event {
	e.fields = append(e.fields, Field{K: k, Kind: KindString, Str: v})
	return e
}

func (e *Event) Int(k string, v int) *Event { return e.Int64(k, int64(v)) }

func (e *Event) Int64(k string, v int64) *Event {
	e.fields = append(e.fields, Field{K: k, Kind: KindInt64, Int64: v})
	return e
}

func (e *Event) Uint64(k string, v uint64) *Event {
	e.fields = append(e.fields, Field{K: k, Kind: KindUint64, Uint64: v})
	return e
}

func (e *Event) Float64(k string, v float64) *Event {
	e.fields = append(e.fields, Field{K: k, Kind: KindFloat64, Float64: v})
	return e
}

func (e *Event) Bool(k string, v bool) *Event {
	e.fields = append(e.fields, Field{K: k, Kind: KindBool, Bool: v})
	return e
}

func (e *Event) Dur(k string, v time.Duration) *Event {
	e.fields = append(e.fields, Field{K: k, Kind: KindDuration, Dur: v})
	return e
}

func (e *Event) Time(k string, v time.Time) *Event {
	e.fields = append(e.fields, Field{K: k, Kind: KindTime, Time: v})
	return e
}

func (e *Event) Bytes(k string, v []byte) *Event {
	e.fields = append(e.fields, Field{K: k, Kind: KindBytes, Bytes: v})
	return e
}

func (e *Event) Err(err error) *Event {
	if err == nil {
		return e
	}
	e.fields = append(e.fields, Field{K: "error", Kind: KindError, Err: err})
	return e
}

func (e *Event) Any(k string, v any) *Event {
	e.fields = append(e.fields, Field{K: k, Kind: KindAny, Any: v})
	return e
}

// Msg terminates the builder and emits the event.
func (e *Event) Msg(msg string) {
	e.l.emit(e.level, msg, e.fields)
	e.putBack()
}
