package xlog

// Facade helpers using global Singleton logger.
// Usage: xlog.Info().Str("k","v").Msg("hello")

func Trace() *Event { return L().Trace() }
func Debug() *Event { return L().Debug() }
func Info() *Event  { return L().Info() }
func Warn() *Event  { return L().Warn() }
func Error() *Event { return L().Error() }
func Fatal() *Event { return L().Fatal() }
