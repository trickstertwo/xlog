package zap

import (
	"io"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/trickstertwo/xclock"
	"github.com/trickstertwo/xlog"
)

// Config is an explicit, code-first configuration for zap + xlog.
// No envs, no hidden init, one call to Use.
type Config struct {
	Writer             io.Writer // default: os.Stdout
	MinLevel           xlog.Level
	Console            bool                  // pretty console-like output via zapcore.NewConsoleEncoder
	EncoderConfig      zapcore.EncoderConfig // if zero, a sensible default is used
	Caller             bool                  // include caller in logs
	CallerSkip         int                   // frames to skip when resolving caller; default 2–5 typically
	TimestampFieldName string                // default "ts" (aligns with xlog's authoritative timestamp)
}

// Use builds a zap-backed xlog logger from Config,
// wires it as the global xlog logger, and returns it.
// Critically, it binds the logger to xclock.Default() so frozen/offset/jitter/calibrated clocks are respected in timestamps.
func Use(cfg Config) *xlog.Logger {
	w := cfg.Writer
	if w == nil {
		w = os.Stdout
	}
	if cfg.TimestampFieldName == "" {
		cfg.TimestampFieldName = "ts"
	}
	if cfg.Caller && cfg.CallerSkip <= 0 {
		cfg.CallerSkip = 2
	}

	// Encoder config defaults: do not let zap inject its own time (xlog provides "ts")
	encCfg := cfg.EncoderConfig
	if encCfg.TimeKey == "" && encCfg.LevelKey == "" && encCfg.MessageKey == "" && encCfg.EncodeTime == nil {
		encCfg = zapcore.EncoderConfig{
			TimeKey:        "", // xlog injects "ts"
			LevelKey:       "level",
			MessageKey:     "message",
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.RFC3339NanoTimeEncoder, // used if you yourself add zap.Time fields
			EncodeDuration: zapcore.StringDurationEncoder,
			CallerKey:      "caller",
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}
	} else {
		// Ensure zap itself doesn't add an extra time field
		encCfg.TimeKey = ""
	}

	var enc zapcore.Encoder
	if cfg.Console {
		enc = zapcore.NewConsoleEncoder(encCfg)
	} else {
		enc = zapcore.NewJSONEncoder(encCfg)
	}

	sink := zapcore.AddSync(w)

	// Use AtomicLevel so Adapter.SetMinLevel can adjust dynamically.
	al := zap.NewAtomicLevelAt(toZapLevel(cfg.MinLevel))
	core := zapcore.NewCore(enc, sink, al)

	opts := []zap.Option{
		zap.AddStacktrace(zapcore.FatalLevel + 1), // effectively off for normal levels
	}
	if cfg.Caller {
		opts = append(opts, zap.AddCaller(), zap.AddCallerSkip(cfg.CallerSkip))
	}

	zl := zap.New(core, opts...)

	// Wrap in adapter and set global
	ad := NewWithTimestampKey(zl, &al, cfg.TimestampFieldName)
	ad.SetMinLevel(cfg.MinLevel)

	// Build an xlog.Logger bound to the current process clock (xclock.Default()).
	logger, err := xlog.NewBuilder().
		WithAdapter(ad).
		WithMinLevel(cfg.MinLevel).
		WithClock(xclock.Default()).
		Build()
	if err != nil {
		panic(err)
	}

	xlog.SetGlobal(logger)
	return logger
}
