package main

import (
	"errors"
	"time"

	"github.com/trickstertwo/xclock/adapter/frozen"
	"go.uber.org/zap/zapcore"

	"github.com/trickstertwo/xlog"
	"github.com/trickstertwo/xlog/adapter/zap"
)

func main() {
	// Pin deterministic time for demo output.
	frozen.Use(frozen.Config{
		Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	})

	// Single explicit call, no envs, no blank-imports. Clear and predictable.
	zap.Use(zap.Config{
		MinLevel: xlog.LevelDebug, // xlog + zap both get this
		Console:  false,           // set to true for console encoder
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "", // xlog injects "ts"
			LevelKey:       "level",
			MessageKey:     "message",
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.RFC3339NanoTimeEncoder, // for any zap.Time you add yourself
			EncodeDuration: zapcore.StringDurationEncoder,
			CallerKey:      "caller",
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		Caller:     true,
		CallerSkip: 3, // adjust to land on your app callsite
		// Writer defaults to os.Stdout
	})

	// Basic Info with a few fields
	xlog.Info().
		Str("service", "payments").
		Int("port", 8080).
		Dur("boot", 125*time.Millisecond).
		Msg("listening")

	// Child logger with bound fields (best practice for request-scoped logging)
	reqLog := xlog.L().With(
		xlog.FStr("request_id", "req-123"),
		xlog.FStr("region", "eu-west-1"),
	)
	reqLog.Debug().Str("path", "/healthz").Int("code", 200).Msg("probe")

	// Demonstrate all field kinds
	xlog.L().With(
		xlog.FStr("k_string", "v"),
		xlog.FInt("k_int64", -42),
		xlog.FUint("k_uint64", 42),
		xlog.FFloat("k_float64", 3.14159),
		xlog.FBool("k_bool", true),
		xlog.FDur("k_duration", 250*time.Millisecond),
		xlog.FTime("k_time", time.Date(2025, 1, 1, 7, 0, 0, 0, time.UTC)),
		xlog.FErr("k_error", errors.New("boom")),
		xlog.FBytes("k_bytes", []byte{0xDE, 0xAD, 0xBE, 0xEF}),
		xlog.FAny("k_any", map[string]any{"a": 1, "b": "two"}),
	).Warn().Msg("all-kinds")

	// Show Fatal semantic (logged as error; does NOT exit)
	xlog.Fatal().Msg("fatal is logged as error (no os.Exit)")
}
