package zerologadapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/trickstertwo/xlog"
)

func TestZerologAdapter_JSON_EmitsTSAndFields(t *testing.T) {
	var buf bytes.Buffer
	zl := zerolog.New(&buf) // JSON by default
	a := New(zl)

	at := time.Date(2024, 12, 31, 23, 59, 59, 123456789, time.UTC)
	fields := []xlog.Field{
		{K: "from", Kind: xlog.KindString, Str: "old"},
		{K: "count", Kind: xlog.KindInt64, Int64: 2},
		{K: "ok", Kind: xlog.KindBool, Bool: true},
		{K: "dur", Kind: xlog.KindDuration, Dur: time.Millisecond},
		{K: "error", Kind: xlog.KindError, Err: errors.New("boom")},
	}
	a.Log(xlog.LevelInfo, "state changed", at, fields)

	line := buf.Bytes()
	if len(line) == 0 {
		t.Fatal("no output from zerolog")
	}

	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		t.Fatalf("json unmarshal: %v; line=%s", err, string(line))
	}

	// "level" and "message" are zerolog defaults
	if m["level"] != "info" {
		t.Fatalf("level mismatch: %v", m["level"])
	}
	if m["message"] != "state changed" {
		t.Fatalf("message mismatch: %v", m["message"])
	}

	// Our adapter-provided timestamp "ts"
	gotTS, _ := m["ts"].(string)
	wantTS := at.Format(time.RFC3339Nano)
	if gotTS != wantTS {
		t.Fatalf("ts mismatch: got %q want %q", gotTS, wantTS)
	}

	// Field checks (JSON unmarshals numbers as float64)
	if m["from"] != "old" {
		t.Fatalf("from mismatch: %v", m["from"])
	}
	if m["count"] != float64(2) {
		t.Fatalf("count mismatch: %v", m["count"])
	}
	if m["ok"] != true {
		t.Fatalf("ok mismatch: %v", m["ok"])
	}
	if m["dur"] != "1ms" {
		t.Fatalf("dur mismatch: %v", m["dur"])
	}
	if m["error"] != "boom" {
		t.Fatalf("error mismatch: %v", m["error"])
	}
}

func TestZerologAdapter_WithBoundFields(t *testing.T) {
	var buf bytes.Buffer
	zl := zerolog.New(&buf)
	a := New(zl)

	bound := []xlog.Field{
		{K: "svc", Kind: xlog.KindString, Str: "api"},
		{K: "ver", Kind: xlog.KindString, Str: "1.0.0"},
	}
	a2 := a.With(bound)

	at := time.Unix(0, 0).UTC()
	fields := []xlog.Field{
		{K: "path", Kind: xlog.KindString, Str: "/healthz"},
	}
	a2.Log(xlog.LevelInfo, "ok", at, fields)

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if m["svc"] != "api" || m["ver"] != "1.0.0" || m["path"] != "/healthz" {
		t.Fatalf("bound + event fields missing: %v", m)
	}
}
