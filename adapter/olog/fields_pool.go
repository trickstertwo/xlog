package xlog

import (
	"sync"

	root "github.com/trickstertwo/xlog"
)

// Pooled field slices to minimize async allocation churn.

var fieldsPool = sync.Pool{
	New: func() any { return make([]root.Field, 0, 8) },
}

// nextPow2 returns the next power-of-two >= n (caps at 1<<30).
func nextPow2(n int) int {
	if n <= 8 {
		return 8
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n++
	if n <= 0 {
		return 1 << 30
	}
	return n
}

func copyFieldsPooled(src []root.Field) ([]root.Field, bool) {
	dst := fieldsPool.Get().([]root.Field)
	if cap(dst) < len(src) {
		dst = make([]root.Field, 0, nextPow2(len(src)))
	}
	dst = dst[:len(src)]
	copy(dst, src)
	return dst, true
}

func releaseFields(s []root.Field) {
	// avoid retaining very large backing arrays
	if cap(s) > 1024 {
		return
	}
	fieldsPool.Put(s[:0])
}
