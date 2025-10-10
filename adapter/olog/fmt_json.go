package olog

import (
	"encoding/json"
	"math"
	"strconv"
	"time"

	root "github.com/trickstertwo/xlog"
)

type JSONFormatter struct{}

func (f *JSONFormatter) FormatLogLine(buf *buffer, level root.Level, msg string, at time.Time, boundPrefix []byte, fields []root.Field, opts Options) {
	writeJSONLine(buf, level, msg, at, boundPrefix, fields, opts)
	buf.writeByte('\n')
}

func writeJSONLine(buf *buffer, level root.Level, msg string, at time.Time, boundPrefix []byte, fields []root.Field, opts Options) {
	buf.writeByte('{')

	switch opts.JSONTime {
	case JSONTimeUnixMillis:
		buf.writeString(`"ts":`)
		appendInt64(buf, at.UnixMilli())
	case JSONTimeUnixNanos:
		buf.writeString(`"ts":`)
		appendInt64(buf, at.UnixNano())
	default: // RFC3339Nano
		buf.writeString(`"ts":"`)
		appendRFC3339Nano(buf, at)
		buf.writeByte('"')
	}

	buf.writeString(`,"level":`)
	appendInt64(buf, int64(level))

	buf.writeString(`,"msg":`)
	appendQuoted(buf, msg)

	if len(boundPrefix) > 0 {
		buf.writeBytes(boundPrefix)
	}
	for i := range fields {
		appendJSONField(buf, &fields[i], opts)
	}

	buf.writeByte('}')
}

func appendJSONField(buf *buffer, f *root.Field, opts Options) {
	buf.writeByte(',')
	appendQuoted(buf, f.K)
	buf.writeByte(':')

	switch f.Kind {
	case root.KindString:
		appendQuoted(buf, f.Str)
	case root.KindInt64:
		appendInt64(buf, f.Int64)
	case root.KindUint64:
		appendUint64(buf, f.Uint64)
	case root.KindFloat64:
		if math.IsNaN(f.Float64) || math.IsInf(f.Float64, 0) {
			buf.writeBytes(jsonNull)
		} else {
			var tmp [32]byte
			b := strconv.AppendFloat(tmp[:0], f.Float64, 'g', -1, 64)
			buf.writeBytes(b)
		}
	case root.KindBool:
		if f.Bool {
			buf.writeBytes(jsonTrue)
		} else {
			buf.writeBytes(jsonFalse)
		}
	case root.KindDuration:
		switch opts.JSONDuration {
		case JSONDurationMillis:
			appendInt64(buf, int64(f.Dur/time.Millisecond))
		case JSONDurationNanos:
			appendInt64(buf, f.Dur.Nanoseconds())
		default:
			appendQuoted(buf, f.Dur.String())
		}
	case root.KindTime:
		switch opts.JSONTime {
		case JSONTimeUnixMillis:
			appendInt64(buf, f.Time.UnixMilli())
		case JSONTimeUnixNanos:
			appendInt64(buf, f.Time.UnixNano())
		default:
			buf.writeByte('"')
			appendRFC3339Nano(buf, f.Time)
			buf.writeByte('"')
		}
	case root.KindError:
		if f.Err != nil {
			appendQuoted(buf, f.Err.Error())
		} else {
			buf.writeBytes(jsonNull)
		}
	case root.KindBytes:
		appendBase64(buf, f.Bytes)
	case root.KindAny:
		switch v := f.Any.(type) {
		case nil:
			buf.writeBytes(jsonNull)
		case RawJSON:
			if len(v) == 0 {
				buf.writeString(`""`)
			} else {
				buf.writeBytes(v)
			}
		case json.Marshaler:
			if data, err := v.MarshalJSON(); err == nil {
				buf.writeBytes(data)
			} else {
				buf.writeBytes(jsonNull)
			}
		case string:
			appendQuoted(buf, v)
		case []byte:
			appendBase64(buf, v)
		case bool:
			if v {
				buf.writeBytes(jsonTrue)
			} else {
				buf.writeBytes(jsonFalse)
			}
		case int:
			appendInt64(buf, int64(v))
		case int8:
			appendInt64(buf, int64(v))
		case int16:
			appendInt64(buf, int64(v))
		case int32:
			appendInt64(buf, int64(v))
		case int64:
			appendInt64(buf, v)
		case uint:
			appendUint64(buf, uint64(v))
		case uint8:
			appendUint64(buf, uint64(v))
		case uint16:
			appendUint64(buf, uint64(v))
		case uint32:
			appendUint64(buf, uint64(v))
		case uint64:
			appendUint64(buf, v)
		case float32:
			f := float64(v)
			if math.IsNaN(f) || math.IsInf(f, 0) {
				buf.writeBytes(jsonNull)
			} else {
				var tmp [32]byte
				b := strconv.AppendFloat(tmp[:0], f, 'g', -1, 32)
				buf.writeBytes(b)
			}
		case float64:
			f := v
			if math.IsNaN(f) || math.IsInf(f, 0) {
				buf.writeBytes(jsonNull)
			} else {
				var tmp [32]byte
				b := strconv.AppendFloat(tmp[:0], f, 'g', -1, 64)
				buf.writeBytes(b)
			}
		case time.Time:
			switch opts.JSONTime {
			case JSONTimeUnixMillis:
				appendInt64(buf, v.UnixMilli())
			case JSONTimeUnixNanos:
				appendInt64(buf, v.UnixNano())
			default:
				buf.writeByte('"')
				appendRFC3339Nano(buf, v)
				buf.writeByte('"')
			}
		case time.Duration:
			switch opts.JSONDuration {
			case JSONDurationMillis:
				appendInt64(buf, int64(v/time.Millisecond))
			case JSONDurationNanos:
				appendInt64(buf, v.Nanoseconds())
			default:
				appendQuoted(buf, v.String())
			}
		default:
			if data, err := json.Marshal(v); err == nil {
				buf.writeBytes(data)
			} else {
				buf.writeBytes(jsonNull)
			}
		}
	default:
		buf.writeBytes(jsonNull)
	}
}
