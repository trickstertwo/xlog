package main

import (
	"log/slog"
	"time"

	"github.com/trickstertwo/xclock"
	"github.com/trickstertwo/xlog"
	xslog "github.com/trickstertwo/xlog/adapter/slog"
)

func main() {
	// Deterministic time for demo output.
	old := xclock.Default()
	defer xclock.SetDefault(old)
	xclock.SetDefault(xclock.NewFrozen(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)))

	// Single explicit call, no envs, no blank-imports.
	// Uses slog JSON handler at Debug level; AddSource shows caller.
	xslog.Use(xslog.Config{
		MinLevel: xlog.LevelDebug,
		Format:   xslog.FormatJSON, // or slogadapter.FormatText
		HandlerOptions: &slog.HandlerOptions{
			Level:     slog.LevelDebug, // backend level (will also be synced via SetMinLevel)
			AddSource: true,            // optional: include caller
		},
		// TimestampFieldName: "ts", // default; override if you prefer a different key
	})

	// Two log lines: INFO and DEBUG
	xlog.Info().
		Str("service", "payments").
		Int("port", 8080).
		Dur("boot", 125*time.Millisecond).
		Msg("listening")

	reqLog := xlog.L().With(
		xlog.FStr("request_id", "req-123"),
		xlog.FStr("region", "eu-west-1"),
	)
	reqLog.Debug().Str("path", "/healthz").Int("code", 200).Msg("probe")
}
