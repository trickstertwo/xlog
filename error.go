package xlog

import "errors"

var (
	ErrNoAdapter = errors.New("xlog: adapter is required (e.g., adapter/slog)")
)
