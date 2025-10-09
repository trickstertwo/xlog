package zerolog

import (
	"time"

	"github.com/rs/zerolog"
	"github.com/trickstertwo/xlog"
)

// Adapter bridges xlog to rs/zerolog with low overhead.
//
// Optimizations:
//   - Pre-binds fields in With() by creating a child zerolog.Logger with those
//     fields attached, eliminating per-log bound-field loops.
//   - Fast pre-check using GetLevel() to avoid allocating zerolog.Event when
//     the level is disabled.
//   - Uses Logger.WithLevel(...) to avoid a level switch at call sites.
type Adapter struct {
	l zerolog.Logger
}

func New(l zerolog.Logger) *Adapter {
	return &Adapter{l: l}
}

// With returns a child adapter by binding fields onto a child zerolog.Logger.
// This applies the cost once, not per-log call.
func (a *Adapter) With(fs []xlog.Field) xlog.Adapter {
	if len(fs) == 0 {
		// Keep behavior consistent: return a shallow copy
		child := *a
		return &child
	}
	ctx := a.l.With()
	for i := range fs {
		ctx = appendCtxField(ctx, &fs[i])
	}
	child := *a
	child.l = ctx.Logger()
	return &child
}

// Log emits a single entry.
// - Single authoritative timestamp provided by xlog passed as "ts".
// - Fatal is treated as error level to avoid os.Exit side-effects.
func (a *Adapter) Log(level xlog.Level, msg string, at time.Time, fields []xlog.Field) {
	zlvl := mapLevel(level)

	// Fast path: drop early if below logger's min level (no Event allocation).
	if zlvl < a.l.GetLevel() {
		return
	}

	ev := a.l.WithLevel(zlvl)

	// Ensure RFC3339Nano precision regardless of zerolog.TimeFieldFormat defaults.
	// Using a string avoids global config changes and keeps output deterministic.
	ev.Str("ts", at.UTC().Format(time.RFC3339Nano))

	// Apply event fields
	for i := range fields {
		appendEventField(ev, &fields[i])
	}

	ev.Msg(msg)
}

// SetMinLevel allows xlog.Builder to propagate min level into zerolog (optional interface).
func (a *Adapter) SetMinLevel(l xlog.Level) {
	a.l = a.l.Level(mapLevel(l))
}

// mapLevel converts xlog.Level to zerolog.Level.
// xlog.LevelFatal is mapped to Error to avoid zerolog.Fatal() (which would exit the process).
func mapLevel(l xlog.Level) zerolog.Level {
	switch {
	case l <= xlog.LevelTrace:
		return zerolog.TraceLevel
	case l <= xlog.LevelDebug:
		return zerolog.DebugLevel
	case l <= xlog.LevelInfo:
		return zerolog.InfoLevel
	case l <= xlog.LevelWarn:
		return zerolog.WarnLevel
	case l <= xlog.LevelError:
		return zerolog.ErrorLevel
	default:
		return zerolog.ErrorLevel
	}
}

// appendEventField writes an xlog.Field to a zerolog.Event.
func appendEventField(e *zerolog.Event, f *xlog.Field) {
	switch f.Kind {
	case xlog.KindString:
		e.Str(f.K, f.Str)
	case xlog.KindInt64:
		e.Int64(f.K, f.Int64)
	case xlog.KindUint64:
		e.Uint64(f.K, f.Uint64)
	case xlog.KindFloat64:
		e.Float64(f.K, f.Float64)
	case xlog.KindBool:
		e.Bool(f.K, f.Bool)
	case xlog.KindDuration:
		e.Dur(f.K, f.Dur)
	case xlog.KindTime:
		e.Time(f.K, f.Time)
	case xlog.KindError:
		if f.Err != nil {
			if f.K == "" || f.K == "error" {
				e.Err(f.Err)
			} else {
				e.AnErr(f.K, f.Err)
			}
		}
	case xlog.KindBytes:
		e.Bytes(f.K, f.Bytes)
	case xlog.KindAny:
		e.Interface(f.K, f.Any)
	default:
		// Keep a placeholder to preserve shape
		e.Interface(f.K, nil)
	}
}

// appendCtxField binds a field to zerolog.Context (used by With()).
func appendCtxField(ctx zerolog.Context, f *xlog.Field) zerolog.Context {
	switch f.Kind {
	case xlog.KindString:
		return ctx.Str(f.K, f.Str)
	case xlog.KindInt64:
		return ctx.Int64(f.K, f.Int64)
	case xlog.KindUint64:
		return ctx.Uint64(f.K, f.Uint64)
	case xlog.KindFloat64:
		return ctx.Float64(f.K, f.Float64)
	case xlog.KindBool:
		return ctx.Bool(f.K, f.Bool)
	case xlog.KindDuration:
		return ctx.Dur(f.K, f.Dur)
	case xlog.KindTime:
		return ctx.Time(f.K, f.Time)
	case xlog.KindError:
		// Context supports Err(err) for default key; no named-error variant.
		if f.Err == nil {
			return ctx
		}
		if f.K == "" || f.K == "error" {
			return ctx.Err(f.Err)
		}
		return ctx.Str(f.K, f.Err.Error())
	case xlog.KindBytes:
		return ctx.Bytes(f.K, f.Bytes)
	case xlog.KindAny:
		return ctx.Interface(f.K, f.Any)
	default:
		return ctx.Interface(f.K, nil)
	}
}
