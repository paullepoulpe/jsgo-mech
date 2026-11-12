package closure

// java.lang.String on Go strings.
//
// A Java String is a sequence of UTF-16 code units; here it is a Go string
// holding UTF-8 (a lone surrogate is kept as its three-byte encoding, "WTF-8").
// Every index that Java code sees counts UTF-16 units, so index-based
// operations translate between unit indexes and byte offsets. For a string
// that is all ASCII the two are equal, which is the fast path.

import (
	"strconv"
	"strings"
	"sync/atomic"
	"unicode"
	"unicode/utf8"
)

func isASCII_(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// decode_ reads one code point (or lone surrogate) at byte offset i.
func decode_(s string, i int) (cp int32, size int) {
	b0 := s[i]
	switch {
	case b0 < 0x80:
		return int32(b0), 1
	case b0 >= 0xC0 && b0 < 0xE0 && i+1 < len(s):
		return int32(b0&0x1F)<<6 | int32(s[i+1]&0x3F), 2
	case b0 >= 0xE0 && b0 < 0xF0 && i+2 < len(s):
		return int32(b0&0x0F)<<12 | int32(s[i+1]&0x3F)<<6 | int32(s[i+2]&0x3F), 3
	case b0 >= 0xF0 && i+3 < len(s):
		return int32(b0&0x07)<<18 | int32(s[i+1]&0x3F)<<12 | int32(s[i+2]&0x3F)<<6 | int32(s[i+3]&0x3F), 4
	}
	return 0xFFFD, 1
}

func appendCP_(b []byte, cp int32) []byte {
	switch {
	case cp < 0x80:
		return append(b, byte(cp))
	case cp < 0x800:
		return append(b, 0xC0|byte(cp>>6), 0x80|byte(cp&0x3F))
	case cp < 0x10000:
		return append(b, 0xE0|byte(cp>>12), 0x80|byte((cp>>6)&0x3F), 0x80|byte(cp&0x3F))
	}
	return append(b, 0xF0|byte(cp>>18), 0x80|byte((cp>>12)&0x3F), 0x80|byte((cp>>6)&0x3F), 0x80|byte(cp&0x3F))
}

// u16index_ maps UTF-16 unit indexes of one long string to byte offsets.
type u16index_ struct {
	s     string
	ascii bool
	n     int32   // length in UTF-16 units
	every []int32 // byte offset of unit 64*k (never inside a 4-byte character's second half)
	adj   []int8  // 1 if unit 64*k is the low half of the character at every[k]
}

const indexStride_ = 64

var indexCache_ [8]atomic.Pointer[u16index_]
var indexNext_ atomic.Uint32

func indexOf_(s string) *u16index_ {
	for i := range indexCache_ {
		if c := indexCache_[i].Load(); c != nil && len(c.s) == len(s) && c.s == s {
			return c
		}
	}
	c := &u16index_{s: s, ascii: isASCII_(s)}
	if c.ascii {
		c.n = int32(len(s))
	} else {
		var n int32
		for i := 0; i < len(s); {
			cp, size := decode_(s, i)
			units := int32(1)
			if cp >= 0x10000 {
				units = 2
			}
			for u := int32(0); u < units; u++ {
				if (n+u)%indexStride_ == 0 {
					c.every = append(c.every, int32(i))
					c.adj = append(c.adj, int8(u))
				}
			}
			n += units
			i += size
		}
		c.n = n
	}
	indexCache_[indexNext_.Add(1)%uint32(len(indexCache_))].Store(c)
	return c
}

// locate_ finds the byte offset of UTF-16 unit i; low reports that i is the
// low-surrogate half of the 4-byte character at that offset.
func locate_(s string, i int32) (off int, low bool) {
	if i < 0 {
		panic(new_StringIndexOutOfBoundsException_String("index " + istr_(int64(i))).self())
	}
	start, unit := 0, int32(0)
	if len(s) >= 256 {
		c := indexOf_(s)
		if c.ascii {
			return int(i), false
		}
		k := int(i / indexStride_)
		if k >= len(c.every) {
			k = len(c.every) - 1
		}
		if k >= 0 {
			start, unit = int(c.every[k]), int32(k)*indexStride_-int32(c.adj[k])
		}
	}
	for off = start; off < len(s); {
		cp, size := decode_(s, off)
		if unit == i {
			return off, false
		}
		if cp >= 0x10000 {
			if unit+1 == i {
				return off, true
			}
			unit += 2
		} else {
			unit++
		}
		off += size
	}
	if unit == i {
		return len(s), false
	}
	panic(new_StringIndexOutOfBoundsException_String("index " + istr_(int64(i)) + ", length " + istr_(int64(unit))).self())
}

// unitsBefore_ is the UTF-16 index of byte offset off.
func unitsBefore_(s string, off int) int32 {
	var n int32
	for i := 0; i < off; {
		cp, size := decode_(s, i)
		if cp >= 0x10000 {
			n += 2
		} else {
			n++
		}
		i += size
	}
	return n
}

func asciiPrefix_(s string, n int) bool {
	if n > len(s) {
		return false
	}
	for i := 0; i < n; i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func shortASCII_(s string) bool { return len(s) < 256 && isASCII_(s) }

func longASCII_(s string) bool { return len(s) >= 256 && indexOf_(s).ascii }

func allASCII_(s string) bool {
	if len(s) < 256 {
		return isASCII_(s)
	}
	return indexOf_(s).ascii
}

func String_length(s string) int32 {
	if len(s) < 256 {
		n := 0
		for i := 0; i < len(s); i++ {
			if s[i] >= 0x80 {
				return unitsBefore_(s, len(s))
			}
			n++
		}
		return int32(n)
	}
	return indexOf_(s).n
}

func String_isEmpty(s string) bool { return len(s) == 0 }

func String_charAt(s string, i int32) uint16 {
	if len(s) < 256 {
		if i >= 0 && int(i) < len(s) && asciiPrefix_(s, int(i)+1) {
			return uint16(s[i])
		}
	} else if c := indexOf_(s); c.ascii {
		if i < 0 || int(i) >= len(s) {
			panic(new_StringIndexOutOfBoundsException_String("index " + istr_(int64(i)) + ", length " + istr_(int64(len(s)))).self())
		}
		return uint16(s[i])
	}
	off, low := locate_(s, i)
	if off >= len(s) {
		panic(new_StringIndexOutOfBoundsException_String("index " + istr_(int64(i))).self())
	}
	cp, _ := decode_(s, off)
	if cp >= 0x10000 {
		cp -= 0x10000
		if low {
			return uint16(0xDC00 + cp&0x3FF)
		}
		return uint16(0xD800 + cp>>10)
	}
	return uint16(cp)
}

func String_codePointAt(s string, i int32) int32 {
	off, low := locate_(s, i)
	cp, size := decode_(s, off)
	if low {
		return 0xDC00 + (cp-0x10000)&0x3FF
	}
	// A high surrogate followed by a low one, kept as two 3-byte sequences.
	if cp >= 0xD800 && cp < 0xDC00 && off+size < len(s) {
		if lo, _ := decode_(s, off+size); lo >= 0xDC00 && lo < 0xE000 {
			return 0x10000 + (cp-0xD800)<<10 + (lo - 0xDC00)
		}
	}
	return cp
}

func surrogateBytes_(u uint16) string {
	return string([]byte{0xE0 | byte(u>>12), 0x80 | byte((u>>6)&0x3F), 0x80 | byte(u&0x3F)})
}

func String_substring(s string, a int32, b int32) string {
	if allASCII_(s) {
		if a < 0 || b > int32(len(s)) || a > b {
			panic(new_StringIndexOutOfBoundsException_String("begin " + istr_(int64(a)) + ", end " + istr_(int64(b)) + ", length " + istr_(int64(len(s)))).self())
		}
		return s[a:b]
	}
	if a > b {
		panic(new_StringIndexOutOfBoundsException_String("begin " + istr_(int64(a)) + ", end " + istr_(int64(b))).self())
	}
	oa, la := locate_(s, a)
	ob, lb := locate_(s, b)
	if !la && !lb {
		return s[oa:ob]
	}
	// A boundary splits a surrogate pair: the halves become lone surrogates.
	var sb strings.Builder
	if la {
		cp, size := decode_(s, oa)
		if a == b {
			return ""
		}
		sb.WriteString(surrogateBytes_(uint16(0xDC00 + (cp-0x10000)&0x3FF)))
		oa += size
	}
	if oa < ob {
		sb.WriteString(s[oa:ob])
	}
	if lb && ob >= oa {
		cp, _ := decode_(s, ob)
		sb.WriteString(surrogateBytes_(uint16(0xD800 + (cp-0x10000)>>10)))
	}
	return sb.String()
}

func String_substring_int(s string, a int32) string {
	if allASCII_(s) {
		if a < 0 || a > int32(len(s)) {
			panic(new_StringIndexOutOfBoundsException_String("begin " + istr_(int64(a)) + ", length " + istr_(int64(len(s)))).self())
		}
		return s[a:]
	}
	return String_substring(s, a, String_length(s))
}

func String_toCharArray(s string) []uint16 {
	out := make([]uint16, 0, len(s))
	for i := 0; i < len(s); {
		if s[i] < 0x80 {
			out = append(out, uint16(s[i]))
			i++
			continue
		}
		cp, size := decode_(s, i)
		if cp >= 0x10000 {
			cp -= 0x10000
			out = append(out, uint16(0xD800+cp>>10), uint16(0xDC00+cp&0x3FF))
		} else {
			out = append(out, uint16(cp))
		}
		i += size
	}
	return out
}

// fromUnits_ builds a string from UTF-16 units, pairing surrogates.
func fromUnits_(u []uint16) string {
	b := make([]byte, 0, len(u))
	for i := 0; i < len(u); i++ {
		c := int32(u[i])
		if c < 0x80 {
			b = append(b, byte(c))
			continue
		}
		if c >= 0xD800 && c < 0xDC00 && i+1 < len(u) && u[i+1] >= 0xDC00 && u[i+1] < 0xE000 {
			c = 0x10000 + (c-0xD800)<<10 + (int32(u[i+1]) - 0xDC00)
			i++
		}
		b = appendCP_(b, c)
	}
	return string(b)
}

func new_String(u []uint16) string { return fromUnits_(u) }

func new_String_charArr_int_int(u []uint16, off int32, n int32) string {
	return fromUnits_(u[off : off+n])
}

func new_String_intArr_int_int(cps []int32, off int32, n int32) string {
	var b []byte
	for _, cp := range cps[off : off+n] {
		b = appendCP_(b, cp)
	}
	return string(b)
}

func String_valueOf_charArr_int_int(u []uint16, off int32, n int32) string {
	return fromUnits_(u[off : off+n])
}

func String_valueOf_char(c uint16) string {
	if c < 0x80 {
		return asciiStrings_[c]
	}
	return string(appendCP_(nil, int32(c)))
}

var asciiStrings_ = func() (t [128]string) {
	for i := range t {
		t[i] = string(rune(i))
	}
	return
}()

func Character_toString(c uint16) string { return String_valueOf_char(c) }

func String_valueOf(i int32) string          { return strconv.FormatInt(int64(i), 10) }
func String_valueOf_long(i int64) string     { return strconv.FormatInt(i, 10) }
func String_valueOf_double(d float64) string { return Double_toString(d) }
func String_valueOf_Object(v any) string     { return objstr_(v) }
func String_toString(s string) string        { return s }

func String_hashCode(s string) int32 {
	var h int32
	for i := 0; i < len(s); {
		if s[i] < 0x80 {
			h = 31*h + int32(s[i])
			i++
			continue
		}
		cp, size := decode_(s, i)
		if cp >= 0x10000 {
			cp -= 0x10000
			h = 31*h + (0xD800 + cp>>10)
			h = 31*h + (0xDC00 + cp&0x3FF)
		} else {
			h = 31*h + cp
		}
		i += size
	}
	return h
}

func String_compareTo(a string, b string) int32 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] && a[i] < 0x80 {
		i++
	}
	if i == n && (i == len(a) || a[i] < 0x80) && (i == len(b) || b[i] < 0x80) {
		if allASCII_(a[i:]) && allASCII_(b[i:]) {
			return int32(len(a) - len(b))
		}
	}
	if i < n && a[i] < 0x80 && b[i] < 0x80 {
		return int32(a[i]) - int32(b[i])
	}
	// Non-ASCII from here on: compare UTF-16 units.
	ua, ub := String_toCharArray(a[i:]), String_toCharArray(b[i:])
	for k := 0; k < len(ua) && k < len(ub); k++ {
		if ua[k] != ub[k] {
			return int32(ua[k]) - int32(ub[k])
		}
	}
	return int32(len(ua) - len(ub))
}

