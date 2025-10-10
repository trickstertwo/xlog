package xlog

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	root "github.com/trickstertwo/xlog"
)

// Adapter is a high-throughput logger with pre-encoded bound prefixes and minimal allocs.
type Adapter struct {
	// immutable after construction
	writerFactory WriterFactory
	opts          Options
	formatter     Formatter

	// write path
	mu         *sync.Mutex
	metrics    atomic.Value // holds MetricsCollector
	wg         *sync.WaitGroup
	asyncQueue chan asyncLogEntry
	stopped    atomic.Bool
	measureDur atomic.Bool

	// counters
	st stats

	// bound fields (immutable)
	bound        []root.Field
	preBoundText []byte // ' key=value' slices
	preBoundJSON []byte // ',"key":value' slices

	// fast path for single writer
	singleWriter bool
	w            io.Writer

	// buffer tuning
	initBufCap int
}

type asyncLogEntry struct {
	level  root.Level
	msg    string
	at     time.Time
	fields []root.Field
	pooled bool
}

var errAsyncQueueFull = errors.New("xlog: async queue full, dropping log entry")

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
	if opts.AsyncPolicy == 0 {
		opts.AsyncPolicy = DropNewest
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
	if opts.BufferSize <= 0 {
		opts.BufferSize = 2048
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
		initBufCap:    opts.BufferSize,
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

// Stats returns a snapshot of internal counters.
func (a *Adapter) Stats() StatsSnapshot { return a.st.snapshot() }

// ResetStats resets internal counters.
func (a *Adapter) ResetStats() { a.st.reset() }

func (a *Adapter) Close() error {
	if a.asyncQueue != nil {
		a.stopped.Store(true)
		close(a.asyncQueue)
		a.wg.Wait()
	}
	return nil
}

// With clones the adapter and pre-encodes bound fields into immutable prefixes.
func (a *Adapter) With(fs []root.Field) root.Adapter {
	child := &Adapter{
		writerFactory: a.writerFactory,
		opts:          a.opts,
		formatter:     a.formatter,
		mu:            a.mu,
		wg:            a.wg,
		asyncQueue:    a.asyncQueue,
		singleWriter:  a.singleWriter,
		w:             a.w,
		initBufCap:    a.initBufCap,
	}
	// inherit metrics atomically
	child.metrics.Store(a.metrics.Load())
	child.measureDur.Store(a.measureDur.Load())

	// Copy existing bound
	if n := len(a.bound); n > 0 {
		child.bound = make([]root.Field, n, n+len(fs))
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
	// counters are shared across clones for global picture
	child.st = a.st
	return child
}

func (a *Adapter) Log(level root.Level, msg string, at time.Time, fields []root.Field) {
	if level < a.opts.MinLevel {
		return
	}
	if a.asyncQueue != nil && !a.stopped.Load() {
		// Copy fields using pool to safely pass to another goroutine
		c, pooled := copyFieldsPooled(fields)

		entry := asyncLogEntry{level: level, msg: msg, at: at, fields: c, pooled: pooled}
		select {
		case a.asyncQueue <- entry:
			return
		default:
			switch a.opts.AsyncPolicy {
			case DropNewest:
				a.st.dropped.Add(1)
				a.st.loggedErrors.Add(1)
				if pooled {
					releaseFields(c)
				}
				a.opts.ErrorHandler(errAsyncQueueFull)
				return
			case DropOldest:
				// Steal one from queue (non-blocking) to make room
				select {
				case ev := <-a.asyncQueue:
					// if a stolen entry used pooled fields, release them
					if ev.pooled {
						releaseFields(ev.fields)
					}
				default:
					// nothing to drop, fall through to blocking send to avoid spin
				}
				select {
				case a.asyncQueue <- entry:
					return
				default:
					// still full; drop newest
					a.st.dropped.Add(1)
					a.st.loggedErrors.Add(1)
					if pooled {
						releaseFields(c)
					}
					a.opts.ErrorHandler(errAsyncQueueFull)
					return
				}
			case Block:
				a.asyncQueue <- entry
				return
			default:
				a.asyncQueue <- entry
				return
			}
		}
	}
	a.logDirect(level, msg, at, fields)
}

func (a *Adapter) logDirect(level root.Level, msg string, at time.Time, fields []root.Field) {
	measure := a.measureDur.Load()
	mc := a.metrics.Load().(MetricsCollector)

	var start time.Time
	if measure {
		start = time.Now()
	}
	buf := getBufWithCap(a.initBufCap)
	defer putBuf(buf)

	defer func() {
		if r := recover(); r != nil {
			a.st.loggedErrors.Add(1)
			// Avoid fmt in panic path to minimize allocs
			_ = r // still produce an error via handler
			a.opts.ErrorHandler(fmt.Errorf("panic during log formatting: %v", r))
			mc.LoggedMessage(level, 0, 0, fmt.Errorf("panic during log formatting"))
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
		a.st.loggedErrors.Add(1)
		a.opts.ErrorHandler(err)
	}
	mc.LoggedMessage(level, durMS, n, err)
}

func (a *Adapter) asyncProcessor() {
	a.wg.Add(1)
	defer a.wg.Done()
	for e := range a.asyncQueue {
		a.logDirect(e.level, e.msg, e.at, e.fields)
		if e.pooled {
			releaseFields(e.fields)
		}
	}
}

func (a *Adapter) SetMinLevel(l root.Level) { a.opts.MinLevel = l }
