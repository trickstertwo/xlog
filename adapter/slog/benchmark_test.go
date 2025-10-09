package slog

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/trickstertwo/xlog"
)

func benchAdapter(b *testing.B, handler slog.Handler, fields []xlog.Field, bound []xlog.Field) {
	sl := slog.New(handler)

	// IMPORTANT: hold the adapter as the interface type, not the concrete *SlogAdapter
	var a xlog.Adapter = New(sl)
	if len(bound) > 0 {
		a = a.With(bound)
	}

	at := time.Date(2024, 12, 31, 23, 59, 59, 123456789, time.UTC)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Log(xlog.LevelInfo, "bench", at, fields)
	}
}

func BenchmarkSlogAdapter_JSON_5Fields(b *testing.B) {
	handler := slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo})
	fields := []xlog.Field{
		{K: "a", Kind: xlog.KindString, Str: "b"},
		{K: "i", Kind: xlog.KindInt64, Int64: 42},
		{K: "ok", Kind: xlog.KindBool, Bool: true},
		{K: "dur", Kind: xlog.KindDuration, Dur: time.Millisecond},
		{K: "f", Kind: xlog.KindFloat64, Float64: 3.14},
	}
	benchAdapter(b, handler, fields, nil)
}

func BenchmarkSlogAdapter_Text_5Fields(b *testing.B) {
	handler := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo})
	fields := []xlog.Field{
		{K: "a", Kind: xlog.KindString, Str: "b"},
		{K: "i", Kind: xlog.KindInt64, Int64: 42},
		{K: "ok", Kind: xlog.KindBool, Bool: true},
		{K: "dur", Kind: xlog.KindDuration, Dur: time.Millisecond},
		{K: "f", Kind: xlog.KindFloat64, Float64: 3.14},
	}
	benchAdapter(b, handler, fields, nil)
}

func BenchmarkSlogAdapter_JSON_WithBound(b *testing.B) {
	handler := slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo})
	fields := []xlog.Field{
		{K: "a", Kind: xlog.KindString, Str: "b"},
		{K: "i", Kind: xlog.KindInt64, Int64: 42},
	}
	bound := []xlog.Field{
		{K: "svc", Kind: xlog.KindString, Str: "api"},
		{K: "ver", Kind: xlog.KindString, Str: "1.0.0"},
		{K: "region", Kind: xlog.KindString, Str: "eu-west-1"},
	}
	benchAdapter(b, handler, fields, bound)
}
