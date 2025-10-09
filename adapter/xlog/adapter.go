package xlog

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/trickstertwo/xlog"
)

// Format defines the output format for log entries
type Format uint8

// RawJSON is a sentinel type that tells the adapter to splice the provided bytes
// directly into the JSON output (no quoting, no escaping). The content MUST be
// valid JSON. Use for maximum performance when you already have pre-encoded data.
type RawJSON []byte

const (
	FormatText Format = iota + 1
	FormatJSON
)

// ErrorHandler defines how logging errors are handled
type ErrorHandler func(error)

// JSONTimeEncoding controls how the "ts" field is encoded in JSON.
type JSONTimeEncoding uint8

const (
	JSONTimeRFC3339Nano JSONTimeEncoding = iota + 1 // default (backward compatible)
	JSONTimeUnixMillis                              // numeric, t.UnixMilli()
	JSONTimeUnixNanos                               // numeric, t.UnixNano()
)

// JSONDurationEncoding controls how time.Duration fields are encoded in JSON.
type JSONDurationEncoding uint8

const (
	JSONDurationString JSONDurationEncoding = iota + 1 // default (e.g., "1ms")
	JSONDurationMillis                                 // numeric milliseconds
	JSONDurationNanos                                  // numeric nanoseconds
)

// Options configures the adapter behavior
type Options struct {
	Format         Format
	MinLevel       xlog.Level
	ErrorHandler   ErrorHandler
	Async          bool
	AsyncQueueSize int
	DisableCaller  bool // reserved
	TimeFormat     string

	// JSON-specific performance toggles (opt-in)
	JSONTime     JSONTimeEncoding     // default JSONTimeRFC3339Nano
	JSONDuration JSONDurationEncoding // default JSONDurationString
}

// WriterFactory allows custom writers per log level
type WriterFactory interface {
	GetWriter(level xlog.Level) io.Writer
}

type DefaultWriterFactory struct{ Writer io.Writer }

func (f *DefaultWriterFactory) GetWriter(level xlog.Level) io.Writer { return f.Writer }

type LevelWriterFactory struct {
	Default     io.Writer
	LevelWriter map[xlog.Level]io.Writer
}

func (f *LevelWriterFactory) GetWriter(level xlog.Level) io.Writer {
	if w, ok := f.LevelWriter[level]; ok {
		return w
	}
	return f.Default
}

// Formatter writes one full line with the given, already pre-encoded bound prefix.
type Formatter interface {
	FormatLogLine(buf *buffer, level xlog.Level, msg string, at time.Time, boundPrefix []byte, fields []xlog.Field, opts Options)
}

type TextFormatter struct{}
type JSONFormatter struct{}

// Metrics
type MetricsCollector interface {
	LoggedMessage(level xlog.Level, durMS float64, size int, err error)
}
type NoopMetricsCollector struct{}

func (*NoopMetricsCollector) LoggedMessage(level xlog.Level, durMS float64, size int, err error) {}

// sentinel error to avoid per-drop allocations on hot path
var errAsyncQueueFull = errors.New("xlog: async queue full, dropping log entry")

// Adapter is a high-throughput logger with pre-encoded bound prefixes and minimal allocs.
type Adapter struct {
	// immutable after construction
	writerFactory WriterFactory
	opts          Options
	formatter     Formatter

	// write path
	mu           *sync.Mutex
	metrics      atomic.Value // holds MetricsCollector
	wg           *sync.WaitGroup
	asyncQueue   chan asyncLogEntry
	stopped      atomic.Bool
	measureDur   atomic.Bool
	loggedErrors atomic.Uint64
	dropped      atomic.Uint64

	// bound fields (immutable)
	bound        []xlog.Field
	preBoundText []byte // ' key=value' slices
	preBoundJSON []byte // ',"key":value' slices

	// fast path for single writer
	singleWriter bool
	w            io.Writer
}

type asyncLogEntry struct {
	level  xlog.Level
	msg    string
	at     time.Time
	fields []xlog.Field
}

func defaultErrorHandler(err error) { fmt.Fprintf(os.Stderr, "xlog error: %v\n", err) }

// New creates a new Adapter with the given writer and options
func New(w io.Writer, opts Options) *Adapter {
	return NewWithWriterFactory(&DefaultWriterFactory{Writer: w}, opts)
}

