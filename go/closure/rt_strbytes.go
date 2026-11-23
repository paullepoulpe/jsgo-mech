package closure

// String methods at byte offsets (post-flip step strbytes).
//
// The step strbytes keeps the int positions that a function computes in one of its strings as
// byte offsets of the WTF-8 string instead of UTF-16 unit indexes, where it has shown that
// every such position is one of:
//
//   - a character boundary at or before the end, which is the unit index u in Java and the
//     byte offset of unit u here (unit and byte offsets name the same place);
//   - a negative number, the same in both;
//   - past the end, length() + k in Java and len(s) + k here.
//
// The helpers below are the String methods at such offsets. Where Java's method throws (an
// offset out of range), they call the String_ helper at the unit index the offset stands for,
// so the same exception is thrown with the same message.

import "strings"

// unitIndex_ is the unit index that byte offset b of s stands for (see above).
func unitIndex_(s string, b int32) int32 {
	switch {
	case b < 0:
		return b
	case int(b) >= len(s):
		return String_length(s) + (b - int32(len(s)))
	}
	return unitsBefore_(s, int(b))
}

// byteAt_ is s.charAt(i) at byte offset i, for a char compared only with ASCII chars: the byte
// there is that char when the char is ASCII, and 0x80 or above when it is not.
func byteAt_(s string, i int32) byte {
	if uint32(i) < uint32(len(s)) {
		return s[i]
	}
	String_charAt(s, unitIndex_(s, i)) // throws StringIndexOutOfBoundsException
	panic("unreachable")
}

// cutBytes_ is s.substring(a, b) at byte offsets a and b.
func cutBytes_(s string, a int32, b int32) string {
	if 0 <= a && a <= b && int(b) <= len(s) {
		return s[a:b]
	}
	return String_substring(s, unitIndex_(s, a), unitIndex_(s, b)) // throws
}

// cutBytesFrom_ is s.substring(a) at byte offset a.
func cutBytesFrom_(s string, a int32) string {
	if 0 <= a && int(a) <= len(s) {
		return s[a:]
	}
	return String_substring_int(s, unitIndex_(s, a)) // throws
}

// indexByteFrom_ is s.indexOf(c, from) of an ASCII char c, from byte offset from on. From
// inside a character, the search finds what it finds from the end of that character: no byte
// of a longer sequence is ASCII.
func indexByteFrom_(s string, c byte, from int32) int32 {
	if from < 0 {
		from = 0
	}
	if int(from) >= len(s) {
		return -1
	}
	i := strings.IndexByte(s[from:], c)
	if i < 0 {
		return -1
	}
	return from + int32(i)
}

// indexFrom_ is s.indexOf(p, from) of an ASCII String p, as String_indexOf_String_int has it.
func indexFrom_(s string, p string, from int32) int32 {
	if from < 0 {
		from = 0
	}
	if int(from) > len(s) {
		return -1
	}
	i := strings.Index(s[from:], p)
	if i < 0 {
		return -1
	}
	return from + int32(i)
}

// lastIndexByteFrom_ is s.lastIndexOf(c, from) of an ASCII char c: the last c at or before
// byte offset from.
func lastIndexByteFrom_(s string, c byte, from int32) int32 {
	if from < 0 {
		return -1
	}
	end := int(from) + 1
	if end > len(s) {
		end = len(s)
	}
	return int32(strings.LastIndexByte(s[:end], c))
}

// lastIndexFrom_ is s.lastIndexOf(p, from) of an ASCII String p, as
// String_lastIndexOf_String_int has it.
func lastIndexFrom_(s string, p string, from int32) int32 {
	if from < 0 {
		return -1
	}
	end := int(from) + len(p)
	if end > len(s) {
		end = len(s)
	}
	return int32(strings.LastIndex(s[:end], p))
}

// hasPrefixAt_ is s.startsWith(p, off) of an ASCII String p at byte offset off.
func hasPrefixAt_(s string, p string, off int32) bool {
	return off >= 0 && int(off) <= len(s) && strings.HasPrefix(s[off:], p)
}
