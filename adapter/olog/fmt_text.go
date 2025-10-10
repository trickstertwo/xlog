package olog

import (
	"time"

	root "github.com/trickstertwo/xlog"
)

// Formatter writes one full line with the given, already pre-encoded bound prefix.
type Formatter interface {
	FormatLogLine(buf *buffer, level root.Level, msg string, at time.Time, boundPrefix []byte, fields []root.Field, opts Options)
}

type TextFormatter struct{}

var (
	textTsPrefix    = []byte("ts=")
	textLevelPrefix = []byte(" level=")
	textMsgPrefix   = []byte(" msg=")
	textTrue        = []byte("true")
	textFalse       = []byte("false")
	textNull        = []byte("null")
	textLenPrefix   = []byte("len:")
)

func (f *TextFormatter) FormatLogLine(buf *buffer, level root.Level, msg string, at time.Time, boundPrefix []byte, fields []root.Field, opts Options) {
	writeTextLine(buf, level, msg, at, boundPrefix, fields, opts)
	buf.writeByte('\n')
}

func writeTextLine(buf *buffer, level root.Level, msg string, at time.Time, boundPrefix []byte, fields []root.Field, opts Options) {
	buf.writeBytes(textTsPrefix)
	// Respect custom time format if provided (local time retained for performance; user can pass UTC times if desired)
	if opts.TimeFormat != "" {
		var tmp [64]byte
		b := at.AppendFormat(tmp[:0], opts.TimeFormat)
		buf.writeBytes(b)
	} else {
		appendRFC3339Nano(buf, at)
	}

	buf.writeBytes(textLevelPrefix)
	appendInt64(buf, int64(level))

	buf.writeBytes(textMsgPrefix)
	appendTextString(buf, msg)

	if len(boundPrefix) > 0 {
		buf.writeBytes(boundPrefix)
	}
	for i := range fields {
		appendTextField(buf, &fields[i])
	}
}

func appendTextField(buf *buffer, f *root.Field) {
	buf.writeByte(' ')
	buf.writeString(f.K)
	buf.writeByte('=')
	appendTextValue(buf, f)
}

func appendTextValue(buf *buffer, f *root.Field) {
	switch f.Kind {
	case root.KindString:
		appendTextString(buf, f.Str)
	case root.KindInt64:
		appendInt64(buf, f.Int64)
	case root.KindUint64:
		appendUint64(buf, f.Uint64)
	case root.KindFloat64:
		appendFloat64(buf, f.Float64)
	case root.KindBool:
		if f.Bool {
			buf.writeBytes(textTrue)
		} else {
			buf.writeBytes(textFalse)
		}
	case root.KindDuration:
		appendDuration(buf, f.Dur)
	case root.KindTime:
		appendRFC3339Nano(buf, f.Time)
	case root.KindError:
		if f.Err != nil {
			appendQuoted(buf, f.Err.Error())
		} else {
			buf.writeBytes(textNull)
		}
	case root.KindBytes:
		buf.writeBytes(textLenPrefix)
		appendInt64(buf, int64(len(f.Bytes)))
	case root.KindAny:
		appendTextAny(buf, f.Any)
	default:
		buf.writeBytes(textNull)
	}
}

func appendTextAny(buf *buffer, v any) {
	if v == nil {
		buf.writeBytes(textNull)
		return
	}
	switch vv := v.(type) {
	case string:
		appendTextString(buf, vv)
	case []byte:
		buf.writeBytes(textLenPrefix)
		appendInt64(buf, int64(len(vv)))
	case bool:
		if vv {
			buf.writeBytes(textTrue)
		} else {
			buf.writeBytes(textFalse)
		}
	case int:
		appendInt64(buf, int64(vv))
	case int8:
		appendInt64(buf, int64(vv))
	case int16:
		appendInt64(buf, int64(vv))
	case int32:
		appendInt64(buf, int64(vv))
	case int64:
		appendInt64(buf, vv)
	case uint:
		appendUint64(buf, uint64(vv))
	case uint8:
		appendUint64(buf, uint64(vv))
	case uint16:
		appendUint64(buf, uint64(vv))
	case uint32:
		appendUint64(buf, uint64(vv))
	case uint64:
		appendUint64(buf, vv)
	case float32:
		appendFloat64(buf, float64(vv))
	case float64:
		appendFloat64(buf, vv)
	case time.Time:
		appendRFC3339Nano(buf, vv)
	case time.Duration:
		appendDuration(buf, vv)
	default:
		// Minimal overhead "unknown" marker
		buf.writeString("unknown")
	}
}
