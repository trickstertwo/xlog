# xlog

A high-performance, modular logging facade with a chainable builder API, pluggable backends (adapters), and a deterministic, swappable time source powered by [xclock](https://github.com/trickstertwo/xclock).

xlog focuses on:
- Dependability and simplicity in application code
- Performance without sacrificing safety or portability
- Clear, stable interfaces for adapters and observers
- One authoritative timestamp per event (via xclock) passed through to adapters and observers

Adapters included:
- slog (standard library structured logger)
- zerolog (github.com/rs/zerolog)
- zap (go.uber.org/zap)
- xlog (built-in, zero-dep, ultra-fast Text or JSON)

Time source:
- xclock provides fast, swappable clocks (system, frozen, jitter, offset, calibrated, etc.) with zero coordination overhead on the hot path. xlog binds to xclock for one authoritative event timestamp.

Patterns used:
- Singleton: global logger via `xlog.SetGlobal()` and `xlog.L()`
- Builder: fluent Event builder and Logger builder
- Factory: `Builder.Build()` constructs a Logger
- Adapter (Strategy): backend selection via an adapter interface
- Observer: subscribe to emitted entries

## Install

```sh
go get github.com/trickstertwo/xlog
```

## Quick start (slog)

```go
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

```

Notes:
- The slog adapter drops slog’s own `"time"` attribute and relies on xlog’s authoritative `"ts"` timestamp from xclock.
- All `Use` helpers bind the logger to `xclock.Default()` so frozen/offset/jitter/calibrated clocks are respected.

## Other adapters

### zerolog

```go
package main

import (
	"time"

	"github.com/trickstertwo/xclock/adapter/frozen"
	"github.com/trickstertwo/xlog"
	zerologadapter "github.com/trickstertwo/xlog/adapter/zerolog"
)

func main() {
	zerologadapter.Use(zerologadapter.Config{
		MinLevel:          xlog.LevelDebug,
		Console:           false,
		ConsoleTimeFormat: time.RFC3339Nano,
		Caller:            true,
		CallerSkip:        5,
		// Writer:         os.Stdout, // optional; defaults to Stdout
	})

	xlog.Debug().Str("component", "worker").Msg("started")
}
```

### zap

```go
package main

import (
	"time"

	"github.com/trickstertwo/xclock/adapter/frozen"
	"github.com/trickstertwo/xlog"
	zapadapter "github.com/trickstertwo/xlog/adapter/zap"
)

func main() {
	zapadapter.Use(zapadapter.Config{
		MinLevel: xlog.LevelDebug,
		Caller:   true, // zap.AddCaller
		// Writer, Console, EncoderConfig... available
	})

	xlog.Info().Msg("hello from zap")
}
```

## Usage (builder API)

```go
xlog.Info().
Str("from", "old").
Dur("took", 1234*time.Millisecond).
Int("count", 42).
Msg("state changed")
```

Child loggers:

```go
reqLog := xlog.L().With(
xlog.Str("request_id", "abc-123"),
xlog.Str("region", "eu-west-1"),
)
reqLog.Debug().Str("path", "/healthz").Int("status", 200).Msg("request")
```

Observers (for metrics, audits, sinks):

```go
type obs struct{}
func (obs) OnEvent(e xlog.EventData)  { /* export metrics */ }
func (obs) OnConfig(c xlog.ConfigChange) {}

logger := xlog.L()
_ = logger // already global
// attach via Builder in production for immutability;
// this example shows the concept.
```

Deterministic time in tests/demos:

```go
restore := frozen.Use(frozen.Config{
  Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
})
defer restore()

// Logger Use() helpers bind to xclock.Default(), so all timestamps use the frozen time.
```

## Why xlog? Benefits

- Single facade, many backends
    - Swap between slog, zerolog or zap without touching call sites.
- Deterministic, fast time
    - Exactly one timestamp per event via xclock, consistent across adapters and observers; freeze or step time in tests without sleeps.
- Low allocations on the hot path
    - Typed fields and pooling minimize allocations; built-in adapter uses pre-encoded bound prefixes and a single atomic write.
- Safety and predictability
    - No hidden `os.Exit`: “fatal” logs as error-level output; control termination where you call it.
    - Single buffered write per entry avoids interleaving lines across goroutines.
- Extensible by design
    - Clean Adapter, Observer, and Builder interfaces make it easy to add backends or sinks.
- Performance first
    - Fast-path timestamp, pooled buffers, hand-rolled encoders in the built-in adapter.
    - Benchmarks provided for core and adapters.

## Benchmarks

Run all benchmarks:

```sh
go test -bench=. -benchmem ./...
```

High-throughput suite (serial and parallel; xlog adapter vs zerolog vs zap):

```sh
go test -bench=HT_ -benchmem ./...
```

## Notes

- The `Use` functions of adapters set a global logger via `xlog.SetGlobal()` and bind it to `xclock.Default()`. If you change the process clock (e.g., `frozen.Use`), do that before calling adapter `Use`.
- For slog, the adapter removes slog’s default `"time"` attribute to avoid mixing multiple timestamps; xlog’s `"ts"` is authoritative.
- Fatal level logs but does not exit; prefer explicit shutdowns.

## License

Apache-2.0