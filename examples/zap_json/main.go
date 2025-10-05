package main

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/trickstertwo/xclock"
	"github.com/trickstertwo/xlog"
	zapadapter "github.com/trickstertwo/xlog/adapter/zap"
)

func main() {
	// Optional: pin a deterministic clock for demo
	old := xclock.Default()
	defer xclock.SetDefault(old)
	xclock.SetDefault(xclock.NewFrozen(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)))

	// Configure a JSON zap logger that writes to stdout.
	encCfg := zapcore.EncoderConfig{
		TimeKey:        "", // we inject our own "ts"
		LevelKey:       "level",
		MessageKey:     "message",
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.RFC3339NanoTimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
	}
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encCfg),
		zapcore.AddSync(os.Stdout),
		zapcore.DebugLevel, // allow debug through zap
	)
	zl := zap.New(core)

	// Wrap with xlog's zap adapter and set global logger.
	adapter := zapadapter.New(zl)
	logger, err := xlog.NewBuilder().
		WithAdapter(adapter).
		WithMinLevel(xlog.LevelDebug). // allow debug through xlog
		Build()
	if err != nil {
		panic(err)
	}
	xlog.SetGlobal(logger)

	// Zerolog-style fluent API (INFO)
	xlog.Info().
		Str("service", "payments").
		Int("port", 8080).
		Dur("boot", 125*time.Millisecond).
		Msg("listening")

	// Child logger with bound fields (DEBUG)
	reqLog := xlog.L().With(
		xlog.Field{K: "request_id", Kind: xlog.KindString, Str: "req-123"},
		xlog.Field{K: "region", Kind: xlog.KindString, Str: "eu-west-1"},
	)
	reqLog.Debug().Str("path", "/healthz").Int("code", 200).Msg("probe")
}
