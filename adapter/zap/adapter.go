package zapadapter

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/trickstertwo/xlog"
)

// Adapter bridges xlog to go.uber.org/zap with low overhead.
// - Applies a single authoritative timestamp `ts` from xlog.
// - Avoids zap.Fatal() to prevent os.Exit; maps LevelFatal to Error.
type Adapter struct {
	l     *zap.Logger
	bound []zap.Field
}

func New(l *zap.Logger) *Adapter {
	if l == nil {
		l = zap.NewNop()
	}
	return &Adapter{l: l}
}

func (a *Adapter) With(fs []xlog.Field) xlog.Adapter {
	child := *a
	// Pre-convert bound fields to zap.Field for minimal per-call overhead.
	if len(a.bound) > 0 {
		child.bound = append([]zap.Field(nil), a.bound...)
	}
	if len(fs) > 0 {
		child.bound = append(child.bound, convertFields(fs)...)
	}
	return &child
}

func (a *Adapter) Log(level xlog.Level, msg string, at time.Time, fields []xlog.Field) {
	zlvl := toZapLevel(level)
	ce := a.l.Check(zlvl, msg)
	if ce == nil {
		return
	}

	// ts + bound + event fields
	zfs := make([]zap.Field, 0, 1+len(a.bound)+len(fields))
	zfs = append(zfs, zap.Time("ts", at))
	if len(a.bound) > 0 {
		zfs = append(zfs, a.bound...)
	}
	if len(fields) > 0 {
		zfs = append(zfs, convertFields(fields)...)
	}
	ce.Write(zfs...)
}

func toZapLevel(l xlog.Level) zapcore.Level {
	switch {
	case l <= xlog.LevelTrace:
		return zapcore.DebugLevel // zap has no trace, map to debug
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
	out := make([]zap.Field, 0, len(fs))
	for i := range fs {
		out = append(out, toZapField(&fs[i]))
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
		return zap.Duration(f.K, f.Dur)
	case xlog.KindTime:
		return zap.Time(f.K, f.Time)
	case xlog.KindError:
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
