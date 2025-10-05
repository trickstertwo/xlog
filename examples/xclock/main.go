package main

import (
	"os"
	"time"

	"github.com/trickstertwo/xclock"
	"github.com/trickstertwo/xlog"
	_ "github.com/trickstertwo/xlog/adapter/zerolog"
)

func main() {
	// Deterministic time for demo output.
	old := xclock.Default()
	defer xclock.SetDefault(old)
	xclock.SetDefault(xclock.NewFrozen(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)))

	// Ensure DEBUG logs are emitted by both xlog and zerolog.
	_ = os.Setenv("XLOG_MIN_LEVEL", "debug")

	// Build and set the global logger using the default adapter (zerolog via init()).
	xlog.New().With(xlog.FStr("app", "xbus-example"))

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
