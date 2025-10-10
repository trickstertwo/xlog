package zerolog

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/trickstertwo/xclock"
	"github.com/trickstertwo/xlog"
)

// Config is an explicit, code-first configuration for zerolog + xlog.
// No envs, no hidden init, one call to Use.
type Config struct {
	Writer             io.Writer // default: os.Stdout
	MinLevel           xlog.Level
	Console            bool   // pretty console output instead of JSON
	ConsoleTimeFormat  string // only used if Console==true; default time.RFC3339Nano
	Caller             bool   // include caller in logs
	CallerSkip         int    // frames to skip when resolving caller; default 5
	TimestampFieldName string // default "ts" (aligns with xlog's authoritative timestamp)
}

// Use builds a zerolog-backed xlog logger from Config, wires it as the global
// xlog logger, and returns it. Critically, it binds the logger to xclock.Default()
// so frozen/offset/jitter/calibrated clocks are respected in timestamps.
func Use(cfg Config) *xlog.Logger {
	w := cfg.Writer
	if w == nil {
		w = os.Stdout
	}
	if cfg.TimestampFieldName == "" {
		cfg.TimestampFieldName = "ts"
	}
	if cfg.Caller && cfg.CallerSkip <= 0 {
		cfg.CallerSkip = 5
	}

	// Build zerolog.Logger according to Config
	var zl zerolog.Logger
	if cfg.Console {
		// Align console’s leading timestamp column with our authoritative ts key
		zerolog.TimestampFieldName = cfg.TimestampFieldName
		cw := zerolog.ConsoleWriter{Out: w}
		if cfg.ConsoleTimeFormat == "" {
			cw.TimeFormat = time.RFC3339Nano
		} else {
			cw.TimeFormat = cfg.ConsoleTimeFormat
		}
		zl = zerolog.New(cw)
	} else {
		zl = zerolog.New(w)
	}

	// Level
	zl = zl.Level(mapLevel(cfg.MinLevel))

	// Caller
	if cfg.Caller {
		zerolog.CallerSkipFrameCount = cfg.CallerSkip
		zl = zl.With().Caller().Logger()
	}

	// Wrap in adapter
	ad := New(zl)
	// Propagate min level down to zerolog (optional interface)
	ad.SetMinLevel(cfg.MinLevel)

	// Build an xlog.Logger bound to the current process clock (xclock.Default()).
	logger, err := xlog.NewBuilder().
		WithAdapter(ad).
		WithMinLevel(cfg.MinLevel).
		WithClock(xclock.Default()).
		Build()
	if err != nil {
		// In practice, Build only fails with a nil adapter which cannot happen here.
		// Keep panic to surface programming errors early.
		panic(err)
	}

	// Set as global and return.
	// If your xlog version exposes SetGlobal, prefer it.
	// Otherwise, you may have a helper like xlog.UseAdapter which creates a new logger.
	xlog.SetGlobal(logger)
	return logger
}
