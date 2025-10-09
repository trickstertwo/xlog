package zerolog

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/trickstertwo/xlog"
)

// Config is an explicit, code-first configuration for zerolog + xlog.
// No envs, no hidden init, one call to Use.
type Config struct {
	Writer             io.Writer // default: os.Stdout
	MinLevel           xlog.Level
	Console            bool   // pretty console output instead of JSON
	ConsoleTimeFormat  string // only used if Console==true; default time.RFC3339Nano
	Caller             bool   // include caller in logs
	CallerSkip         int    // frames to skip when resolving caller; default 5
	TimestampFieldName string // default "ts" (aligns with xlog's authoritative timestamp)
}

// Use builds a zerolog-backed xlog logger from Config,
// wires it as the global xlog logger, and returns it.
func Use(cfg Config) *xlog.Logger {
	w := cfg.Writer
	if w == nil {
		w = os.Stdout
	}
	if cfg.TimestampFieldName == "" {
		cfg.TimestampFieldName = "ts"
	}
	if cfg.Caller && cfg.CallerSkip <= 0 {
		cfg.CallerSkip = 5
	}

	// Build zerolog.Logger according to Config
	var zl zerolog.Logger
	if cfg.Console {
		// Align console’s leading timestamp column with our authoritative "ts"
		zerolog.TimestampFieldName = cfg.TimestampFieldName
		cw := zerolog.ConsoleWriter{Out: w}
		if cfg.ConsoleTimeFormat == "" {
			cw.TimeFormat = time.RFC3339Nano
		} else {
			cw.TimeFormat = cfg.ConsoleTimeFormat
		}
		zl = zerolog.New(cw)
	} else {
		zl = zerolog.New(w)
	}

	// Level
	zl = zl.Level(mapLevel(cfg.MinLevel))

	// Caller
	if cfg.Caller {
		zerolog.CallerSkipFrameCount = cfg.CallerSkip
		zl = zl.With().Caller().Logger()
	}

	// Wrap in adapter and use as global xlog logger
	ad := New(zl)
	// Propagate min level down to zerolog (optional interface)
	ad.SetMinLevel(cfg.MinLevel)

	return xlog.UseAdapter(ad, cfg.MinLevel)
}

// Optional: env-friendly entrypoint if you still want it (explicitly opted-in).
// Keep env usage out of main by default to reduce hidden magic.
type EnvKeys struct {
	MinLevelEnv          string // e.g. "XLOG_MIN_LEVEL" or "XLOG_LEVEL"
	ConsoleEnv           string // e.g. "XLOG_CONSOLE"
	CallerEnv            string // e.g. "XLOG_CALLER"
	CallerSkipEnv        string // e.g. "XLOG_CALLER_SKIP"
	ConsoleTimeFormatEnv string // e.g. "XLOG_CONSOLE_TIMEFORMAT"
}

// UseFromEnv is a convenience wrapper for teams that still prefer env wiring.
// Not used by the single-call example unless you explicitly opt in.
func UseFromEnv(keys EnvKeys) *xlog.Logger {
	// Lightweight parsers; defaults chosen to be safe.
	lvl := parseMinLevel(getEnvFirst(keys.MinLevelEnv, "info"))
	console := os.Getenv(keys.ConsoleEnv) == "1"
	caller := os.Getenv(keys.CallerEnv) == "1"
	callerSkip := parseInt(os.Getenv(keys.CallerSkipEnv), 5)
	tf := os.Getenv(keys.ConsoleTimeFormatEnv)

	return Use(Config{
		Writer:            os.Stdout,
		MinLevel:          lvl,
		Console:           console,
		ConsoleTimeFormat: tf,
		Caller:            caller,
		CallerSkip:        callerSkip,
	})
}

func getEnvFirst(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseMinLevel(s string) xlog.Level {
	switch s {
	case "trace", "TRACE", "Trace":
		return xlog.LevelTrace
	case "debug", "DEBUG", "Debug":
		return xlog.LevelDebug
	case "info", "INFO", "Info", "":
		return xlog.LevelInfo
	case "warn", "warning", "WARN", "Warning":
		return xlog.LevelWarn
	case "error", "ERROR", "Error":
		return xlog.LevelError
	case "fatal", "FATAL", "Fatal":
		return xlog.LevelFatal
	default:
		return xlog.LevelInfo
	}
}

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	n := 0
	sign := 1
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 0 && c == '-' {
			sign = -1
			continue
		}
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	return n * sign
}
