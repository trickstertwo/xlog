package main

import (
	"time"

	"github.com/trickstertwo/xlog"
	slogadapter "github.com/trickstertwo/xlog/adapter/slog"
)

func main() {
	slogadapter.Use(slogadapter.Config{
		MinLevel: xlog.LevelInfo,
	})

	xlog.Info().
		Str("service", "payments").
		Int("port", 8080).
		Dur("boot", 125*time.Millisecond).
		Msg("listening")

}