func NewWithWriterFactory(factory WriterFactory, opts Options) *Adapter {
	if factory == nil {
		factory = &DefaultWriterFactory{Writer: os.Stdout}
	}
	if opts.Format == 0 {
		opts.Format = FormatText
	}
	if opts.ErrorHandler == nil {
		opts.ErrorHandler = defaultErrorHandler
	}
	// JSON performance toggles default to backward-compatible behavior
	if opts.JSONTime == 0 {
		opts.JSONTime = JSONTimeRFC3339Nano
	}
	if opts.JSONDuration == 0 {
		opts.JSONDuration = JSONDurationString
	}

	var formatter Formatter
	if opts.Format == FormatJSON {
		formatter = &JSONFormatter{}
	} else {
		formatter = &TextFormatter{}
	}

	a := &Adapter{
		writerFactory: factory,
		opts:          opts,
		formatter:     formatter,
		mu:            &sync.Mutex{},
		wg:            &sync.WaitGroup{},
	}

	// metrics defaults
	a.metrics.Store(MetricsCollector(&NoopMetricsCollector{}))
	a.measureDur.Store(false)

	if df, ok := factory.(*DefaultWriterFactory); ok {
		a.singleWriter = true
		a.w = df.Writer
	}

	if opts.Async {
		q := opts.AsyncQueueSize
		if q <= 0 {
			q = 1024
		}
		a.asyncQueue = make(chan asyncLogEntry, q)
		go a.asyncProcessor()
	}
	return a
}

// SetMetricsCollector installs a collector; when not Noop, we also measure durations.
func (a *Adapter) SetMetricsCollector(collector MetricsCollector) {
	if collector == nil {
		collector = &NoopMetricsCollector{}
	}
	a.metrics.Store(collector)
	_, isNoop := collector.(*NoopMetricsCollector)
	a.measureDur.Store(!isNoop)
}

func (a *Adapter) Close() error {
	if a.asyncQueue != nil {
		a.stopped.Store(true)
		close(a.asyncQueue)
		a.wg.Wait()
	}
	return nil
}

// With clones the adapter and pre-encodes bound fields into immutable prefixes.
func (a *Adapter) With(fs []xlog.Field) xlog.Adapter {
	child := &Adapter{
		writerFactory: a.writerFactory,
		opts:          a.opts,
		formatter:     a.formatter,
		mu:            a.mu,
		wg:            a.wg,
		asyncQueue:    a.asyncQueue,
		singleWriter:  a.singleWriter,
		w:             a.w,
	}
	// inherit metrics atomically
	child.metrics.Store(a.metrics.Load())
	child.measureDur.Store(a.measureDur.Load())

	// Copy existing bound
	if n := len(a.bound); n > 0 {
		child.bound = make([]xlog.Field, n, n+len(fs))
		copy(child.bound, a.bound)
	}
	// Append new bound
	if len(fs) > 0 {
		child.bound = append(child.bound, fs...)
	}
	// Pre-encode prefixes once (immutable)
	if len(child.bound) > 0 {
		child.preBoundText = encodeBoundText(child.bound)
		child.preBoundJSON = encodeBoundJSON(child.bound, child.opts)
	}
	return child
}

func (a *Adapter) Log(level xlog.Level, msg string, at time.Time, fields []xlog.Field) {
	if level < a.opts.MinLevel {
		return
	}
	if a.asyncQueue != nil && !a.stopped.Load() {
		c := make([]xlog.Field, len(fields))
		copy(c, fields)
		select {
		case a.asyncQueue <- asyncLogEntry{level: level, msg: msg, at: at, fields: c}:
			return
		default:
			a.dropped.Add(1)
			a.loggedErrors.Add(1)
			a.opts.ErrorHandler(errAsyncQueueFull)
			return
		}
	}
	a.logDirect(level, msg, at, fields)
}

func (a *Adapter) logDirect(level xlog.Level, msg string, at time.Time, fields []xlog.Field) {
	measure := a.measureDur.Load()
	mc := a.metrics.Load().(MetricsCollector)

	var start time.Time
	if measure {
		start = time.Now()
	}
	buf := getBuf()
	defer putBuf(buf)

	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic during log formatting: %v", r)
			a.loggedErrors.Add(1)
			a.opts.ErrorHandler(err)
			mc.LoggedMessage(level, 0, 0, err)
		}
	}()

	var boundPrefix []byte
	if a.opts.Format == FormatJSON {
		boundPrefix = a.preBoundJSON
	} else {
		boundPrefix = a.preBoundText
	}

	a.formatter.FormatLogLine(buf, level, msg, at, boundPrefix, fields, a.opts)

	var w io.Writer
	if a.singleWriter {
		w = a.w
	} else {
		w = a.writerFactory.GetWriter(level)
	}
	if w == nil {
		return
	}

	a.mu.Lock()
	n, err := w.Write(buf.b)
	a.mu.Unlock()

	var durMS float64
	if measure {
		durMS = float64(time.Since(start)) / float64(time.Millisecond)
	}
	if err != nil {
		a.loggedErrors.Add(1)
		a.opts.ErrorHandler(fmt.Errorf("failed to write log entry: %w", err))
	}
	mc.LoggedMessage(level, durMS, n, err)
}

