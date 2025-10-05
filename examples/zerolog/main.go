package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/trickstertwo/xclock"
	"github.com/trickstertwo/xlog"
	_ "github.com/trickstertwo/xlog/adapter/zerolog" // register zerolog as xlog's default
)

func main() {
	// Deterministic time for demo output.
	old := xclock.Default()
	defer xclock.SetDefault(old)
	xclock.SetDefault(xclock.NewFrozen(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)))

	// 1) JSON output, no caller
	fmt.Println("\n=== JSON (no caller) ===")
	_ = os.Setenv("XLOG_MIN_LEVEL", "debug")
	_ = os.Unsetenv("XLOG_CONSOLE")
	_ = os.Unsetenv("XLOG_CALLER")
	xlog.New()
	runAllExamples()

	// 2) JSON output, with caller (adjust skip if needed)
	fmt.Println("\n=== JSON (with caller) ===")
	_ = os.Setenv("XLOG_CALLER", "1")
	_ = os.Setenv("XLOG_CALLER_SKIP", "5")
	xlog.New()
	runAllExamples()

	// 3) Console (pretty) output, with caller and custom time format
	fmt.Println("\n=== CONSOLE (pretty, with caller) ===")
	_ = os.Setenv("XLOG_CONSOLE", "1")
	_ = os.Setenv("XLOG_CONSOLE_TIMEFORMAT", time.RFC3339Nano)
	xlog.New()
	runAllExamples()
}

func runAllExamples() {
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

	// Demonstrate all field kinds (bound once; emitted once)
	allKinds := xlog.L().With(
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
	)
	allKinds.Warn().Msg("all-kinds")

	// Show Fatal semantic (logged as error; does NOT exit)
	xlog.Fatal().Msg("fatal is logged as error (no os.Exit)")
}