func String_equalsIgnoreCase(a string, b string) bool {
	if b == nullstr_ {
		return false
	}
	return a == b || strings.EqualFold(a, b)
}

func String_startsWith(s string, p string) bool { return strings.HasPrefix(s, p) }
func String_endsWith(s string, p string) bool   { return strings.HasSuffix(s, p) }

func String_startsWith_String_int(s string, p string, off int32) bool {
	if off < 0 || off > String_length(s) {
		return false
	}
	o, low := locate_(s, off)
	return !low && strings.HasPrefix(s[o:], p)
}

func csString_(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case interface{ toString() string }:
		return x.toString()
	}
	panic(new_NullPointerException_0().self())
}

func String_contains(s string, sub any) bool { return strings.Contains(s, csString_(sub)) }

// cpString_ is the string of one code point given as an int (char or supplementary).
func cpString_(cp int32) string {
	if cp < 0x80 && cp >= 0 {
		return asciiStrings_[cp]
	}
	return string(appendCP_(nil, cp))
}

func String_indexOf(s string, ch int32) int32 { return String_indexOf_int_int(s, ch, 0) }

func String_indexOf_int_int(s string, ch int32, from int32) int32 {
	return String_indexOf_String_int(s, cpString_(ch), from)
}

func String_indexOf_String(s string, sub string) int32 {
	i := strings.Index(s, sub)
	if i < 0 || allASCII_(s) {
		return int32(i)
	}
	return unitsBefore_(s, i)
}

