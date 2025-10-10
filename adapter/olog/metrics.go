package xlog

import root "github.com/trickstertwo/xlog"

// MetricsCollector receives write metrics. Implementations must be concurrency-safe.
type MetricsCollector interface {
	LoggedMessage(level root.Level, durMS float64, size int, err error)
}

type NoopMetricsCollector struct{}

func (*NoopMetricsCollector) LoggedMessage(level root.Level, durMS float64, size int, err error) {}
