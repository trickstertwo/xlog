package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/trickstertwo/xclock"
	"github.com/trickstertwo/xlog"
	"github.com/trickstertwo/xlog/adapter/slog"
)

func main() {
	// Pin deterministic time for demo output
	old := xclock.Default()
	defer xclock.SetDefault(old)
	xclock.SetDefault(xclock.NewFrozen(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)))

	// Configure slog JSON handler with Debug level
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	sl := slog.New(h)

	// Wire xlog -> slog adapter
	adapter := slogadapter.New(sl)
	logger, err := xlog.NewBuilder().
		WithAdapter(adapter).
		WithMinLevel(xlog.LevelDebug).
		Build()
	if err != nil {
		panic(err)
	}
	xlog.SetGlobal(logger)

	// Two log lines: INFO and DEBUG
	xlog.Info().
		Str("service", "payments").
		Int("port", 8080).
		Dur("boot", 125*time.Millisecond).
		Msg("listening")

	reqLog := xlog.L().With(
		xlog.Field{K: "request_id", Kind: xlog.KindString, Str: "req-123"},
		xlog.Field{K: "region", Kind: xlog.KindString, Str: "eu-west-1"},
	)
	reqLog.Debug().Str("path", "/healthz").Int("code", 200).Msg("probe")
}
