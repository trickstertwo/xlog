package slog

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/trickstertwo/xlog"
)

func TestSlogAdapter_JSONHandler_EmitsTSAndFields(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	sl := slog.New(h)
	a := New(sl)

	at := time.Date(2024, 12, 31, 23, 59, 59, 123456789, time.UTC)
	fields := []xlog.Field{
		{K: "from", Kind: xlog.KindString, Str: "old"},
		{K: "count", Kind: xlog.KindInt64, Int64: 2},
	}
	a.Log(xlog.LevelInfo, "state changed", at, fields)

	// Parse a single JSON line
	line := buf.Bytes()
	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		t.Fatalf("json unmarshal: %v; line=%s", err, string(line))
	}

	// Verify adapter-provided timestamp "ts" equals our 'at'
	gotTS, _ := m["ts"].(string)
	wantTS := at.Format(time.RFC3339Nano)
	if gotTS != wantTS {
		t.Fatalf("ts mismatch: got %q want %q", gotTS, wantTS)
	}

	// Verify other fields exist
	if m["from"] != "old" {
		t.Fatalf("from mismatch: got %v", m["from"])
	}
	// Slog JSON handler numbers become float64 in generic map
	if m["count"] != float64(2) {
		t.Fatalf("count mismatch: got %v", m["count"])
	}
	if m["msg"] != "state changed" {
		t.Fatalf("msg mismatch: got %v", m["msg"])
	}
	// Level and time are produced by slog; we don't assert them due to variability
}
