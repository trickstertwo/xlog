package olog

import "sync"

// buffer is a simple growing byte buffer with manual capacity management.
type buffer struct{ b []byte }

func (buf *buffer) writeString(s string) { buf.b = append(buf.b, s...) }
func (buf *buffer) writeByte(c byte)     { buf.b = append(buf.b, c) }
func (buf *buffer) writeBytes(p []byte)  { buf.b = append(buf.b, p...) }

func (buf *buffer) grow(n int) {
	free := cap(buf.b) - len(buf.b)
	if n <= free {
		return
	}
	need := len(buf.b) + n
	newCap := cap(buf.b) * 2
	if newCap < need {
		newCap = need
	}
	nb := make([]byte, len(buf.b), newCap)
	copy(nb, buf.b)
	buf.b = nb
}

var bufPool = sync.Pool{New: func() any { return &buffer{b: make([]byte, 0, 2048)} }}

func getBufWithCap(initCap int) *buffer {
	if initCap <= 0 {
		initCap = 2048
	}
	buf := bufPool.Get().(*buffer)
	if cap(buf.b) < initCap {
		buf.b = make([]byte, 0, initCap)
	} else {
		buf.b = buf.b[:0]
	}
	return buf
}
func getBuf() *buffer {
	buf := bufPool.Get().(*buffer)
	buf.b = buf.b[:0]
	return buf
}
func putBuf(buf *buffer) {
	if cap(buf.b) <= 64*1024 {
		bufPool.Put(buf)
	}
}
