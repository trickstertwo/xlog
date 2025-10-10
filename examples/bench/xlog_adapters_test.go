package xlog_test

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/trickstertwo/xlog"
	xlogadapter "github.com/trickstertwo/xlog/adapter/olog"
	slogadapter "github.com/trickstertwo/xlog/adapter/slog"
	zapadapter "github.com/trickstertwo/xlog/adapter/zap"
	zerologadapter "github.com/trickstertwo/xlog/adapter/zerolog"
)

type scenario struct {
	name string
	newA func() xlog.Adapter
}

var (
	at = time.Date(2024, 12, 31, 23, 59, 59, 123_000_000, time.UTC)

	fields5 = []xlog.Field{
		{K: "a", Kind: xlog.KindString, Str: "b"},
		{K: "i", Kind: xlog.KindInt64, Int64: 42},
		{K: "ok", Kind: xlog.KindBool, Bool: true},
		{K: "dur", Kind: xlog.KindDuration, Dur: time.Millisecond},
		{K: "f", Kind: xlog.KindFloat64, Float64: 3.14},
	}

	bound3 = []xlog.Field{
		{K: "svc", Kind: xlog.KindString, Str: "api"},
		{K: "ver", Kind: xlog.KindString, Str: "1.0.0"},
		{K: "region", Kind: xlog.KindString, Str: "eu-west-1"},
	}

	msg = "bench"
)

// adapters returns a fresh slice to avoid accidental mutation across sub-benchmarks.
func adapters() []scenario {
	return []scenario{
		{
			name: "xlogadapter/JSON",
			newA: func() xlog.Adapter {
				return xlogadapter.New(io.Discard, xlogadapter.Options{Format: xlogadapter.FormatJSON})
			},
		},
		{
			name: "xlogadapter/Text",
			newA: func() xlog.Adapter {
				return xlogadapter.New(io.Discard, xlogadapter.Options{Format: xlogadapter.FormatText})
			},
		},
		{
			name: "zerolog/JSON",
			newA: func() xlog.Adapter {
				zl := zerolog.New(io.Discard)
				return zerologadapter.New(zl)
			},
		},
		{
			name: "zap/JSON",
			newA: func() xlog.Adapter {
				enc := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
					LevelKey:       "level",
					MessageKey:     "message",
					EncodeLevel:    zapcore.LowercaseLevelEncoder,
					EncodeTime:     zapcore.RFC3339NanoTimeEncoder,
					EncodeDuration: zapcore.StringDurationEncoder,
				})
				core := zapcore.NewCore(enc, zapcore.AddSync(io.Discard), zapcore.DebugLevel)
				return zapadapter.New(zap.New(core))
			},
		},
		{
			name: "slog/JSON",
			newA: func() xlog.Adapter {
				h := slog.NewJSONHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})
				return slogadapter.New(slog.New(h))
			},
		},
	}
}

func runBenchCase(b *testing.B, caseName string, fields []xlog.Field, bound []xlog.Field) {
	b.Run(caseName, func(b *testing.B) {
		for _, sc := range adapters() {
			b.Run(sc.name, func(b *testing.B) {
				a := sc.newA()
				if len(bound) > 0 {
					a = a.With(bound)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					a.Log(xlog.LevelInfo, msg, at, fields)
				}
			})
		}
	})
}

func BenchmarkNoFields(b *testing.B) {
	runBenchCase(b, "NoFields", nil, nil)
}

func BenchmarkFiveFields(b *testing.B) {
	runBenchCase(b, "5Fields", fields5, nil)
}

func BenchmarkWithBoundAndTwoFields(b *testing.B) {
	runBenchCase(b, "WithBound+2Fields", []xlog.Field{
		{K: "path", Kind: xlog.KindString, Str: "/healthz"},
		{K: "code", Kind: xlog.KindInt64, Int64: 200},
	}, bound3)
}