func String_indexOf_String_int(s string, sub string, from int32) int32 {
	if from < 0 {
		from = 0
	}
	if allASCII_(s) {
		if int(from) > len(s) {
			return -1
		}
		i := strings.Index(s[from:], sub)
		if i < 0 {
			return -1
		}
		return from + int32(i)
	}
	if from > String_length(s) {
		return -1
	}
	off, low := locate_(s, from)
	if low {
		_, size := decode_(s, off)
		off += size
	}
	i := strings.Index(s[off:], sub)
	if i < 0 {
		return -1
	}
	return unitsBefore_(s, off+i)
}

func String_lastIndexOf(s string, ch int32) int32 { return String_lastIndexOf_String(s, cpString_(ch)) }

func String_lastIndexOf_int_int(s string, ch int32, from int32) int32 {
	return String_lastIndexOf_String_int(s, cpString_(ch), from)
}

func String_lastIndexOf_String(s string, sub string) int32 {
	i := strings.LastIndex(s, sub)
	if i < 0 || allASCII_(s) {
		return int32(i)
	}
	return unitsBefore_(s, i)
}

func String_lastIndexOf_String_int(s string, sub string, from int32) int32 {
	if from < 0 {
		return -1
	}
	n := String_length(s)
	end := from + String_length(sub)
	if end > n {
		end = n
	}
	return String_lastIndexOf_String(String_substring(s, 0, end), sub)
}

