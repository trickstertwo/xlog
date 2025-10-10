package xlog

import "unicode/utf8"

func appendQuoted(buf *buffer, s string) {
	buf.writeByte('"')
	appendQuotedContent(buf, s)
	buf.writeByte('"')
}

func appendQuotedContent(buf *buffer, s string) {
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c >= 0x20 && c != '\\' && c != '"' && c < 0x80 {
			i++
			continue
		}
		if start < i {
			buf.writeString(s[start:i])
		}
		if c < 0x80 {
			switch c {
			case '\\', '"':
				buf.writeByte('\\')
				buf.writeByte(c)
			case '\n':
				buf.writeString(`\n`)
			case '\r':
				buf.writeString(`\r`)
			case '\t':
				buf.writeString(`\t`)
			case '\b':
				buf.writeString(`\b`)
			case '\f':
				buf.writeString(`\f`)
			default:
				buf.writeString(`\u00`)
				buf.writeByte(digits[c>>4])
				buf.writeByte(digits[c&0xF])
			}
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			buf.writeString(`\uFFFD`)
			i++
			start = i
			continue
		}
		if r == '\u2028' || r == '\u2029' {
			if r == '\u2028' {
				buf.writeString(`\u2028`)
			} else {
				buf.writeString(`\u2029`)
			}
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		buf.writeString(s[start:])
	}
}

func appendTextString(buf *buffer, s string) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c <= 0x1F || c == ' ' || c == '"' {
			appendQuoted(buf, s)
			return
		}
	}
	buf.writeString(s)
}
