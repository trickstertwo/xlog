package slog

import (
	"io"
	"log/slog"
	"os"
	"time"

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
	Writer             io.Writer            // default: os.Stdout
	MinLevel           xlog.Level           // xlog + slog will both use this
	Format             Format               // JSON (default) or Text
	HandlerOptions     *slog.HandlerOptions // optional; Level is managed by Use via LevelVar
	TimestampFieldName string               // default "ts" (aligns with xlog's authoritative timestamp)
	Caller             bool                 // uses slog.AddSource via HandlerOptions if desired (set in HandlerOptions)
	_                  struct{}             // future-proofing
}

// Use builds a slog-backed xlog logger from Config, sets it as global, and returns it.
func Use(cfg Config) *xlog.Logger {
	w := cfg.Writer
	if w == nil {
		w = os.Stdout
	}
	if cfg.TimestampFieldName == "" {
		cfg.TimestampFieldName = "ts"
	}
	opts := cfg.HandlerOptions
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}

	// Use a LevelVar to allow dynamic SetMinLevel on the adapter.
	var lv slog.LevelVar
	lv.Set(slog.Level(cfg.MinLevel))
	opts.Level = &lv

	var h slog.Handler
	if cfg.Format == 0 || cfg.Format == FormatJSON {
		h = slog.NewJSONHandler(w, opts)
	} else {
		h = slog.NewTextHandler(w, opts)
	}
	sl := slog.New(h)

	ad := NewWithTimestampKey(sl, &lv, cfg.TimestampFieldName)
	ad.SetMinLevel(cfg.MinLevel)

	return xlog.UseAdapter(ad, cfg.MinLevel)
}

// Ensure we refer to time to avoid unused import in some build contexts.
var _ = time.Duration(0)