func String_trim(s string) string {
	a, b := 0, len(s)
	for a < b && s[a] <= ' ' {
		a++
	}
	for b > a && s[b-1] <= ' ' {
		b--
	}
	return s[a:b]
}

func String_toLowerCase(s string, l *Locale) string {
	if allASCII_(s) {
		for i := 0; i < len(s); i++ {
			if s[i] >= 'A' && s[i] <= 'Z' {
				return strings.ToLower(s)
			}
		}
		return s
	}
	return strings.ToLower(s)
}

func String_toUpperCase(s string, l *Locale) string {
	if allASCII_(s) {
		for i := 0; i < len(s); i++ {
			if s[i] >= 'a' && s[i] <= 'z' {
				return strings.ToUpper(s)
			}
		}
		return s
	}
	return strings.ToUpper(s)
}

func String_repeat(s string, n int32) string { return strings.Repeat(s, int(n)) }

func String_replace(s string, a any, b any) string {
	return strings.ReplaceAll(s, csString_(a), csString_(b))
}

func String_replace_char_char(s string, a uint16, b uint16) string {
	return strings.ReplaceAll(s, String_valueOf_char(a), String_valueOf_char(b))
}

func String_regionMatches_int_String_int_int(s string, off int32, o string, ooff int32, n int32) bool {
	return String_regionMatches(s, false, off, o, ooff, n)
}

