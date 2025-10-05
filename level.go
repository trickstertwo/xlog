package xlog

// Level mirrors slog numeric semantics and extends with Trace (-8) and Fatal (12).
type Level int

const (
	LevelTrace Level = -8
	LevelDebug Level = -4
	LevelInfo  Level = 0
	LevelWarn  Level = 4
	LevelError Level = 8
	LevelFatal Level = 12
)
