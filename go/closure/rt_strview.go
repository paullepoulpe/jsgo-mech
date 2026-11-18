package closure

// A UTF-16 view of a string (post-flip step strview).
//
// A Java String is a sequence of UTF-16 units, and code that scans one (a lexer, a parser of a
// pattern, a loop with a lookahead) reads unit i, compares positions with length() and cuts
// between positions. The Go string holds the text as WTF-8, where String_charAt, String_length
// and String_substring find a unit by decoding the string from its start (or from a
// checkpoint) on every call. utf16_ is the string with what those calls would work out each
// time, worked out once: its length in units, whether it is all ASCII (then unit i is byte
// i), and otherwise, for a short string, its units. Every method gives what the String_
// helper of the same name gives for the string, and throws what it throws (by calling it).

import (
	"strings"
	"unsafe"
)

type utf16_ struct {
	s string
	u []uint16 // the units of s; nil when s is all ASCII; utf16Long_ for a long string
	n int32    // the length of s in units
}

// utf16Long_ stands for the units of a long string that is not ASCII (a source text) in a
// view made by utf16Of_: they are not kept, the String_ helpers find them through the
// string's index (indexOf_), as they did before there was a view. No index is inside it,
// so every method leaves its fast path for the helper.
var utf16Long_ = []uint16{}

// utf16Of_ is the view of s. It reads s once; for a long string the answer is the one kept
// with the string's index, so that a function called for every token of a text does not read
// the text again.
func utf16Of_(s string) utf16_ {
	if len(s) < 256 {
		if asciiPrefix_(s, len(s)) {
			return utf16_{s: s, n: int32(len(s))}
		}
		if s == nullstr_ { // a field's default: the String_ helpers read null as this text
			return utf16Null_
		}
		u := String_toCharArray(s)
		return utf16_{s: s, u: u, n: int32(len(u))}
	}
	c := indexOf_(s)
	if c.ascii {
		return utf16_{s: s, n: int32(len(s))}
	}
	return utf16_{s: s, u: utf16Long_, n: c.n}
}

// utf16Kept_ is the view of s that an object keeps next to its string field (and a package
// has of a constant): as utf16Of_, but with the units of a long string that is not ASCII
// too. A scanner reads every char of its source text, so the units pay for themselves; a
// view that a function makes for a few operations (utf16Of_) leaves such a text to the
// helpers' index. The view is made once per assignment of the field: a field that is
// assigned another viewed field takes its view with it.
func utf16Kept_(s string) utf16_ {
	v := utf16Of_(s)
	if len(v.u) == 0 && v.u != nil {
		v.u = String_toCharArray(s)
	}
	return v
}

var utf16Null_ = utf16_{s: nullstr_, u: String_toCharArray(nullstr_), n: String_length(nullstr_)}

// of is the view of s, which is v when v is the view of this very string (the same bytes, not
// equal ones): the view of a string that an expression gives again and again, such as o.get()
// of an Optional<String> variable, is kept in a variable next to the expression and made anew
// when the expression gives another string.
func (v *utf16_) of(s string) utf16_ {
	if len(s) != len(v.s) || unsafe.StringData(s) != unsafe.StringData(v.s) {
		*v = utf16Of_(s)
	}
	return *v
}

func (v utf16_) length() int32 { return v.n }

// charAt is inlined: for an ASCII string it is a bounds check and a byte.
func (v utf16_) charAt(i int32) uint16 {
	if v.u == nil && uint32(i) < uint32(len(v.s)) {
		return uint16(v.s[i])
	}
	return v.unitAt(i)
}

func (v utf16_) unitAt(i int32) uint16 {
	if v.u != nil && uint32(i) < uint32(len(v.u)) {
		return v.u[i]
	}
	return String_charAt(v.s, i) // throws StringIndexOutOfBoundsException
}

func (v utf16_) substring(a int32, b int32) string {
	if v.u == nil && 0 <= a && a <= b && int(b) <= len(v.s) {
		return v.s[a:b]
	}
	return String_substring(v.s, a, b)
}

func (v utf16_) substring_int(a int32) string {
	if v.u == nil && 0 <= a && int(a) <= len(v.s) {
		return v.s[a:]
	}
	return String_substring_int(v.s, a)
}

func (v utf16_) indexOf(ch int32) int32 {
	if v.u == nil && uint32(ch) < 0x80 {
		return int32(strings.IndexByte(v.s, byte(ch)))
	}
	return String_indexOf(v.s, ch)
}

func (v utf16_) indexOf_int_int(ch int32, from int32) int32 {
	if v.u == nil && uint32(ch) < 0x80 {
		return indexByteFrom_(v.s, byte(ch), from)
	}
	return String_indexOf_int_int(v.s, ch, from)
}

func (v utf16_) indexOf_String(sub string) int32 {
	if v.u == nil {
		return int32(strings.Index(v.s, sub))
	}
	return String_indexOf_String(v.s, sub)
}

func (v utf16_) indexOf_String_int(sub string, from int32) int32 {
	if v.u == nil {
		return indexFrom_(v.s, sub, from)
	}
	return String_indexOf_String_int(v.s, sub, from)
}

func (v utf16_) lastIndexOf(ch int32) int32 {
	if v.u == nil && uint32(ch) < 0x80 {
		return int32(strings.LastIndexByte(v.s, byte(ch)))
	}
	return String_lastIndexOf(v.s, ch)
}

func (v utf16_) lastIndexOf_String(sub string) int32 {
	if v.u == nil {
		return int32(strings.LastIndex(v.s, sub))
	}
	return String_lastIndexOf_String(v.s, sub)
}

func (v utf16_) startsWith_String_int(p string, off int32) bool {
	if v.u == nil {
		return hasPrefixAt_(v.s, p, off)
	}
	return String_startsWith_String_int(v.s, p, off)
}

func (v utf16_) lastIndexOf_int_int(ch int32, from int32) int32 {
	if v.u == nil && uint32(ch) < 0x80 {
		return lastIndexByteFrom_(v.s, byte(ch), from)
	}
	return String_lastIndexOf_int_int(v.s, ch, from)
}

// codePointAt is unit i of an ASCII string; in any other string the helper finds the code
// point (the units of a malformed sequence do not give it back).
func (v utf16_) codePointAt(i int32) int32 {
	if v.u == nil && uint32(i) < uint32(len(v.s)) {
		return int32(v.s[i])
	}
	return String_codePointAt(v.s, i)
}

// regionMatches compares n units at off with n units of o at ooff. With both strings ASCII
// and both regions inside them, the regions are byte slices; every other case is the
// helper's.
func (v utf16_) regionMatches(ignoreCase bool, off int32, o string, ooff int32, n int32) bool {
	if v.u == nil && off >= 0 && ooff >= 0 && n >= 0 && int(off)+int(n) <= len(v.s) && int(ooff)+int(n) <= len(o) && asciiPrefix_(o, len(o)) {
		a, b := v.s[off:off+n], o[ooff:ooff+n]
		if ignoreCase {
			return strings.EqualFold(a, b)
		}
		return a == b
	}
	return String_regionMatches(v.s, ignoreCase, off, o, ooff, n)
}

func (v utf16_) regionMatches_int_String_int_int(off int32, o string, ooff int32, n int32) bool {
	return v.regionMatches(false, off, o, ooff, n)
}