func (a *Adapter) asyncProcessor() {
	a.wg.Add(1)
	defer a.wg.Done()
	for e := range a.asyncQueue {
		a.logDirect(e.level, e.msg, e.at, e.fields)
	}
}

func (a *Adapter) SetMinLevel(l xlog.Level) { a.opts.MinLevel = l }

// ---------------- Prefix encoders (one-time per With) ----------------

func encodeBoundText(bound []xlog.Field) []byte {
	if len(bound) == 0 {
		return nil
	}
	buf := getBuf()
	for i := range bound {
		appendTextField(buf, &bound[i]) // leading space included
	}
	cp := make([]byte, len(buf.b))
	copy(cp, buf.b)
	putBuf(buf)
	return cp
}

func encodeBoundJSON(bound []xlog.Field, opts Options) []byte {
	if len(bound) == 0 {
		return nil
	}
	buf := getBuf()
	for i := range bound {
		// NOTE: pass opts to appendJSONField so encoding matches runtime options
		appendJSONField(buf, &bound[i], opts)
	}
	cp := make([]byte, len(buf.b))
	copy(cp, buf.b)
	putBuf(buf)
	return cp
}

// ---------------- Buffer management ----------------

type buffer struct{ b []byte }

func (buf *buffer) writeString(s string) { buf.b = append(buf.b, s...) }
func (buf *buffer) writeByte(c byte)     { buf.b = append(buf.b, c) }
func (buf *buffer) writeBytes(p []byte)  { buf.b = append(buf.b, p...) }

func (buf *buffer) grow(n int) {
	free := cap(buf.b) - len(buf.b)
	if n <= free {
		return
	}
	need := len(buf.b) + n
	newCap := cap(buf.b) * 2
	if newCap < need {
		newCap = need
	}
	nb := make([]byte, len(buf.b), newCap)
	copy(nb, buf.b)
	buf.b = nb
}

var bufPool = sync.Pool{New: func() any { return &buffer{b: make([]byte, 0, 2048)} }}

func getBuf() *buffer {
	buf := bufPool.Get().(*buffer)
	buf.b = buf.b[:0]
	return buf
}
func putBuf(buf *buffer) {
	if cap(buf.b) <= 64*1024 {
		bufPool.Put(buf)
	}
}

// ---------------- Text encoding ----------------

var (
	textTsPrefix    = []byte("ts=")
	textLevelPrefix = []byte(" level=")
	textMsgPrefix   = []byte(" msg=")
	textTrue        = []byte("true")
	textFalse       = []byte("false")
	textNull        = []byte("null")
	textLenPrefix   = []byte("len:")
)

func (f *TextFormatter) FormatLogLine(buf *buffer, level xlog.Level, msg string, at time.Time, boundPrefix []byte, fields []xlog.Field, _ Options) {
	writeTextLine(buf, level, msg, at, boundPrefix, fields)
	buf.writeByte('\n')
}

func writeTextLine(buf *buffer, level xlog.Level, msg string, at time.Time, boundPrefix []byte, fields []xlog.Field) {
	buf.writeBytes(textTsPrefix)
	appendRFC3339Nano(buf, at.UTC())

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

func appendTextField(buf *buffer, f *xlog.Field) {
	buf.writeByte(' ')
	buf.writeString(f.K)
	buf.writeByte('=')
	appendTextValue(buf, f)
}

func appendTextValue(buf *buffer, f *xlog.Field) {
	switch f.Kind {
	case xlog.KindString:
		appendTextString(buf, f.Str)
	case xlog.KindInt64:
		appendInt64(buf, f.Int64)
	case xlog.KindUint64:
		appendUint64(buf, f.Uint64)
	case xlog.KindFloat64:
		appendFloat64(buf, f.Float64)
	case xlog.KindBool:
		if f.Bool {
			buf.writeBytes(textTrue)
		} else {
			buf.writeBytes(textFalse)
		}
	case xlog.KindDuration:
		appendDuration(buf, f.Dur)
	case xlog.KindTime:
		appendRFC3339Nano(buf, f.Time.UTC())
	case xlog.KindError:
		if f.Err != nil {
			appendQuoted(buf, f.Err.Error())
		} else {
			buf.writeBytes(textNull)
		}
	case xlog.KindBytes:
		buf.writeBytes(textLenPrefix)
		appendInt64(buf, int64(len(f.Bytes)))
	case xlog.KindAny:
		appendTextAny(buf, f.Any)
	default:
		buf.writeBytes(textNull)
	}
}

func appendTextString(buf *buffer, s string) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c <= 0x1F || c == ' ' || c == '"' {
			appendQuoted(buf, s)
			return
		}
	}
	buf.writeString(s)
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
		appendRFC3339Nano(buf, vv.UTC())
	case time.Duration:
		appendDuration(buf, vv)
	default:
		buf.writeString("unknown")
	}
}

