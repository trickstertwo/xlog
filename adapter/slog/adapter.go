package slogadapter

import (
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/trickstertwo/xlog"
)

// SlogAdapter adapts xlog to the Go slog API (Adapter Strategy).
// It builds slog.Attrs directly for low overhead and uses LogAttrs.
type SlogAdapter struct {
	l     *slog.Logger
	bound []xlog.Field
}

func toSlog(l xlog.Level) slog.Level {
	return slog.Level(l)
}

func New(l *slog.Logger) *SlogAdapter {
	if l == nil {
		l = slog.Default()
	}
	return &SlogAdapter{l: l}
}

func (a *SlogAdapter) With(fs []xlog.Field) xlog.Adapter {
	child := *a
	child.bound = append(copyFields(nil, a.bound), fs...)
	return &child
}

func (a *SlogAdapter) Log(level xlog.Level, msg string, at time.Time, fields []xlog.Field) {
	attrs := make([]slog.Attr, 0, len(a.bound)+len(fields)+1)

	// Single authoritative timestamp provided by Logger
	attrs = append(attrs, slog.Time("ts", at))

	// bound fields
	for i := range a.bound {
		attrs = append(attrs, toAttr(a.bound[i]))
	}
	// event fields
	for i := range fields {
		attrs = append(attrs, toAttr(fields[i]))
	}

	// Use LogAttrs for minimal allocations
	a.l.LogAttrs(context.Background(), toSlog(level), msg, attrs...)
}

func toAttr(f xlog.Field) slog.Attr {
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

func copyFields(dst, src []xlog.Field) []xlog.Field {
	if len(src) == 0 {
		return dst
	}
	return append(dst, src...)
}

// NewJSONLogger builds an xlog.Logger wired to a slog JSON handler.
func NewJSONLogger(w io.Writer, minLevel xlog.Level, opts *slog.HandlerOptions, observers ...xlog.Observer) (*xlog.Logger, error) {
	if w == nil {
		w = os.Stdout
	}
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	opts.Level = slog.Level(minLevel)
	handler := slog.NewJSONHandler(w, opts)
	sl := slog.New(handler)
	adapter := New(sl)

	b := xlog.NewBuilder().
		WithAdapter(adapter).
		WithMinLevel(minLevel)
	for _, o := range observers {
		b = b.AddObserver(o)
	}
	return b.Build()
}

// NewTextLogger builds an xlog.Logger wired to a slog text handler.
func NewTextLogger(w io.Writer, minLevel xlog.Level, opts *slog.HandlerOptions, observers ...xlog.Observer) (*xlog.Logger, error) {
	if w == nil {
		w = os.Stdout
	}
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	opts.Level = slog.Level(minLevel)
	handler := slog.NewTextHandler(w, opts)
	sl := slog.New(handler)
	adapter := New(sl)

	b := xlog.NewBuilder().
		WithAdapter(adapter).
		WithMinLevel(minLevel)
	for _, o := range observers {
		b = b.AddObserver(o)
	}
	return b.Build()
}

// Ensure we refer to time to avoid unused import in some build contexts.
var _ = time.Duration(0)
