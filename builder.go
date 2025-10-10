package xlog

import "github.com/trickstertwo/xclock"

// Config for constructing a Logger (Factory data structure).
type Config struct {
	Adapter   Adapter
	MinLevel  Level
	Observers []Observer
	Clock     xclock.Clock // optional; defaults to xclock.System()
}

// Builder separates construction from representation (Builder pattern).
type Builder struct {
	cfg Config
}

func NewBuilder() *Builder {
	return &Builder{cfg: Config{MinLevel: LevelInfo}}
}

func (b *Builder) WithAdapter(a Adapter) *Builder {
	b.cfg.Adapter = a
	return b
}

func (b *Builder) WithMinLevel(l Level) *Builder {
	b.cfg.MinLevel = l
	return b
}

func (b *Builder) WithClock(c xclock.Clock) *Builder {
	b.cfg.Clock = c
	return b
}

func (b *Builder) AddObserver(o Observer) *Builder {
	b.cfg.Observers = append(b.cfg.Observers, o)
	return b
}

// Build constructs the Logger (Factory + Builder).
func (b *Builder) Build() (*Logger, error) {
	if b.cfg.Adapter == nil {
		return nil, ErrNoAdapter
	}
	// Propagate settings into the adapter when supported.
	b.applyAdapterConfig(b.cfg.Adapter)
	return newLogger(b.cfg), nil
}

// adapterLevelSetter is an optional interface adapters can implement
// to receive min-level configuration from xlog.Builder/Config.
type adapterLevelSetter interface {
	SetMinLevel(Level)
}

// applyAdapterConfig applies Config-derived settings to the adapter if it
// supports them via optional interfaces (like adapterLevelSetter).
func (b *Builder) applyAdapterConfig(a Adapter) {
	if ls, ok := a.(adapterLevelSetter); ok {
		ls.SetMinLevel(b.cfg.MinLevel)
	}
}
