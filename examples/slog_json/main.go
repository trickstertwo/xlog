package main

import (
	"time"

	"github.com/trickstertwo/xclock/adapter/frozen"
	"github.com/trickstertwo/xlog"
	"github.com/trickstertwo/xlog/adapter/slog"
)

func main() {
	// Pin deterministic time for demo output.
	frozen.Use(frozen.Config{
		Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	})

	// Single explicit call, no envs, no blank-imports.
	// Uses slog JSON handler at Debug level; AddSource shows caller.
	slog.Use(slog.Config{
		MinLevel: xlog.LevelDebug,
		Format:   slog.FormatJSON,
	})

	// Two log lines: INFO and DEBUG
	xlog.Info().
		Str("service", "payments").
		Int("port", 8080).
		Dur("boot", 125*time.Millisecond).
		Msg("listening")

	reqLog := xlog.L().With(
		xlog.Str("request_id", "req-123"),
		xlog.Str("region", "eu-west-1"),
	)
	reqLog.Debug().Str("path", "/healthz").Int("code", 200).Msg("probe")
}
