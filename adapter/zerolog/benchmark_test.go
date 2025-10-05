package zerologadapter

import (
	"io"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/trickstertwo/xlog"
)

func benchAdapter(b *testing.B, zl zerolog.Logger, fields []xlog.Field, bound []xlog.Field) {
	var a xlog.Adapter = New(zl)
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

func BenchmarkZerologAdapter_JSON_5Fields(b *testing.B) {
	zl := zerolog.New(io.Discard)
	fields := []xlog.Field{
		{K: "a", Kind: xlog.KindString, Str: "b"},
		{K: "i", Kind: xlog.KindInt64, Int64: 42},
		{K: "ok", Kind: xlog.KindBool, Bool: true},
		{K: "dur", Kind: xlog.KindDuration, Dur: time.Millisecond},
		{K: "f", Kind: xlog.KindFloat64, Float64: 3.14},
	}
	benchAdapter(b, zl, fields, nil)
}

func BenchmarkZerologAdapter_JSON_WithBound(b *testing.B) {
	zl := zerolog.New(io.Discard)
	fields := []xlog.Field{
		{K: "a", Kind: xlog.KindString, Str: "b"},
		{K: "i", Kind: xlog.KindInt64, Int64: 42},
	}
	bound := []xlog.Field{
		{K: "svc", Kind: xlog.KindString, Str: "api"},
		{K: "ver", Kind: xlog.KindString, Str: "1.0.0"},
		{K: "region", Kind: xlog.KindString, Str: "eu-west-1"},
	}
	benchAdapter(b, zl, fields, bound)
}

func BenchmarkZerologAdapter_JSON_NoFields(b *testing.B) {
	zl := zerolog.New(io.Discard)
	at := time.Unix(0, 0).UTC()
	var a xlog.Adapter = New(zl)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Log(xlog.LevelInfo, "ok", at, nil)
	}
}
