package zap

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/trickstertwo/xlog"
)

// Adapter bridges xlog to go.uber.org/zap with low overhead.
//
// Optimizations:
//   - Pre-binds fields in With() by creating a child zap.Logger with those
//     fields attached, eliminating per-log bound-field loops.
//   - Uses Logger.Check(level, msg) to avoid building fields when disabled.
//   - Guarantees RFC3339Nano "ts" precision by writing it as a string field.
//
// Optional behavior:
//   - SetMinLevel leverages zap.AtomicLevel when provided at construction time
//     to adjust backend filtering to match xlog's MinLevel. If no AtomicLevel
//     is provided, SetMinLevel is a no-op (xlog filtering still applies).
type Adapter struct {
	l     *zap.Logger
	al    *zap.AtomicLevel // optional, enables SetMinLevel
	tsKey string           // timestamp field key; default "ts"
}

// New creates an adapter for the provided zap logger.
func New(l *zap.Logger) *Adapter {
	if l == nil {
		l = zap.NewNop()
	}
	return &Adapter{l: l, tsKey: "ts"}
}

// NewWithAtomicLevel creates an adapter and wires a zap.AtomicLevel so
// SetMinLevel can dynamically adjust the backend's filter.
func NewWithAtomicLevel(l *zap.Logger, al *zap.AtomicLevel) *Adapter {
	if l == nil {
		l = zap.NewNop()
	}
	return &Adapter{l: l, al: al, tsKey: "ts"}
}

// NewWithTimestampKey lets callers override the timestamp field key (default "ts").
func NewWithTimestampKey(l *zap.Logger, al *zap.AtomicLevel, tsKey string) *Adapter {
	if l == nil {
		l = zap.NewNop()
	}
	if tsKey == "" {
		tsKey = "ts"
	}
	return &Adapter{l: l, al: al, tsKey: tsKey}
}

// With returns a child adapter by binding fields onto a child zap.Logger.
// This applies the cost once, not per-log call.
func (a *Adapter) With(fs []xlog.Field) xlog.Adapter {
	if len(fs) == 0 {
		child := *a
		return &child
	}
	child := *a
	child.l = a.l.With(convertFields(fs)...)
	return &child
}

// Log emits a single entry.
// - Uses xlog's authoritative timestamp as tsKey with RFC3339Nano precision.
// - Maps LevelFatal to Error to avoid os.Exit in library code.
func (a *Adapter) Log(level xlog.Level, msg string, at time.Time, fields []xlog.Field) {
	zlvl := toZapLevel(level)

	// Fast path: skip if disabled. Avoids building fields.
	ce := a.l.Check(zlvl, msg)
	if ce == nil {
		return
	}

	// Pre-size for ts + event fields (bound fields are baked into the logger).
	zfs := make([]zap.Field, 0, 1+len(fields))

	// Ensure RFC3339Nano precision regardless of encoder defaults.
	zfs = append(zfs, zap.String(a.tsKey, at.UTC().Format(time.RFC3339Nano)))

	// Convert event fields
	for i := range fields {
		zfs = append(zfs, toZapField(&fields[i]))
	}

	ce.Write(zfs...)
}

// SetMinLevel updates the backend filter when an AtomicLevel was supplied.
// If not provided, this is a no-op (xlog filtering still applies).
func (a *Adapter) SetMinLevel(l xlog.Level) {
	if a.al == nil {
		return
	}
	a.al.SetLevel(toZapLevel(l))
}

func toZapLevel(l xlog.Level) zapcore.Level {
	switch {
	case l <= xlog.LevelTrace:
		return zapcore.DebugLevel // zap has no trace; map to debug
	case l <= xlog.LevelDebug:
		return zapcore.DebugLevel
	case l <= xlog.LevelInfo:
		return zapcore.InfoLevel
	case l <= xlog.LevelWarn:
		return zapcore.WarnLevel
	case l <= xlog.LevelError:
		return zapcore.ErrorLevel
	default:
		// Avoid Fatal/DPanic to prevent exits in library code.
		return zapcore.ErrorLevel
	}
}

func convertFields(fs []xlog.Field) []zap.Field {
	out := make([]zap.Field, len(fs))
	for i := range fs {
		out[i] = toZapField(&fs[i])
	}
	return out
}

func toZapField(f *xlog.Field) zap.Field {
	switch f.Kind {
	case xlog.KindString:
		return zap.String(f.K, f.Str)
	case xlog.KindInt64:
		return zap.Int64(f.K, f.Int64)
	case xlog.KindUint64:
		return zap.Uint64(f.K, f.Uint64)
	case xlog.KindFloat64:
		return zap.Float64(f.K, f.Float64)
	case xlog.KindBool:
		return zap.Bool(f.K, f.Bool)
	case xlog.KindDuration:
		return zap.Duration(f.K, f.Dur) // encoder decides string vs numeric
	case xlog.KindTime:
		return zap.Time(f.K, f.Time)
	case xlog.KindError:
		if f.Err == nil {
			return zap.Skip()
		}
		if f.K == "" || f.K == "error" {
			return zap.Error(f.Err)
		}
		return zap.NamedError(f.K, f.Err)
	case xlog.KindBytes:
		return zap.ByteString(f.K, f.Bytes)
	case xlog.KindAny:
		return zap.Any(f.K, f.Any)
	default:
		return zap.Skip()
	}
}
