package olog

import (
	"encoding/base64"
	"math"
	"strconv"
	"time"
)

const digits = "0123456789abcdef"
const minInt64Str = "-9223372036854775808"

var (
	jsonTrue  = []byte("true")
	jsonFalse = []byte("false")
	jsonNull  = []byte("null")
)

func appendInt64(buf *buffer, v int64) {
	if v == 0 {
		buf.writeByte('0')
		return
	}
	if v < 0 {
		if v == -1<<63 {
			buf.writeString(minInt64Str)
			return
		}
		buf.writeByte('-')
		v = -v
	}
	appendUint64(buf, uint64(v))
}

func appendUint64(buf *buffer, v uint64) {
	if v == 0 {
		buf.writeByte('0')
		return
	}
	var tmp [20]byte
	i := len(tmp)
	for v > 0 {
		i--
		tmp[i] = byte('0' + v%10)
		v /= 10
	}
	buf.writeBytes(tmp[i:])
}

func appendFloat64(buf *buffer, f float64) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		if math.IsNaN(f) {
			buf.writeString("NaN")
		} else if math.IsInf(f, 1) {
			buf.writeString("+Inf")
		} else {
			buf.writeString("-Inf")
		}
		return
	}
	var tmp [32]byte
	b := strconv.AppendFloat(tmp[:0], f, 'g', -1, 64)
	buf.writeBytes(b)
}

func appendDuration(buf *buffer, d time.Duration) { buf.writeString(d.String()) }

func appendRFC3339Nano(buf *buffer, t time.Time) {
	var tmp [64]byte
	b := t.AppendFormat(tmp[:0], time.RFC3339Nano)
	buf.writeBytes(b)
}

func appendBase64(buf *buffer, data []byte) {
	if len(data) == 0 {
		buf.writeString(`""`)
		return
	}
	buf.writeByte('"')
	encodedLen := base64.StdEncoding.EncodedLen(len(data))
	buf.grow(encodedLen + 1)
	start := len(buf.b)
	buf.b = buf.b[:start+encodedLen]
	base64.StdEncoding.Encode(buf.b[start:], data)
	buf.writeByte('"')
}
