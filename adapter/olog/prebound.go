package olog

import root "github.com/trickstertwo/xlog"

// Pre-encode bound fields for both formats to avoid per-log overhead.

func encodeBoundText(bound []root.Field) []byte {
	if len(bound) == 0 {
		return nil
	}
	buf := getBuf()
	for i := range bound {
		appendTextField(buf, &bound[i]) // leading space included
	}
	cp := make([]byte, len(buf.b))
	copy(cp, buf.b)
	putBuf(buf)
	return cp
}

func encodeBoundJSON(bound []root.Field, opts Options) []byte {
	if len(bound) == 0 {
		return nil
	}
	buf := getBuf()
	for i := range bound {
		appendJSONField(buf, &bound[i], opts)
	}
	cp := make([]byte, len(buf.b))
	copy(cp, buf.b)
	putBuf(buf)
	return cp
}
