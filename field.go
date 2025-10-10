package xlog

import (
	"time"
)

// Kind identifies the concrete type stored in a Field.
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

// Field is a typed key/value pair for structured logging.
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

// Helpers for ergonomics.

func Str(k, v string) Field             { return Field{K: k, Kind: KindString, Str: v} }
func Int64(k string, v int64) Field     { return Field{K: k, Kind: KindInt64, Int64: v} }
func Uint64(k string, v uint64) Field   { return Field{K: k, Kind: KindUint64, Uint64: v} }
func Float64(k string, v float64) Field { return Field{K: k, Kind: KindFloat64, Float64: v} }
func Bool(k string, v bool) Field       { return Field{K: k, Kind: KindBool, Bool: v} }
func Dur(k string, v time.Duration) Field {
	return Field{K: k, Kind: KindDuration, Dur: v}
}
func Time(k string, v time.Time) Field { return Field{K: k, Kind: KindTime, Time: v} }
func Err(k string, e error) Field      { return Field{K: k, Kind: KindError, Err: e} }
func Bytes(k string, b []byte) Field   { return Field{K: k, Kind: KindBytes, Bytes: b} }
func Any(k string, v any) Field        { return Field{K: k, Kind: KindAny, Any: v} }
