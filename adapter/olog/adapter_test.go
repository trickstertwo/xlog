package olog

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/trickstertwo/xlog"
)

func TestTextLine_FieldsAndNewline(t *testing.T) {
	var buf bytes.Buffer
	a := New(&buf, Options{Format: FormatText})

	at := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fields := []xlog.Field{
		{K: "from", Kind: xlog.KindString, Str: "old"},
		{K: "count", Kind: xlog.KindInt64, Int64: 2},
		{K: "ok", Kind: xlog.KindBool, Bool: true},
		{K: "dur", Kind: xlog.KindDuration, Dur: time.Millisecond},
	}
	a.Log(xlog.LevelInfo, "state changed", at, fields)

	out := buf.String()
	if !strings.Contains(out, "ts=2025-01-01T00:00:00Z") {
		t.Fatalf("missing ts: %s", out)
	}
	if !strings.Contains(out, "level=0") || !strings.Contains(out, "msg=state changed") {
		t.Fatalf("missing level/msg: %s", out)
	}
	for _, want := range []string{" from=old", " count=2", " ok=true", " dur=1ms"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing field %q in %s", want, out)
		}
	}
	if !strings.HasSuffix(out, "\n") {
		t.Fatalf("missing newline: %q", out)
	}
}

func TestJSONLine_ObjectAndFields(t *testing.T) {
	var buf bytes.Buffer
	a := New(&buf, Options{Format: FormatJSON})

	at := time.Date(2024, 12, 31, 23, 59, 59, 123456789, time.UTC)
	fields := []xlog.Field{
		{K: "from", Kind: xlog.KindString, Str: "old"},
		{K: "count", Kind: xlog.KindInt64, Int64: 2},
		{K: "ok", Kind: xlog.KindBool, Bool: true},
		{K: "dur", Kind: xlog.KindDuration, Dur: time.Millisecond},
	}
	a.Log(xlog.LevelInfo, "state changed", at, fields)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if m["msg"] != "state changed" {
		t.Fatalf("msg mismatch: %v", m["msg"])
	}
	gotTS, _ := m["ts"].(string)
	wantTS := at.Format(time.RFC3339Nano)
	if gotTS != wantTS {
		t.Fatalf("ts mismatch: got %q want %q", gotTS, wantTS)
	}
	if m["level"] != float64(0) {
		t.Fatalf("level mismatch: %v", m["level"])
	}
	if m["from"] != "old" || m["count"] != float64(2) || m["ok"] != true || m["dur"] != "1ms" {
		t.Fatalf("field mismatch: %v", m)
	}
}
