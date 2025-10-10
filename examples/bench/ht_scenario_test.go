package xlog_test

import (
	"io"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/trickstertwo/xlog"
	xlogadapter "github.com/trickstertwo/xlog/adapter/olog"
	zapadapter "github.com/trickstertwo/xlog/adapter/zap"
	zerologadapter "github.com/trickstertwo/xlog/adapter/zerolog"
)

type htScenario struct {
	name string
	newA func() xlog.Adapter
}

var (
	htAt  = time.Date(2024, 12, 31, 23, 59, 59, 123_000_000, time.UTC)
	htMsg = "bench"
)

// Only test: xlog adapter (JSON), zerolog, zap.
func htAdapters() []htScenario {
	return []htScenario{
		{
			name: "xlogadapter/JSON",
			newA: func() xlog.Adapter {
				return xlogadapter.New(io.Discard, xlogadapter.Options{
					Format:   xlogadapter.FormatJSON,
					MinLevel: xlog.LevelDebug,
				})
			},
		},
		{
			name: "zerolog/JSON",
			newA: func() xlog.Adapter {
				zl := zerolog.New(io.Discard).Level(zerolog.DebugLevel)
				return zerologadapter.New(zl)
			},
		},
		{
			name: "zap/JSON",
			newA: func() xlog.Adapter {
				enc := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
					TimeKey:        "", // xlog adapter injects "ts"
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
	}
}

func genFieldsN(n int) []xlog.Field {
	if n <= 0 {
		return nil
	}
	fs := make([]xlog.Field, 0, n)
	for i := 0; i < n; i++ {
		switch i % 5 {
		case 0:
			fs = append(fs, xlog.Field{K: "s", Kind: xlog.KindString, Str: "v"})
		case 1:
			fs = append(fs, xlog.Field{K: "i", Kind: xlog.KindInt64, Int64: int64(i)})
		case 2:
			fs = append(fs, xlog.Field{K: "b", Kind: xlog.KindBool, Bool: (i&1 == 0)})
		case 3:
			fs = append(fs, xlog.Field{K: "d", Kind: xlog.KindDuration, Dur: time.Millisecond})
		default:
			fs = append(fs, xlog.Field{K: "f", Kind: xlog.KindFloat64, Float64: 3.14159})
		}
	}
	return fs
}

func genBoundN(n int) []xlog.Field {
	return genFieldsN(n)
}

func runHTCase(b *testing.B, caseName string, fields []xlog.Field, bound []xlog.Field) {
	b.Run(caseName, func(b *testing.B) {
		for _, sc := range htAdapters() {
			b.Run(sc.name, func(b *testing.B) {
				a := sc.newA()
				if len(bound) > 0 {
					a = a.With(bound)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					a.Log(xlog.LevelInfo, htMsg, htAt, fields)
				}
			})
		}
	})
}

func runHTCaseParallel(b *testing.B, caseName string, fields []xlog.Field, bound []xlog.Field) {
	b.Run(caseName, func(b *testing.B) {
		for _, sc := range htAdapters() {
			b.Run(sc.name, func(b *testing.B) {
				a := sc.newA()
				if len(bound) > 0 {
					a = a.With(bound)
				}
				b.ReportAllocs()
				b.ResetTimer()
				b.RunParallel(func(pb *testing.PB) {
					for pb.Next() {
						a.Log(xlog.LevelInfo, htMsg, htAt, fields)
					}
				})
			})
		}
	})
}

// -------- Serial high-throughput style (tight loop) --------

func BenchmarkHT_Serial_NoFields(b *testing.B) {
	runHTCase(b, "Serial/NoFields", nil, nil)
}

func BenchmarkHT_Serial_5Fields(b *testing.B) {
	runHTCase(b, "Serial/5Fields", genFieldsN(5), nil)
}

func BenchmarkHT_Serial_10Fields(b *testing.B) {
	runHTCase(b, "Serial/10Fields", genFieldsN(10), nil)
}

func BenchmarkHT_Serial_20Fields(b *testing.B) {
	runHTCase(b, "Serial/20Fields", genFieldsN(20), nil)
}

func BenchmarkHT_Serial_Bound3_2Fields(b *testing.B) {
	runHTCase(b, "Serial/Bound3+2Fields", genFieldsN(2), genBoundN(3))
}

func BenchmarkHT_Serial_Bound5_5Fields(b *testing.B) {
	runHTCase(b, "Serial/Bound5+5Fields", genFieldsN(5), genBoundN(5))
}

// -------- Parallel (many goroutines) --------

func BenchmarkHT_Parallel_NoFields(b *testing.B) {
	runHTCaseParallel(b, "Parallel/NoFields", nil, nil)
}

func BenchmarkHT_Parallel_5Fields(b *testing.B) {
	runHTCaseParallel(b, "Parallel/5Fields", genFieldsN(5), nil)
}

func BenchmarkHT_Parallel_10Fields(b *testing.B) {
	runHTCaseParallel(b, "Parallel/10Fields", genFieldsN(10), nil)
}

func BenchmarkHT_Parallel_20Fields(b *testing.B) {
	runHTCaseParallel(b, "Parallel/20Fields", genFieldsN(20), nil)
}

func BenchmarkHT_Parallel_Bound3_2Fields(b *testing.B) {
	runHTCaseParallel(b, "Parallel/Bound3+2Fields", genFieldsN(2), genBoundN(3))
}

func BenchmarkHT_Parallel_Bound5_5Fields(b *testing.B) {
	runHTCaseParallel(b, "Parallel/Bound5+5Fields", genFieldsN(5), genBoundN(5))
}