// ---------------- JSON encoding ----------------

var (
	jsonTrue  = []byte("true")
	jsonFalse = []byte("false")
	jsonNull  = []byte("null")
)

func (f *JSONFormatter) FormatLogLine(buf *buffer, level xlog.Level, msg string, at time.Time, boundPrefix []byte, fields []xlog.Field, opts Options) {
	writeJSONLine(buf, level, msg, at, boundPrefix, fields, opts)
	buf.writeByte('\n')
}

func writeJSONLine(buf *buffer, level xlog.Level, msg string, at time.Time, boundPrefix []byte, fields []xlog.Field, opts Options) {
	buf.writeByte('{')

	switch opts.JSONTime {
	case JSONTimeUnixMillis:
		buf.writeString(`"ts":`)
		appendInt64(buf, at.UTC().UnixMilli())
	case JSONTimeUnixNanos:
		buf.writeString(`"ts":`)
		appendInt64(buf, at.UTC().UnixNano())
	default: // RFC3339Nano
		buf.writeString(`"ts":"`)
		appendRFC3339Nano(buf, at.UTC())
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

func appendJSONField(buf *buffer, f *xlog.Field, opts Options) {
	buf.writeByte(',')
	appendQuoted(buf, f.K)
	buf.writeByte(':')

	switch f.Kind {
	case xlog.KindString:
		appendQuoted(buf, f.Str)
	case xlog.KindInt64:
		appendInt64(buf, f.Int64)
	case xlog.KindUint64:
		appendUint64(buf, f.Uint64)
	case xlog.KindFloat64:
		if math.IsNaN(f.Float64) || math.IsInf(f.Float64, 0) {
			buf.writeBytes(jsonNull)
		} else {
			var tmp [32]byte
			b := strconv.AppendFloat(tmp[:0], f.Float64, 'g', -1, 64)
			buf.writeBytes(b)
		}
	case xlog.KindBool:
		if f.Bool {
			buf.writeBytes(jsonTrue)
		} else {
			buf.writeBytes(jsonFalse)
		}
	case xlog.KindDuration:
		switch opts.JSONDuration {
		case JSONDurationMillis:
			appendInt64(buf, int64(f.Dur/time.Millisecond))
		case JSONDurationNanos:
			appendInt64(buf, f.Dur.Nanoseconds())
		default:
			appendQuoted(buf, f.Dur.String())
		}
	case xlog.KindTime:
		// Field times follow global JSON time encoding choice for consistency.
		switch opts.JSONTime {
		case JSONTimeUnixMillis:
			appendInt64(buf, f.Time.UTC().UnixMilli())
		case JSONTimeUnixNanos:
			appendInt64(buf, f.Time.UTC().UnixNano())
		default:
			buf.writeByte('"')
			appendRFC3339Nano(buf, f.Time.UTC())
			buf.writeByte('"')
		}
	case xlog.KindError:
		if f.Err != nil {
			appendQuoted(buf, f.Err.Error())
		} else {
			buf.writeBytes(jsonNull)
		}
	case xlog.KindBytes:
		appendBase64(buf, f.Bytes)
	case xlog.KindAny:
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
				appendInt64(buf, v.UTC().UnixMilli())
			case JSONTimeUnixNanos:
				appendInt64(buf, v.UTC().UnixNano())
			default:
				buf.writeByte('"')
				appendRFC3339Nano(buf, v.UTC())
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

// ---------------- Primitive formatters ----------------

const digits = "0123456789abcdef"
const minInt64Str = "-9223372036854775808"

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

func appendQuoted(buf *buffer, s string) {
	buf.writeByte('"')
	appendQuotedContent(buf, s)
	buf.writeByte('"')
}

func appendQuotedContent(buf *buffer, s string) {
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c >= 0x20 && c != '\\' && c != '"' && c < 0x80 {
			i++
			continue
		}
		if start < i {
			buf.writeString(s[start:i])
		}
		if c < 0x80 {
			switch c {
			case '\\', '"':
				buf.writeByte('\\')
				buf.writeByte(c)
			case '\n':
				buf.writeString(`\n`)
			case '\r':
				buf.writeString(`\r`)
			case '\t':
				buf.writeString(`\t`)
			case '\b':
				buf.writeString(`\b`)
			case '\f':
				buf.writeString(`\f`)
			default:
				buf.writeString(`\u00`)
				buf.writeByte(digits[c>>4])
				buf.writeByte(digits[c&0xF])
			}
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			buf.writeString(`\uFFFD`)
			i++
			start = i
			continue
		}
		if r == '\u2028' || r == '\u2029' {
			if r == '\u2028' {
				buf.writeString(`\u2028`)
			} else {
				buf.writeString(`\u2029`)
			}
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		buf.writeString(s[start:])
	}
}
