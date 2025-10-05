package zerologadapter

import (
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/trickstertwo/xlog"
)

// Env:
//
//	XLOG_MIN_LEVEL or XLOG_LEVEL: trace|debug|info|warn|error|fatal (fatal maps to error)
//	XLOG_CONSOLE=1          : enable ConsoleWriter (pretty output)
//	XLOG_CALLER=1                : include caller (applies to both JSON and console)
//	XLOG_CALLER_SKIP=<int>       : frames to skip (default 5)
//	XLOG_CONSOLE_TIMEFORMAT=...  : optional console time layout (default RFC3339Nano)
func init() {
	xlog.RegisterDefaultAdapterFactory(func(w io.Writer) xlog.Adapter {
		if w == nil {
			w = os.Stdout
		}
		level := envToZlLevel(firstNonEmpty(os.Getenv("XLOG_MIN_LEVEL"), os.Getenv("XLOG_LEVEL")))
		wantCaller := os.Getenv("XLOG_CALLER") == "1"
		skip := parseInt(os.Getenv("XLOG_CALLER_SKIP"), 5)

		if os.Getenv("XLOG_CONSOLE") == "1" {
			// Make ConsoleWriter use the "ts" field as the timestamp column.
			zerolog.TimestampFieldName = "ts"

			cw := zerolog.ConsoleWriter{Out: w}
			if tf := os.Getenv("XLOG_CONSOLE_TIMEFORMAT"); tf != "" {
				cw.TimeFormat = tf
			} else {
				cw.TimeFormat = time.RFC3339Nano
			}

			// If caller isn't enabled, hide the caller column to avoid "<nil>".
			if !wantCaller {
				cw.PartsExclude = append(cw.PartsExclude, zerolog.CallerFieldName)
			}

			zl := zerolog.New(cw).Level(level)

			if wantCaller {
				zerolog.CallerSkipFrameCount = skip
				zl = zl.With().Caller().Logger()
			}

			return New(zl)
		}

		// JSON mode
		zl := zerolog.New(w).Level(level)
		if wantCaller {
			zerolog.CallerSkipFrameCount = skip
			zl = zl.With().Caller().Logger()
		}
		return New(zl)
	})
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func envToZlLevel(s string) zerolog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "info", "":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
