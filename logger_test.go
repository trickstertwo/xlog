package xlog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/trickstertwo/xclock"
)

// stubAdapter is a minimal Adapter for tests. It records logs and can write
// a deterministic line into an optional writer for examples.
type stubAdapter struct {
	mu     sync.Mutex
	bound  []Field
	logs   []stubEntry
	writer *bytes.Buffer
}

type stubEntry struct {
	At     time.Time
	Level  Level
	Msg    string
	Fields []Field
}

func newStubAdapter(w *bytes.Buffer) *stubAdapter {
	return &stubAdapter{writer: w}
}

func (a *stubAdapter) With(fs []Field) Adapter {
	child := *a
	child.bound = append(copyFields(nil, a.bound), fs...)
	// logs must not be shared with parent
	child.logs = nil
	return &child
}

func (a *stubAdapter) Log(level Level, msg string, at time.Time, fields []Field) {
	a.mu.Lock()
	defer a.mu.Unlock()

	combined := make([]Field, 0, len(a.bound)+len(fields))
	if len(a.bound) > 0 {
		combined = append(combined, a.bound...)
	}
	if len(fields) > 0 {
		combined = append(combined, fields...)
	}
	a.logs = append(a.logs, stubEntry{
		At:     at,
		Level:  level,
		Msg:    msg,
		Fields: combined,
	})

	// Optional write for examples
	if a.writer != nil {
		// Deterministic, reflection-free formatting
		fmt.Fprintf(a.writer, "ts=%s level=%d msg=%s", at.UTC().Format(time.RFC3339Nano), int(level), msg)
		for _, f := range combined {
			switch f.Kind {
			case KindString:
				fmt.Fprintf(a.writer, " %s=%s", f.K, f.Str)
			case KindInt64:
				fmt.Fprintf(a.writer, " %s=%d", f.K, f.Int64)
			case KindUint64:
				fmt.Fprintf(a.writer, " %s=%d", f.K, f.Uint64)
			case KindBool:
				fmt.Fprintf(a.writer, " %s=%t", f.K, f.Bool)
			case KindDuration:
				fmt.Fprintf(a.writer, " %s=%s", f.K, f.Dur)
			case KindFloat64:
				b, _ := json.Marshal(f.Float64)
				fmt.Fprintf(a.writer, " %s=%s", f.K, string(b))
			case KindTime:
				fmt.Fprintf(a.writer, " %s=%s", f.K, f.Time.UTC().Format(time.RFC3339Nano))
			case KindError:
				if f.Err != nil {
					fmt.Fprintf(a.writer, " %s=%q", f.K, f.Err.Error())
				}
			case KindBytes:
				fmt.Fprintf(a.writer, " %s_len=%d", f.K, len(f.Bytes))
			case KindAny:
				fmt.Fprintf(a.writer, " %s=%T", f.K, f.Any)
			}
		}
		a.writer.WriteByte('\n')
	}
}

func TestGlobalAndFacade(t *testing.T) {
	t.Parallel()

	// Freeze time for determinism
	old := xclock.Default()
	defer xclock.SetDefault(old)
	ft := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	xclock.SetDefault(xclock.NewFrozen(ft))

	adapter := newStubAdapter(nil)
	logger, err := NewBuilder().WithAdapter(adapter).WithMinLevel(LevelDebug).Build()
	if err != nil {
		t.Fatalf("build logger: %v", err)
	}
	SetGlobal(logger)

	Info().Str("from", "old").Dur("to", time.Second).Int("count", 2).Msg("state changed")

	adapter.mu.Lock()
	defer adapter.mu.Unlock()

	if len(adapter.logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(adapter.logs))
	}
	entry := adapter.logs[0]
	if entry.Level != LevelInfo {
		t.Fatalf("level mismatch: got %v", entry.Level)
	}
	if entry.Msg != "state changed" {
		t.Fatalf("msg mismatch: %q", entry.Msg)
	}
	if !entry.At.Equal(ft) {
		t.Fatalf("timestamp mismatch: got %s want %s", entry.At, ft)
	}
	assertHasStr(t, entry.Fields, "from", "old")
	assertHasDur(t, entry.Fields, "to", time.Second)
	assertHasInt64(t, entry.Fields, "count", 2)
}

func TestMinLevelFilter(t *testing.T) {
	t.Parallel()

	adapter := newStubAdapter(nil)
	logger, err := NewBuilder().WithAdapter(adapter).WithMinLevel(LevelWarn).Build()
	if err != nil {
		t.Fatalf("build logger: %v", err)
	}

	// No global needed; use instance
	logger.Info().Msg("not emitted")

	adapter.mu.Lock()
	defer adapter.mu.Unlock()
	if got := len(adapter.logs); got != 0 {
		t.Fatalf("expected 0 logs, got %d", got)
	}
}

func TestWithAndObserverMerge(t *testing.T) {
	t.Parallel()

	// Freeze time
	old := xclock.Default()
	defer xclock.SetDefault(old)
	ft := time.Date(2030, 2, 2, 3, 4, 5, 0, time.UTC)
	xclock.SetDefault(xclock.NewFrozen(ft))

	adapter := newStubAdapter(nil)
	var got []Entry
	obs := ObserverFunc(func(e Entry) { got = append(got, e) })

	logger, err := NewBuilder().
		WithAdapter(adapter).
		WithMinLevel(LevelInfo).
		AddObserver(obs).
		Build()
	if err != nil {
		t.Fatalf("build logger: %v", err)
	}

	child := logger.With(Field{K: "request_id", Kind: KindString, Str: "r-1"})
	child.Info().Str("path", "/api").Int("status", 200).Msg("done")

	// Check observer got merged fields and timestamp
	if len(got) != 1 {
		t.Fatalf("expected 1 observer entry, got %d", len(got))
	}
	e := got[0]
	if !e.At.Equal(ft) {
		t.Fatalf("observer ts mismatch: got %s want %s", e.At, ft)
	}
	if e.Message != "done" || e.Level != LevelInfo {
		t.Fatalf("observer basic fields mismatch: %+v", e)
	}
	assertHasStr(t, e.Fields, "request_id", "r-1")
	assertHasStr(t, e.Fields, "path", "/api")
	assertHasInt64(t, e.Fields, "status", 200)
}

func assertHasStr(t *testing.T, fs []Field, k, v string) {
	t.Helper()
	for _, f := range fs {
		if f.K == k && f.Kind == KindString && f.Str == v {
			return
		}
	}
	t.Fatalf("missing string field %q=%q in %+v", k, v, fs)
}

func assertHasInt64(t *testing.T, fs []Field, k string, v int64) {
	t.Helper()
	for _, f := range fs {
		if f.K == k && f.Kind == KindInt64 && f.Int64 == v {
			return
		}
	}
	t.Fatalf("missing int64 field %q=%d in %+v", k, v, fs)
}

func assertHasDur(t *testing.T, fs []Field, k string, v time.Duration) {
	t.Helper()
	for _, f := range fs {
		if f.K == k && f.Kind == KindDuration && f.Dur == v {
			return
		}
	}
	t.Fatalf("missing duration field %q=%s in %+v", k, v, fs)
}
