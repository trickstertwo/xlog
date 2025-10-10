package main

import (
	"errors"
	"time"

	"github.com/trickstertwo/xclock/adapter/frozen"

	"github.com/trickstertwo/xlog"
	"github.com/trickstertwo/xlog/adapter/zerolog"
)

func main() {
	// Pin deterministic time for demo output.
	frozen.Use(frozen.Config{
		Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	})

	// Single explicit call, no envs, no blank-imports. Clear and predictable.
	zerolog.Use(zerolog.Config{
		MinLevel:          xlog.LevelDebug,
		Console:           false,
		ConsoleTimeFormat: time.RFC3339Nano,
		Caller:            true,
		CallerSkip:        5,
		// Writer:          os.Stdout,
	})

	// Basic Info with a few fields
	xlog.Info().
		Str("service", "payments").
		Int("port", 8080).
		Dur("boot", 125*time.Millisecond).
		Msg("listening")

	run()
}

func run() {
	// Child logger with bound fields
	reqLog := xlog.L().With(
		xlog.Str("request_id", "req-123"),
		xlog.Str("region", "eu-west-1"),
	)
	reqLog.Debug().Str("path", "/healthz").Int("code", 200).Msg("probe")

	// Demonstrate all field kinds (bound once; emitted once)
	allKinds := xlog.L().With(
		xlog.Str("k_string", "v"),
		xlog.Int64("k_int64", -42),
		xlog.Uint64("k_uint64", 42),
		xlog.Float64("k_float64", 3.14159),
		xlog.Bool("k_bool", true),
		xlog.Dur("k_duration", 250*time.Millisecond),
		xlog.Time("k_time", time.Date(2025, 1, 1, 7, 0, 0, 0, time.UTC)),
		xlog.Err("k_error", errors.New("boom")),
		xlog.Bytes("k_bytes", []byte{0xDE, 0xAD, 0xBE, 0xEF}),
		xlog.Any("k_any", map[string]any{"a": 1, "b": "two"}),
	)
	allKinds.Warn().Msg("all-kinds")

	// Show Fatal semantic (logged as error; does NOT exit)
	xlog.Fatal().Msg("fatal is logged as error (no os.Exit)")
}
