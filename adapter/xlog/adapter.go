package xlogadapter

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

const (
	FormatText Format = iota + 1
	FormatJSON
)

// ErrorHandler defines how logging errors are handled
type ErrorHandler func(error)

// Options configures the adapter behavior
type Options struct {
	// Format specifies the output format (Text or JSON)
	Format Format

	// MinLevel filters logs below this level
	MinLevel xlog.Level

	// ErrorHandler receives errors that occur during logging
	ErrorHandler ErrorHandler

	// Async enables asynchronous logging (higher performance, but potential loss)
	Async bool

	// AsyncQueueSize sets buffer size for async mode
	AsyncQueueSize int

	// DisableCaller disables source code location annotations
	DisableCaller bool

	// TimeFormat specifies custom time format (empty = RFC3339Nano)
	TimeFormat string
}

// WriterFactory allows custom writers per log level
type WriterFactory interface {
	GetWriter(level xlog.Level) io.Writer
}

// DefaultWriterFactory sends all logs to the same writer
type DefaultWriterFactory struct {
	Writer io.Writer
}

func (f *DefaultWriterFactory) GetWriter(level xlog.Level) io.Writer {
	return f.Writer
}

// LevelWriterFactory sends different log levels to different writers
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

// Formatter implements a logging format strategy
type Formatter interface {
	FormatLogLine(buf *buffer, level xlog.Level, msg string, at time.Time, fields []xlog.Field)
}

// TextFormatter implements the text format strategy
type TextFormatter struct{}

// JSONFormatter implements the JSON format strategy
type JSONFormatter struct{}

// MetricsCollector observes logger events
type MetricsCollector interface {
	LoggedMessage(level xlog.Level, durMS float64, size int, err error)
}

// NoopMetricsCollector is a no-op implementation
type NoopMetricsCollector struct{}

func (c *NoopMetricsCollector) LoggedMessage(level xlog.Level, durMS float64, size int, err error) {}

// Adapter outputs log entries with efficient buffering and formatting
type Adapter struct {
	writerFactory WriterFactory
	mu            *sync.Mutex      // shared across parent/children
	bound         []xlog.Field     // fields bound to this instance
	opts          Options          // configuration options
	formatter     Formatter        // format strategy
	metrics       MetricsCollector // observability
	wg            *sync.WaitGroup  // tracks async operations
	asyncQueue    chan asyncLogEntry
	stopped       atomic.Bool
	loggedErrors  atomic.Uint64 // count of logging errors
}

type asyncLogEntry struct {
	level  xlog.Level
	msg    string
	at     time.Time
	fields []xlog.Field
}

// defaultErrorHandler writes errors to stderr
func defaultErrorHandler(err error) {
	fmt.Fprintf(os.Stderr, "xlog error: %v\n", err)
}

// New creates a new Adapter with the given writer and options
func New(w io.Writer, opts Options) *Adapter {
	return NewWithWriterFactory(&DefaultWriterFactory{Writer: w}, opts)
}

// NewWithWriterFactory creates a new Adapter with a custom writer factory
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

	var formatter Formatter
	if opts.Format == FormatJSON {
		formatter = &JSONFormatter{}
	} else {
		formatter = &TextFormatter{}
	}

	a := &Adapter{
		writerFactory: factory,
		mu:            &sync.Mutex{},
		opts:          opts,
		formatter:     formatter,
		metrics:       &NoopMetricsCollector{},
		wg:            &sync.WaitGroup{},
	}

	if opts.Async {
		queueSize := opts.AsyncQueueSize
		if queueSize <= 0 {
			queueSize = 1000
		}
		a.asyncQueue = make(chan asyncLogEntry, queueSize)
		go a.asyncProcessor()
	}

	return a
}

// SetMetricsCollector sets a custom metrics collector for observability
func (a *Adapter) SetMetricsCollector(collector MetricsCollector) {
	if collector == nil {
		collector = &NoopMetricsCollector{}
	}
	a.metrics = collector
}

// Close flushes any pending logs and releases resources
func (a *Adapter) Close() error {
	if a.asyncQueue != nil {
		a.stopped.Store(true)
		close(a.asyncQueue)
		a.wg.Wait()
	}
	return nil
}

// With returns a child adapter that inherits configuration and bound fields
func (a *Adapter) With(fs []xlog.Field) xlog.Adapter {
	child := &Adapter{
		writerFactory: a.writerFactory,
		mu:            a.mu,
		opts:          a.opts,
		formatter:     a.formatter,
		metrics:       a.metrics,
		wg:            a.wg,
		asyncQueue:    a.asyncQueue,
	}

	if n := len(a.bound); n > 0 {
		child.bound = make([]xlog.Field, n, n+len(fs))
		copy(child.bound, a.bound)
	}

	if len(fs) > 0 {
		child.bound = append(child.bound, fs...)
	}

	return child
}

