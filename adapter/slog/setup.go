package slog

import (
	"io"
	stdslog "log/slog"
	"os"
	"time"

	"github.com/trickstertwo/xclock"
	"github.com/trickstertwo/xlog"
)

// Format selects the slog handler format.
type Format uint8

const (
	FormatJSON Format = iota + 1
)

// Config is an explicit, code-first configuration for slog + xlog.
// One call to Use wires a slog-backed xlog logger and sets it global.
type Config struct {
	Writer             io.Writer  // default: os.Stdout
	MinLevel           xlog.Level // xlog + slog will both use this
	Format             Format     // JSON (default)
	TimestampFieldName string     // default "ts" (aligns with xlog's authoritative timestamp)
	Caller             bool       // sets AddSource=true when requested
	_                  struct{}   // future-proofing
}

// Use builds a slog-backed xlog logger from Config, sets it as global, and returns it.
// It drops slog's default "time" field to avoid leaking real wall time and relies on xlog's "ts".
func Use(cfg Config) *xlog.Logger {
	w := cfg.Writer
	if w == nil {
		w = os.Stdout
	}
	if cfg.TimestampFieldName == "" {
		cfg.TimestampFieldName = "ts"
	}
	opts := &stdslog.HandlerOptions{}
	if cfg.Caller {
		opts.AddSource = true
	}

	// Use LevelVar so SetMinLevel can adjust dynamically.
	var lv stdslog.LevelVar
	lv.Set(stdslog.Level(cfg.MinLevel))
	opts.Level = &lv

	// Chain ReplaceAttr to drop slog's own time attribute while preserving user's ReplaceAttr.
	opts.ReplaceAttr = chainReplaceAttr(opts.ReplaceAttr, func(_ []string, a stdslog.Attr) stdslog.Attr {
		if a.Key == stdslog.TimeKey {
			return stdslog.Attr{} // drop default "time"
		}
		return a
	})

	// Handler
	var h stdslog.Handler
	switch cfg.Format {
	case FormatJSON, 0:
		h = stdslog.NewJSONHandler(w, opts)
	default:
		h = stdslog.NewJSONHandler(w, opts)
	}
	sl := stdslog.New(h)

	// Wrap in adapter and bind xlog to the current process clock (xclock.Default()).
	ad := NewWithTimestampKey(sl, &lv, cfg.TimestampFieldName)
	ad.SetMinLevel(cfg.MinLevel)

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

// chainReplaceAttr composes an existing ReplaceAttr with an extra step.
// newStep runs first; if it returns zero Attr, the attribute is dropped.
// Otherwise the possibly modified Attr is passed to userStep (if any).
func chainReplaceAttr(
	userStep func([]string, stdslog.Attr) stdslog.Attr,
	newStep func([]string, stdslog.Attr) stdslog.Attr,
) func([]string, stdslog.Attr) stdslog.Attr {
	return func(groups []string, a stdslog.Attr) stdslog.Attr {
		a = newStep(groups, a)
		if a.Equal(stdslog.Attr{}) {
			return a // dropped
		}
		if userStep != nil {
			a = userStep(groups, a)
		}
		return a
	}
}

// Ensure we refer to time to avoid unused import in some build contexts.
var _ = time.Duration(0)
