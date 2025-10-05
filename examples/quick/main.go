package main

import (
	"os"
	"time"

	"github.com/trickstertwo/xlog"
	_ "github.com/trickstertwo/xlog/adapter/zerolog" // register zerolog as xlog's default
)

func main() {
	// Allow DEBUG logs, pretty console, and include caller.
	_ = os.Setenv("XLOG_MIN_LEVEL", "debug")

	// Enable pretty console print
	_ = os.Setenv("XLOG_CONSOLE", "1")

	// Enable Caller func
	_ = os.Setenv("XLOG_CALLER", "1")
	// Tip: If the frame isn’t your callsite,
	// tweak XLOG_CALLER_SKIP between 4–7 depending on inlining and build flags.
	_ = os.Setenv("XLOG_CALLER_SKIP", "5")

	// Build and set the global logger using the default adapter (zerolog via init()).
	xlog.New()

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