// Log records a log entry with the specified level, message, time, and fields
func (a *Adapter) Log(level xlog.Level, msg string, at time.Time, fields []xlog.Field) {
	// Level filtering
	if level < a.opts.MinLevel {
		return
	}

	// Async path
	if a.asyncQueue != nil && !a.stopped.Load() {
		// Make copies of fields to avoid race conditions
		fieldsCopy := make([]xlog.Field, len(fields))
		copy(fieldsCopy, fields)

		select {
		case a.asyncQueue <- asyncLogEntry{
			level:  level,
			msg:    msg,
			at:     at,
			fields: fieldsCopy,
		}:
			// Successfully queued
		default:
			// Queue full, log an error
			a.loggedErrors.Add(1)
			a.opts.ErrorHandler(errors.New("async log queue full, dropping message"))
		}
		return
	}

	// Synchronous path
	a.logDirect(level, msg, at, fields)
}

// logDirect performs the actual logging synchronously
func (a *Adapter) logDirect(level xlog.Level, msg string, at time.Time, fields []xlog.Field) {
	start := time.Now()
	buf := getBuf()
	defer putBuf(buf)

	// Recover from panics during formatting
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic during log formatting: %v", r)
			a.loggedErrors.Add(1)
			a.opts.ErrorHandler(err)
			a.metrics.LoggedMessage(level, 0, 0, err)
		}
	}()

	// Format the log entry
	a.formatter.FormatLogLine(buf, level, msg, at, append(a.bound, fields...))

	// Get the appropriate writer for this level
	w := a.writerFactory.GetWriter(level)
	if w == nil {
		return
	}

	// Write the log entry atomically
	a.mu.Lock()
	n, err := w.Write(buf.b)
	a.mu.Unlock()

	dur := time.Since(start).Seconds() * 1000

	// Handle write errors
	if err != nil {
		a.loggedErrors.Add(1)
		a.opts.ErrorHandler(fmt.Errorf("failed to write log entry: %w", err))
	}

	// Report metrics
	a.metrics.LoggedMessage(level, dur, n, err)
}

// asyncProcessor handles asynchronous logging
func (a *Adapter) asyncProcessor() {
	a.wg.Add(1)
	defer a.wg.Done()

	for entry := range a.asyncQueue {
		a.logDirect(entry.level, entry.msg, entry.at, entry.fields)
	}
}

// FormatLogLine implements the Formatter interface for TextFormatter
func (f *TextFormatter) FormatLogLine(buf *buffer, level xlog.Level, msg string, at time.Time, fields []xlog.Field) {
	writeTextLine(buf, level, msg, at, fields)
	buf.writeByte('\n')
}

// FormatLogLine implements the Formatter interface for JSONFormatter
func (f *JSONFormatter) FormatLogLine(buf *buffer, level xlog.Level, msg string, at time.Time, fields []xlog.Field) {
	writeJSONLine(buf, level, msg, at, fields)
	buf.writeByte('\n')
}

// ------ Buffer Management ------

// buffer wraps a byte slice with efficient write operations
type buffer struct{ b []byte }

func (buf *buffer) writeString(s string) { buf.b = append(buf.b, s...) }
func (buf *buffer) writeByte(c byte)     { buf.b = append(buf.b, c) }
func (buf *buffer) writeBytes(p []byte)  { buf.b = append(buf.b, p...) }

// grow ensures capacity for n more bytes
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

var bufPool = sync.Pool{
	New: func() any { return &buffer{b: make([]byte, 0, 2048)} },
}

func getBuf() *buffer {
	buf := bufPool.Get().(*buffer)
	buf.b = buf.b[:0]
	return buf
}

func putBuf(buf *buffer) {
	// Keep pool bounded; drop extremely large buffers
	if cap(buf.b) <= 64*1024 {
		bufPool.Put(buf)
	}
}

// ------ Text Encoding ------

var (
	textTsPrefix    = []byte("ts=")
	textLevelPrefix = []byte(" level=")
	textMsgPrefix   = []byte(" msg=")
	textTrue        = []byte("true")
	textFalse       = []byte("false")
	textNull        = []byte("null")
	textLenPrefix   = []byte("len:")
)

