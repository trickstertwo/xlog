package xlog

import "time"

type Kind uint8

const (
	KindString Kind = iota + 1
	KindInt64
	KindUint64
	KindFloat64
	KindBool
	KindDuration
	KindTime
	KindError
	KindBytes
	KindAny
)

// Field is a compact, reflection-free union for structured fields.
type Field struct {
	K       string
	Kind    Kind
	Str     string
	Int64   int64
	Uint64  uint64
	Float64 float64
	Bool    bool
	Dur     time.Duration
	Time    time.Time
	Err     error
	Bytes   []byte
	Any     any
}

// Entry is sent to Observers when an event is emitted.
type Entry struct {
	At      time.Time
	Level   Level
	Message string
	Fields  []Field
}

// Observer is notified for each emitted entry (Observer pattern).
type Observer interface {
	OnLog(entry Entry)
}

// ObserverFunc adapter.
type ObserverFunc func(Entry)

func (f ObserverFunc) OnLog(e Entry) { f(e) }

func FStr(k, v string) Field               { return Field{K: k, Kind: KindString, Str: v} }
func FInt(k string, v int64) Field         { return Field{K: k, Kind: KindInt64, Int64: v} }
func FUint(k string, v uint64) Field       { return Field{K: k, Kind: KindUint64, Uint64: v} }
func FFloat(k string, v float64) Field     { return Field{K: k, Kind: KindFloat64, Float64: v} }
func FBool(k string, v bool) Field         { return Field{K: k, Kind: KindBool, Bool: v} }
func FDur(k string, v time.Duration) Field { return Field{K: k, Kind: KindDuration, Dur: v} }
func FTime(k string, v time.Time) Field    { return Field{K: k, Kind: KindTime, Time: v} }
func FErr(k string, err error) Field       { return Field{K: k, Kind: KindError, Err: err} }
func FBytes(k string, b []byte) Field      { return Field{K: k, Kind: KindBytes, Bytes: b} }
func FAny(k string, v any) Field           { return Field{K: k, Kind: KindAny, Any: v} }
