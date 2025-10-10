package xlog

import (
	"io"
	"os"

	root "github.com/trickstertwo/xlog"
)

// Config is an explicit, code-first configuration for the built-in xlog adapter.
// Use provides a single-call setup with no envs or side-imports.
type Config struct {
	// Writer routes all logs to this writer when WriterFactory is nil.
	// Defaults to os.Stdout.
	Writer io.Writer

	// WriterFactory optionally routes logs by level or destination.
	// When set, it takes precedence over Writer.
	WriterFactory WriterFactory

	// Core behavior (mirrors Options)
	MinLevel       root.Level
	Format         Format
	ErrorHandler   ErrorHandler
	Async          bool
	AsyncQueueSize int
	AsyncPolicy    AsyncDropPolicy
	DisableCaller  bool
	TimeFormat     string
	JSONTime       JSONTimeEncoding
	JSONDuration   JSONDurationEncoding
	BufferSize     int

	Metrics MetricsCollector // optional observability
}

// Use builds an xlog.Logger backed by the built-in adapter with Config,
// sets it as the global logger, and returns it.
// No envs, no init-time magic.
func Use(cfg Config) *root.Logger {
	// Build adapter options
	opts := Options{
		Format:         cfg.Format,
		MinLevel:       cfg.MinLevel,
		ErrorHandler:   cfg.ErrorHandler,
		Async:          cfg.Async,
		AsyncQueueSize: cfg.AsyncQueueSize,
		AsyncPolicy:    cfg.AsyncPolicy,
		DisableCaller:  cfg.DisableCaller,
		TimeFormat:     cfg.TimeFormat,
		JSONTime:       cfg.JSONTime,
		JSONDuration:   cfg.JSONDuration,
		BufferSize:     cfg.BufferSize,
	}

	var ad *Adapter
	if cfg.WriterFactory != nil {
		ad = NewWithWriterFactory(cfg.WriterFactory, opts)
	} else {
		w := cfg.Writer
		if w == nil {
			w = os.Stdout
		}
		ad = New(w, opts)
	}

	// Optional metrics hook
	if cfg.Metrics != nil {
		ad.SetMetricsCollector(cfg.Metrics)
	}

	// Keep xlog's filter and adapter's filter aligned.
	// UseAdapter builds and sets the global Logger.
	return root.UseAdapter(ad, cfg.MinLevel)
}
