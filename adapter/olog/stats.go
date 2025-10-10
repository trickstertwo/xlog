package olog

import "sync/atomic"

type stats struct {
	loggedErrors atomic.Uint64
	dropped      atomic.Uint64
}

// StatsSnapshot is a point-in-time counters snapshot.
type StatsSnapshot struct {
	LoggedErrors uint64
	Dropped      uint64
}

func (s *stats) snapshot() StatsSnapshot {
	return StatsSnapshot{
		LoggedErrors: s.loggedErrors.Load(),
		Dropped:      s.dropped.Load(),
	}
}

func (s *stats) reset() {
	s.loggedErrors.Store(0)
	s.dropped.Store(0)
}
