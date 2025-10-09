package slog

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/trickstertwo/xlog"
)

// Adapter bridges xlog to the Go slog API with low overhead.
//
// Optimizations:
//   - Pre-binds fields in With() by creating a child slog.Logger with those
//     attributes attached, eliminating per-log bound-field loops.
//   - Uses Logger.Enabled to avoid building attrs when level is disabled.
//   - Writes RFC3339Nano timestamp under a configurable key (default "ts") as a string
//     for deterministic precision across handlers and encoders.
//
// Optional behavior:
//   - SetMinLevel leverages slog.LevelVar when provided at construction time
//     to adjust backend filtering to match xlog's MinLevel. If no LevelVar is
//     provided, SetMinLevel is a no-op (xlog filtering still applies).
type Adapter struct {
	l     *slog.Logger
	lv    *slog.LevelVar // optional, enables SetMinLevel
	tsKey string         // timestamp field key; default "ts"
}

var bg = context.Background()

func toSlog(l xlog.Level) slog.Level { return slog.Level(l) }

// New creates an adapter for the provided slog logger.
func New(l *slog.Logger) *Adapter {
	if l == nil {
		l = slog.Default()
	}
	return &Adapter{l: l, tsKey: "ts"}
}

// NewWithLevelVar creates an adapter that can dynamically adjust slog level via SetMinLevel.
func NewWithLevelVar(l *slog.Logger, lv *slog.LevelVar) *Adapter {
	if l == nil {
		l = slog.Default()
	}
	return &Adapter{l: l, lv: lv, tsKey: "ts"}
}

// NewWithTimestampKey lets callers override the timestamp field key (default "ts").
func NewWithTimestampKey(l *slog.Logger, lv *slog.LevelVar, tsKey string) *Adapter {
	if l == nil {
		l = slog.Default()
	}
	if tsKey == "" {
		tsKey = "ts"
	}
	return &Adapter{l: l, lv: lv, tsKey: tsKey}
}

// With returns a child adapter by binding fields onto a child slog.Logger.
// This applies the cost once, not per-log call.
func (a *Adapter) With(fs []xlog.Field) xlog.Adapter {
	if len(fs) == 0 {
		child := *a
		return &child
	}
	attrs := make([]slog.Attr, 0, len(fs))
	for i := range fs {
		attrs = append(attrs, toAttr(&fs[i]))
	}
	// slog.Logger.With expects ...any; convert []slog.Attr -> []any
	args := make([]any, len(attrs))
	for i := range attrs {
		args[i] = attrs[i]
	}
	child := *a
	child.l = a.l.With(args...)
	return &child
}

// Log emits a single entry.
// - Uses xlog's authoritative timestamp as tsKey with RFC3339Nano precision.
// - Leverages slog.Enabled to skip work when a level is disabled.
func (a *Adapter) Log(level xlog.Level, msg string, at time.Time, fields []xlog.Field) {
	sl := toSlog(level)
	if !a.l.Enabled(bg, sl) {
		return
	}

	// Pre-size for ts + event fields (bound fields are baked into the logger).
	attrs := make([]slog.Attr, 0, 1+len(fields))

	// Deterministic timestamp precision across handlers/encoders.
	attrs = append(attrs, slog.String(a.tsKey, at.UTC().Format(time.RFC3339Nano)))

	for i := range fields {
		attrs = append(attrs, toAttr(&fields[i]))
	}

	a.l.LogAttrs(bg, sl, msg, attrs...)
}

// SetMinLevel updates the backend filter when a LevelVar was supplied.
// If not provided, this is a no-op (xlog filtering still applies).
func (a *Adapter) SetMinLevel(l xlog.Level) {
	if a.lv == nil {
		return
	}
	a.lv.Set(toSlog(l))
}

func toAttr(f *xlog.Field) slog.Attr {
	switch f.Kind {
	case xlog.KindString:
		return slog.String(f.K, f.Str)
	case xlog.KindInt64:
		return slog.Int64(f.K, f.Int64)
	case xlog.KindUint64:
		return slog.Uint64(f.K, f.Uint64)
	case xlog.KindFloat64:
		return slog.Float64(f.K, f.Float64)
	case xlog.KindBool:
		return slog.Bool(f.K, f.Bool)
	case xlog.KindDuration:
		return slog.Duration(f.K, f.Dur)
	case xlog.KindTime:
		return slog.Time(f.K, f.Time)
	case xlog.KindError:
		return slog.Any(f.K, f.Err)
	case xlog.KindBytes:
		return slog.Any(f.K, f.Bytes)
	case xlog.KindAny:
		return slog.Any(f.K, f.Any)
	default:
		return slog.Any(f.K, nil)
	}
}

// NewJSONLogger builds an xlog.Logger wired to a slog JSON handler.
// It uses a LevelVar so Adapter.SetMinLevel can adjust the backend level.
func NewJSONLogger(w io.Writer, minLevel xlog.Level, opts *slog.HandlerOptions, observers ...xlog.Observer) (*xlog.Logger, error) {
	if w == nil {
		w = os.Stdout
	}
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	var lv slog.LevelVar
	lv.Set(toSlog(minLevel))
	opts.Level = &lv

	handler := slog.NewJSONHandler(w, opts)
	sl := slog.New(handler)
	adapter := NewWithLevelVar(sl, &lv)

	b := xlog.NewBuilder().
		WithAdapter(adapter).
		WithMinLevel(minLevel)
	for _, o := range observers {
		b = b.AddObserver(o)
	}
	return b.Build()
}

// NewTextLogger builds an xlog.Logger wired to a slog text handler.
// It uses a LevelVar so Adapter.SetMinLevel can adjust the backend level.
func NewTextLogger(w io.Writer, minLevel xlog.Level, opts *slog.HandlerOptions, observers ...xlog.Observer) (*xlog.Logger, error) {
	if w == nil {
		w = os.Stdout
	}
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	var lv slog.LevelVar
	lv.Set(toSlog(minLevel))
	opts.Level = &lv

	handler := slog.NewTextHandler(w, opts)
	sl := slog.New(handler)
	adapter := NewWithLevelVar(sl, &lv)

	b := xlog.NewBuilder().
		WithAdapter(adapter).
		WithMinLevel(minLevel)
	for _, o := range observers {
		b = b.AddObserver(o)
	}
	return b.Build()
}
