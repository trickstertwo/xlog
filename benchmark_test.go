package xlog

import (
	"testing"
	"time"

	"github.com/trickstertwo/xclock"
)

// blackhole variables prevent compiler from optimizing away code paths.
var (
	bhI   int64
	bhT   time.Time
	bhLen int
)

type nopAdapter struct {
	bound []Field
}

func (a *nopAdapter) With(fs []Field) Adapter {
	child := *a
	if len(a.bound) > 0 {
		child.bound = append([]Field(nil), a.bound...)
	}
	child.bound = append(child.bound, fs...)
	return &child
}

func (a *nopAdapter) Log(level Level, msg string, at time.Time, fields []Field) {
	// Touch inputs to avoid elimination; do not allocate.
	if len(a.bound)+len(fields) == -1 {
		bhI++
	}
	bhT = at
	bhLen = len(fields)
}

func newBenchLogger(min Level) *Logger {
	l, err := NewBuilder().
		WithAdapter(&nopAdapter{}).
		WithMinLevel(min).
		Build()
	if err != nil {
		panic(err)
	}
	return l
}

func BenchmarkInfo_NoFields(b *testing.B) {
	l := newBenchLogger(LevelDebug)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info().Msg("ok")
	}
}

func BenchmarkInfo_5Fields(b *testing.B) {
	l := newBenchLogger(LevelDebug)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info().
			Str("a", "b").
			Int("i", i).
			Bool("ok", true).
			Dur("d", time.Millisecond*25).
			Float64("f", 1.23).
			Msg("five")
	}
}

func BenchmarkInfo_10Fields(b *testing.B) {
	l := newBenchLogger(LevelDebug)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info().
			Str("a", "b").
			Str("c", "d").
			Str("e", "f").
			Str("g", "h").
			Str("i", "j").
			Int("k", i).
			Int64("l", int64(i)).
			Uint64("m", uint64(i)).
			Bool("n", i%2 == 0).
			Dur("o", time.Second).
			Msg("ten")
	}
}

func BenchmarkFiltered_NoFields(b *testing.B) {
	// Min level WARN filters INFO immediately after level check.
	l := newBenchLogger(LevelWarn)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info().Msg("not-logged")
	}
}

func BenchmarkFiltered_10Fields(b *testing.B) {
	// Still builds fields, then fast-returns on level check.
	l := newBenchLogger(LevelError)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info().
			Str("a", "b").
			Str("c", "d").
			Str("e", "f").
			Str("g", "h").
			Str("i", "j").
			Int("k", i).
			Int64("l", int64(i)).
			Uint64("m", uint64(i)).
			Bool("n", i%2 == 0).
			Dur("o", time.Second).
			Msg("ten-filtered")
	}
}

func BenchmarkChild_Bound4_Event4(b *testing.B) {
	l := newBenchLogger(LevelDebug)
	child := l.With(
		Field{K: "svc", Kind: KindString, Str: "api"},
		Field{K: "ver", Kind: KindString, Str: "1.0.0"},
		Field{K: "region", Kind: KindString, Str: "eu-west-1"},
		Field{K: "debug", Kind: KindBool, Bool: true},
	)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		child.Info().
			Str("path", "/healthz").
			Int("code", 200).
			Dur("lat", 2*time.Millisecond).
			Bool("hit", true).
			Msg("ok")
	}
}

func BenchmarkParallel_10Fields(b *testing.B) {
	l := newBenchLogger(LevelDebug)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			l.Debug().
				Str("k1", "v1").
				Str("k2", "v2").
				Str("k3", "v3").
				Str("k4", "v4").
				Int("i1", i).
				Int64("i2", int64(i)).
				Uint64("u1", uint64(i)).
				Bool("b1", i%2 == 0).
				Dur("d1", time.Millisecond).
				Float64("f1", 3.14).
				Msg("p")
			i++
		}
	})
}

// Optional: benchmark impact of xclock swap to a frozen clock (deterministic time)
// to observe any difference vs default fast-path system clock.
func BenchmarkInfo_FrozenClock(b *testing.B) {
	orig := xclock.Default()
	defer xclock.SetDefault(orig)
	xclock.SetDefault(xclock.NewFrozen(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)))

	l := newBenchLogger(LevelDebug)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info().Str("k", "v").Msg("frozen")
	}
}