func writeTextLine(buf *buffer, level xlog.Level, msg string, at time.Time, fields []xlog.Field) {
	buf.writeBytes(textTsPrefix)
	appendRFC3339Nano(buf, at.UTC())

	buf.writeBytes(textLevelPrefix)
	appendInt64(buf, int64(level))

	buf.writeBytes(textMsgPrefix)
	appendTextString(buf, msg)

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
		appendFloat64(buf, f.Float64) // emits "null" for NaN/Inf in JSON; text prints numeric
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
	// Quote if control, space, or double-quote is present.
	for i := 0; i < len(s); i++ {
		c := s[i]
		// Note: '\t' and '\n' are subset of c <= 0x1F; no need to check separately.
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
		// Keep text mode compact and predictable for unknown types.
		buf.writeString("unknown")
	}
}

// ------ JSON Encoding ------

var (
	jsonTrue  = []byte("true")
	jsonFalse = []byte("false")
	jsonNull  = []byte("null")
)

func writeJSONLine(buf *buffer, level xlog.Level, msg string, at time.Time, fields []xlog.Field) {
	buf.writeByte('{')

	buf.writeString(`"ts":"`)
	appendRFC3339Nano(buf, at.UTC())
	buf.writeByte('"')

	buf.writeString(`,"level":`)
	appendInt64(buf, int64(level))

	buf.writeString(`,"msg":`)
	appendQuoted(buf, msg)

	for i := range fields {
		appendJSONField(buf, &fields[i])
	}

	buf.writeByte('}')
}

func appendJSONField(buf *buffer, f *xlog.Field) {
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
		// JSON validity: NaN/Inf -> null, finite -> AppendFloat.
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
		appendQuoted(buf, f.Dur.String())
	case xlog.KindTime:
		buf.writeByte('"')
		appendRFC3339Nano(buf, f.Time.UTC())
		buf.writeByte('"')
	case xlog.KindError:
		if f.Err != nil {
			appendQuoted(buf, f.Err.Error())
		} else {
			buf.writeBytes(jsonNull)
		}
	case xlog.KindBytes:
		appendBase64(buf, f.Bytes)
	case xlog.KindAny:
		appendJSONAny(buf, f.Any)
	default:
		buf.writeBytes(jsonNull)
	}
}

func appendJSONAny(buf *buffer, v any) {
	if v == nil {
		buf.writeBytes(jsonNull)
		return
	}

	switch vv := v.(type) {
	case string:
		appendQuoted(buf, vv)
	case []byte:
		appendBase64(buf, vv)
	case bool:
		if vv {
			buf.writeBytes(jsonTrue)
		} else {
			buf.writeBytes(jsonFalse)
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
		f := float64(vv)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			buf.writeBytes(jsonNull)
		} else {
			var tmp [32]byte
			b := strconv.AppendFloat(tmp[:0], f, 'g', -1, 32)
			buf.writeBytes(b)
		}
	case float64:
		f := vv
		if math.IsNaN(f) || math.IsInf(f, 0) {
			buf.writeBytes(jsonNull)
		} else {
			var tmp [32]byte
			b := strconv.AppendFloat(tmp[:0], f, 'g', -1, 64)
			buf.writeBytes(b)
		}
	case time.Time:
		buf.writeByte('"')
		appendRFC3339Nano(buf, vv.UTC())
		buf.writeByte('"')
	case time.Duration:
		appendQuoted(buf, vv.String())
	default:
		// Fallback to json.Marshal for complex/rare values (keeps correctness).
		if data, err := json.Marshal(v); err == nil {
			buf.writeBytes(data)
		} else {
			appendQuoted(buf, "marshal_error")
		}
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
		// Handle MinInt64 without overflowing on negation.
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
	// For text mode we still need a safe, compact representation.
	if math.IsNaN(f) || math.IsInf(f, 0) {
		// Text output stays readable and explicit.
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

func appendDuration(buf *buffer, d time.Duration) {
	buf.writeString(d.String())
}

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
	buf.grow(encodedLen + 1) // +1 for the closing quote
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

// JSON-safe string quoter with a single scan and no intermediate allocations.
func appendQuotedContent(buf *buffer, s string) {
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		// Fast path for safe ASCII
		if c >= 0x20 && c != '\\' && c != '"' && c < 0x80 {
			i++
			continue
		}
		// Flush safe region
		if start < i {
			buf.writeString(s[start:i])
		}
		// ASCII escapes
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
		// UTF-8
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			buf.writeString(`\uFFFD`)
			i++
			start = i
			continue
		}
		// Escape U+2028/U+2029 for JS safety
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
		// Keep as-is, continue scan
		i += size
	}
	// Flush tail
	if start < len(s) {
		buf.writeString(s[start:])
	}
}
