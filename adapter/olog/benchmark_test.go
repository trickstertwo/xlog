package olog

import (
	"io"
	"testing"
	"time"

	"github.com/trickstertwo/xlog"
)

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

// Text mode benchmarks

func BenchmarkXlogAdapter_Text_5Fields(b *testing.B) {
	a := New(discardWriter{}, Options{Format: FormatText})
	at := time.Date(2024, 12, 31, 23, 59, 59, 1, time.UTC)
	fields := []xlog.Field{
		{K: "a", Kind: xlog.KindString, Str: "b"},
		{K: "i", Kind: xlog.KindInt64, Int64: 42},
		{K: "ok", Kind: xlog.KindBool, Bool: true},
		{K: "dur", Kind: xlog.KindDuration, Dur: time.Millisecond},
		{K: "f", Kind: xlog.KindFloat64, Float64: 3.14},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Log(xlog.LevelInfo, "bench", at, fields)
	}
}

func BenchmarkXlogAdapter_Text_WithBound(b *testing.B) {
	a := New(discardWriter{}, Options{Format: FormatText})
	a2 := a.With([]xlog.Field{
		{K: "svc", Kind: xlog.KindString, Str: "api"},
		{K: "ver", Kind: xlog.KindString, Str: "1.0.0"},
	})
	at := time.Unix(0, 0).UTC()
	fields := []xlog.Field{
		{K: "path", Kind: xlog.KindString, Str: "/healthz"},
		{K: "code", Kind: xlog.KindInt64, Int64: 200},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a2.Log(xlog.LevelInfo, "probe", at, fields)
	}
}

func BenchmarkXlogAdapter_Text_NoFields(b *testing.B) {
	a := New(io.Discard, Options{Format: FormatText})
	at := time.Now()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Log(xlog.LevelInfo, "ok", at, nil)
	}
}

// JSON mode benchmarks

func BenchmarkXlogAdapter_JSON_5Fields(b *testing.B) {
	a := New(discardWriter{}, Options{Format: FormatJSON})
	at := time.Date(2024, 12, 31, 23, 59, 59, 1, time.UTC)
	fields := []xlog.Field{
		{K: "a", Kind: xlog.KindString, Str: "b"},
		{K: "i", Kind: xlog.KindInt64, Int64: 42},
		{K: "ok", Kind: xlog.KindBool, Bool: true},
		{K: "dur", Kind: xlog.KindDuration, Dur: time.Millisecond},
		{K: "f", Kind: xlog.KindFloat64, Float64: 3.14},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Log(xlog.LevelInfo, "bench", at, fields)
	}
}

func BenchmarkXlogAdapter_JSON_WithBound(b *testing.B) {
	a := New(discardWriter{}, Options{Format: FormatJSON})
	a2 := a.With([]xlog.Field{
		{K: "svc", Kind: xlog.KindString, Str: "api"},
		{K: "ver", Kind: xlog.KindString, Str: "1.0.0"},
	})
	at := time.Unix(0, 0).UTC()
	fields := []xlog.Field{
		{K: "path", Kind: xlog.KindString, Str: "/healthz"},
		{K: "code", Kind: xlog.KindInt64, Int64: 200},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a2.Log(xlog.LevelInfo, "probe", at, fields)
	}
}

func BenchmarkXlogAdapter_JSON_NoFields(b *testing.B) {
	a := New(io.Discard, Options{Format: FormatJSON})
	at := time.Now()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Log(xlog.LevelInfo, "ok", at, nil)
	}
}