func String_regionMatches(s string, ignoreCase bool, off int32, o string, ooff int32, n int32) bool {
	if off < 0 || ooff < 0 || off+n > String_length(s) || ooff+n > String_length(o) {
		return false
	}
	a, b := String_substring(s, off, off+n), String_substring(o, ooff, ooff+n)
	if ignoreCase {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func String_join(sep any, it Iterable) string {
	var parts []string
	for i := it.iterator(); i.hasNext(); {
		parts = append(parts, objstr_(i.next()))
	}
	return strings.Join(parts, csString_(sep))
}

func String_join_CharSequence_CharSequenceArr(sep any, vs []any) string {
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = objstr_(v)
	}
	return strings.Join(parts, csString_(sep))
}

// ---- CharSequence

func CharSequence_length(v any) int32 {
	switch x := v.(type) {
	case string:
		return String_length(x)
	case interface{ length() int32 }:
		return x.length()
	}
	panic(new_NullPointerException_0().self())
}

func CharSequence_charAt(v any, i int32) uint16 {
	switch x := v.(type) {
	case string:
		return String_charAt(x, i)
	case interface{ charAt(int32) uint16 }:
		return x.charAt(i)
	}
	panic(new_NullPointerException_0().self())
}

func CharSequence_toString(v any) string { return csString_(v) }

func CharSequence_subSequence(v any, a int32, b int32) any {
	return String_substring(csString_(v), a, b)
}

// ---- java.lang.StringBuilder

// StringBuilder keeps bytes while everything appended is ASCII and switches to
// UTF-16 units at the first non-ASCII character.
type StringBuilder struct {
	b    []byte
	w    []uint16
	wide bool
}

func new_StringBuilder() *StringBuilder { return &StringBuilder{} }

func new_StringBuilder_int(n int32) *StringBuilder {
	return &StringBuilder{b: make([]byte, 0, n)}
}

func new_StringBuilder_String(s string) *StringBuilder {
	return new_StringBuilder().append(s)
}

func (this *StringBuilder) widen() {
	if this.wide {
		return
	}
	this.w = make([]uint16, len(this.b), 2*len(this.b)+16)
	for i, c := range this.b {
		this.w[i] = uint16(c)
	}
	this.b = nil
	this.wide = true
}

func (this *StringBuilder) append(s string) *StringBuilder {
	if s == nullstr_ {
		s = "null"
	}
	if !this.wide {
		if allASCII_(s) {
			this.b = append(this.b, s...)
			return this
		}
		this.widen()
	}
	for i := 0; i < len(s); {
		if s[i] < 0x80 {
			this.w = append(this.w, uint16(s[i]))
			i++
			continue
		}
		cp, size := decode_(s, i)
		if cp >= 0x10000 {
			cp -= 0x10000
			this.w = append(this.w, uint16(0xD800+cp>>10), uint16(0xDC00+cp&0x3FF))
		} else {
			this.w = append(this.w, uint16(cp))
		}
		i += size
	}
	return this
}

func (this *StringBuilder) appendUnit(c uint16) {
	if !this.wide {
		if c < 0x80 {
			this.b = append(this.b, byte(c))
			return
		}
		this.widen()
	}
	this.w = append(this.w, c)
}

func (this *StringBuilder) append_char(c uint16) Appendable {
	this.appendUnit(c)
	return this
}

func (this *StringBuilder) append_CharSequence(v any) Appendable {
	if v == nil {
		return this.append("null")
	}
	return this.append(csString_(v))
}

func (this *StringBuilder) append_CharSequence_int_int(v any, a int32, b int32) Appendable {
	return this.append(String_substring(csString_(v), a, b))
}

func (this *StringBuilder) append_Object(v any) *StringBuilder { return this.append(objstr_(v)) }

func (this *StringBuilder) append_charArr(u []uint16) *StringBuilder {
	for _, c := range u {
		this.appendUnit(c)
	}
	return this
}

func (this *StringBuilder) append_int(i int32) *StringBuilder {
	if !this.wide {
		this.b = strconv.AppendInt(this.b, int64(i), 10)
		return this
	}
	return this.append(strconv.FormatInt(int64(i), 10))
}

func (this *StringBuilder) append_long(i int64) *StringBuilder {
	return this.append(strconv.FormatInt(i, 10))
}

func (this *StringBuilder) append_double(d float64) *StringBuilder {
	return this.append(Double_toString(d))
}

func (this *StringBuilder) append_boolean(b bool) *StringBuilder { return this.append(boolstr_(b)) }

func (this *StringBuilder) appendCodePoint(cp int32) *StringBuilder {
	return this.append(cpString_(cp))
}

func (this *StringBuilder) length() int32 {
	if this.wide {
		return int32(len(this.w))
	}
	return int32(len(this.b))
}

func (this *StringBuilder) charAt(i int32) uint16 {
	if this.wide {
		return this.w[i]
	}
	return uint16(this.b[i])
}

func (this *StringBuilder) setCharAt(i int32, c uint16) {
	if !this.wide {
		if c < 0x80 {
			this.b[i] = byte(c)
			return
		}
		this.widen()
	}
	this.w[i] = c
}

func (this *StringBuilder) setLength(n int32) {
	if this.wide {
		for int32(len(this.w)) < n {
			this.w = append(this.w, 0)
		}
		this.w = this.w[:n]
		return
	}
	for int32(len(this.b)) < n {
		this.b = append(this.b, 0)
	}
	this.b = this.b[:n]
}

func (this *StringBuilder) deleteCharAt(i int32) *StringBuilder {
	if this.wide {
		this.w = append(this.w[:i], this.w[i+1:]...)
	} else {
		this.b = append(this.b[:i], this.b[i+1:]...)
	}
	return this
}

func (this *StringBuilder) delete(a int32, b int32) *StringBuilder {
	if b > this.length() {
		b = this.length()
	}
	if this.wide {
		this.w = append(this.w[:a], this.w[b:]...)
	} else {
		this.b = append(this.b[:a], this.b[b:]...)
	}
	return this
}

func (this *StringBuilder) insert_int_String(i int32, s string) *StringBuilder {
	if !this.wide && allASCII_(s) {
		this.b = append(this.b[:i], append([]byte(s), this.b[i:]...)...)
		return this
	}
	this.widen()
	u := String_toCharArray(s)
	this.w = append(this.w[:i], append(u, this.w[i:]...)...)
	return this
}

func (this *StringBuilder) insert(i int32, c uint16) *StringBuilder {
	return this.insert_int_String(i, String_valueOf_char(c))
}

func (this *StringBuilder) substring_int_int(a int32, b int32) string {
	if this.wide {
		return fromUnits_(this.w[a:b])
	}
	return string(this.b[a:b])
}

func (this *StringBuilder) substring(a int32) string {
	return this.substring_int_int(a, this.length())
}

func (this *StringBuilder) indexOf(s string) int32 { return String_indexOf_String(this.toString(), s) }

func (this *StringBuilder) lastIndexOf(s string) int32 {
	return String_lastIndexOf_String(this.toString(), s)
}

func (this *StringBuilder) repeat(v any, n int32) *StringBuilder {
	return this.append(strings.Repeat(csString_(v), int(n)))
}

func (this *StringBuilder) repeat_int_int(cp int32, n int32) *StringBuilder {
	return this.append(strings.Repeat(cpString_(cp), int(n)))
}

func (this *StringBuilder) reverse() *StringBuilder {
	this.widen()
	for i, j := 0, len(this.w)-1; i < j; i, j = i+1, j-1 {
		this.w[i], this.w[j] = this.w[j], this.w[i]
	}
	return this
}

func (this *StringBuilder) isEmpty() bool { return this.length() == 0 }

func (this *StringBuilder) toString() string {
	if this.wide {
		return fromUnits_(this.w)
	}
	return string(this.b)
}

func (this *StringBuilder) compareTo(o any) int32 {
	return String_compareTo(this.toString(), o.(*StringBuilder).toString())
}

// ---- java.lang.Character

func Character_charValue(v any) uint16 { return v.(uint16) }

func Character_valueOf(c uint16) any { return c }

func Character_isSupplementaryCodePoint(cp int32) bool { return cp >= 0x10000 && cp <= 0x10FFFF }

func Character_toChars(cp int32) []uint16 {
	if cp >= 0x10000 {
		cp -= 0x10000
		return []uint16{uint16(0xD800 + cp>>10), uint16(0xDC00 + cp&0x3FF)}
	}
	return []uint16{uint16(cp)}
}

func Character_isDigit(c uint16) bool {
	if c < 0x80 {
		return c >= '0' && c <= '9'
	}
	return unicode.Is(unicode.Nd, rune(c))
}

func Character_isLetter(c uint16) bool {
	if c < 0x80 {
		return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
	}
	return unicode.IsLetter(rune(c))
}

func Character_isLetterOrDigit(c uint16) bool { return Character_isLetter(c) || Character_isDigit(c) }

func Character_isUpperCase(c uint16) bool { return unicode.IsUpper(rune(c)) }
func Character_isLowerCase(c uint16) bool { return unicode.IsLower(rune(c)) }

func Character_toUpperCase(c uint16) uint16 {
	r := unicode.ToUpper(rune(c))
	if r > 0xFFFF {
		return c
	}
	return uint16(r)
}

func Character_toLowerCase(c uint16) uint16 {
	r := unicode.ToLower(rune(c))
	if r > 0xFFFF {
		return c
	}
	return uint16(r)
}

func Character_isWhitespace(cp int32) bool {
	switch cp {
	case '\t', '\n', 0x0B, '\f', '\r', 0x1C, 0x1D, 0x1E, 0x1F:
		return true
	case 0xA0, 0x2007, 0x202F:
		return false
	}
	return unicode.In(rune(cp), unicode.Zs, unicode.Zl, unicode.Zp)
}

func Character_isIdentifierIgnorable(c uint16) bool {
	return c <= 8 || c >= 0x0E && c <= 0x1B || c >= 0x7F && c <= 0x9F || unicode.Is(unicode.Cf, rune(c))
}

func Character_isJavaIdentifierStart(c uint16) bool {
	if c < 0x80 {
		return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c == '$'
	}
	r := rune(c)
	return unicode.IsLetter(r) || unicode.In(r, unicode.Nl, unicode.Sc, unicode.Pc)
}

func Character_isJavaIdentifierPart(c uint16) bool {
	if c < 0x80 {
		return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_' || c == '$' || c >= '0' && c <= '9' || Character_isIdentifierIgnorable(c)
	}
	r := rune(c)
	return unicode.IsLetter(r) || unicode.In(r, unicode.Nl, unicode.Sc, unicode.Pc, unicode.Nd, unicode.Mc, unicode.Mn) || Character_isIdentifierIgnorable(c)
}

func Character_digit(c uint16, radix int32) int32 {
	var d int32 = -1
	switch {
	case c >= '0' && c <= '9':
		d = int32(c - '0')
	case c >= 'a' && c <= 'z':
		d = int32(c-'a') + 10
	case c >= 'A' && c <= 'Z':
		d = int32(c-'A') + 10
	case c >= 0x80 && unicode.Is(unicode.Nd, rune(c)):
		d = int32(digitValue_(rune(c)))
	}
	if d >= radix {
		return -1
	}
	return d
}

func digitValue_(r rune) int {
	// Decimal digits come in runs of ten code points starting at a zero.
	for z := r; z > r-10; z-- {
		if !unicode.Is(unicode.Nd, z-1) || (z-1 < r-9) {
			return int(r - z)
		}
	}
	return 0
}

var javaCategories_ = []struct {
	t    *unicode.RangeTable
	code int32
}{
	{unicode.Lu, 1}, {unicode.Ll, 2}, {unicode.Lt, 3}, {unicode.Lm, 4}, {unicode.Lo, 5}, {unicode.Mn, 6}, {unicode.Me, 7},
	{unicode.Mc, 8}, {unicode.Nd, 9}, {unicode.Nl, 10}, {unicode.No, 11}, {unicode.Zs, 12}, {unicode.Zl, 13}, {unicode.Zp, 14},
	{unicode.Cc, 15}, {unicode.Cf, 16}, {unicode.Co, 18}, {unicode.Cs, 19}, {unicode.Pd, 20}, {unicode.Ps, 21}, {unicode.Pe, 22},
	{unicode.Pc, 23}, {unicode.Po, 24}, {unicode.Sm, 25}, {unicode.Sc, 26}, {unicode.Sk, 27}, {unicode.So, 28}, {unicode.Pi, 29},
	{unicode.Pf, 30},
}

func Character_getType(cp int32) int32 {
	for _, c := range javaCategories_ {
		if unicode.Is(c.t, rune(cp)) {
			return c.code
		}
	}
	return 0
}

func Character_getType_char(c uint16) int32 { return Character_getType(int32(c)) }

var _ = utf8.RuneError
