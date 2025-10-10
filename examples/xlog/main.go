package main

import (
	"time"

	"github.com/trickstertwo/xclock/adapter/frozen"
	"github.com/trickstertwo/xlog"
	xadapter "github.com/trickstertwo/xlog/adapter/xlog"
)

func main() {
	// Pin deterministic time for demo output.
	frozen.Use(frozen.Config{
		Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	})

	// Single explicit call, no envs, no blank-imports. Clear and predictable.
	// Choose between FormatText and FormatJSON.
	xadapter.Use(xadapter.Config{
		MinLevel: xlog.LevelDebug,     // xlog + adapter filters aligned
		Format:   xadapter.FormatJSON, // or xadapter.FormatJSON
		// Async:  true, AsyncQueueSize: 2048, // optional async mode
		// Writer: os.Stdout,                 // optional (defaults to Stdout)
		// TimeFormat: time.RFC3339Nano,      // currently reserved; default RFC3339Nano
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
