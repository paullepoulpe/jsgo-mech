package closure

import (
	"strconv"
	"strings"
	"unicode/utf8"
	"unsafe"
)

// strBuilder_ is a java.lang.StringBuilder held as the bytes of the appended strings, with no
// UTF-16 copy and no copy at toString (the post-flip step sbuild gives
// it to builders whose every use it has: locals, and the parameters, fields and results they
// flow through).
//
// StringBuilder holds UTF-16 units, and toString encodes them again, pairing surrogates. For
// strings that are valid UTF-8 (no lone surrogate, no stray byte) that is the concatenation
// of their bytes. From the first string that is not, the builder continues as a StringBuilder
// holding what was appended so far, so the result is StringBuilder's in every case. It keeps
// StringBuilder's UTF-16 semantics on the fast path: length counts UTF-16 units as they are
// appended; while everything is ASCII a unit is a byte, so charAt, substring, setLength,
// setCharAt, insert and delete work on the bytes. With other text, charAt reads the last unit
// from the last character, setLength(0) empties the builder, and the other reads by index and
// changes in place continue as a StringBuilder too.
//
// toString returns the bytes as a string without a copy (as strings.Builder does); the bytes
// it returned are never written again: a change in place after it first copies them.
//
// A local variable of this type is a builder, and is never copied.
type strBuilder_ struct {
	b      []byte
	n16    int32          // the length in UTF-16 units
	wide   bool           // some appended character is not ASCII
	shared bool           // toString returned b[:len(b)]: copy before changing those bytes
	slow   *StringBuilder // the UTF-16 builder, from the first string that is not valid UTF-8
}

// str is the bytes as a string, shared with the builder.
func (this *strBuilder_) str() string {
	if len(this.b) == 0 {
		return ""
	}
	this.shared = true
	return unsafe.String(&this.b[0], len(this.b))
}

// own makes the bytes the builder's alone before a change in place.
func (this *strBuilder_) own() {
	if this.shared {
		this.b = append([]byte(nil), this.b...)
		this.shared = false
	}
}

// strBuilderCap_ is new StringBuilder(n).
func strBuilderCap_(n int32) *strBuilder_ {
	b := &strBuilder_{}
	b.grow(n)
	return b
}

// strBuilderOf_ is new StringBuilder(s).
func strBuilderOf_(s string) *strBuilder_ {
	b := &strBuilder_{}
	b.append(s)
	return b
}

// toSlow continues the builder as a StringBuilder.
func (this *strBuilder_) toSlow() *StringBuilder {
	if this.slow == nil {
		this.slow = new_StringBuilder_String(string(this.b))
	}
	return this.slow
}

func (this *strBuilder_) grow(n int32) {
	if n > 0 && cap(this.b)-len(this.b) < int(n) {
		b := make([]byte, len(this.b), len(this.b)+int(n))
		copy(b, this.b)
		this.b, this.shared = b, false
	}
}

func (this *strBuilder_) append(s string) *strBuilder_ {
	if this.slow != nil {
		this.slow.append(s)
		return this
	}
	if s == nullstr_ {
		s = "null"
	}
	if !allASCII_(s) {
		if !utf8.ValidString(s) {
			this.toSlow().append(s)
			return this
		}
		this.wide = true
		this.n16 += String_length(s)
	} else {
		this.n16 += int32(len(s))
	}
	this.b = append(this.b, s...)
	return this
}

func (this *strBuilder_) append_char(c uint16) *strBuilder_ {
	switch {
	case this.slow != nil || (c >= 0xD800 && c < 0xE000):
		this.toSlow().appendUnit(c)
	case c < 0x80:
		this.b = append(this.b, byte(c))
		this.n16++
	default:
		this.wide = true
		this.b = utf8.AppendRune(this.b, rune(c))
		this.n16++
	}
	return this
}

func (this *strBuilder_) append_charArr(u []uint16) *strBuilder_ {
	for _, c := range u {
		this.append_char(c)
	}
	return this
}

func (this *strBuilder_) append_int(i int32) *strBuilder_ { return this.append_long(int64(i)) }

func (this *strBuilder_) append_long(i int64) *strBuilder_ {
	if this.slow != nil {
		this.slow.append_long(i)
		return this
	}
	var buf [20]byte
	d := strconv.AppendInt(buf[:0], i, 10)
	this.b = append(this.b, d...)
	this.n16 += int32(len(d))
	return this
}

func (this *strBuilder_) append_boolean(b bool) *strBuilder_ { return this.append(boolstr_(b)) }

func (this *strBuilder_) append_double(d float64) *strBuilder_ {
	return this.append(Double_toString(d))
}

func (this *strBuilder_) append_Object(v any) *strBuilder_ { return this.append(objstr_(v)) }

func (this *strBuilder_) append_CharSequence(v any) *strBuilder_ {
	if v == nil {
		return this.append("null")
	}
	return this.append(csString_(v))
}

func (this *strBuilder_) append_CharSequence_s(v string) *strBuilder_ { return this.append(v) }

func (this *strBuilder_) append_CharSequence_int_int(v any, a int32, b int32) *strBuilder_ {
	return this.append(String_substring(csString_(v), a, b))
}

func (this *strBuilder_) append_CharSequence_int_int_s(v string, a int32, b int32) *strBuilder_ {
	return this.append(String_substring(csStr_(v), a, b))
}

func (this *strBuilder_) repeat(v any, n int32) *strBuilder_ {
	return this.append(strings.Repeat(csString_(v), int(n)))
}

func (this *strBuilder_) repeat_int_int(cp int32, n int32) *strBuilder_ {
	return this.append(strings.Repeat(cpString_(cp), int(n)))
}

// length is the number of UTF-16 units.
func (this *strBuilder_) length() int32 {
	if this.slow != nil {
		return this.slow.length()
	}
	return this.n16
}

func (this *strBuilder_) isEmpty() bool { return this.length() == 0 }

// charAt is the UTF-16 unit at i: a byte while everything is ASCII; the last unit from the
// last character; otherwise as a StringBuilder.
func (this *strBuilder_) charAt(i int32) uint16 {
	switch {
	case this.slow != nil:
		return this.slow.charAt(i)
	case !this.wide:
		return uint16(this.b[i])
	case i == this.n16-1:
		r, _ := utf8.DecodeLastRune(this.b)
		if r >= 0x10000 {
			return uint16(0xDC00 + (r-0x10000)&0x3FF)
		}
		return uint16(r)
	}
	return this.toSlow().charAt(i)
}

// setLength truncates or pads with '\0'. While everything is ASCII it truncates the bytes;
// with other text setLength(0) empties the builder and a shorter length continues as a
// StringBuilder.
func (this *strBuilder_) setLength(n int32) {
	switch {
	case this.slow != nil:
		this.slow.setLength(n)
	case n > this.n16:
		for this.n16 < n {
			this.append_char(0)
		}
	case n == this.n16:
	case n == 0:
		if this.shared {
			this.b, this.shared = nil, false
		}
		this.b, this.n16, this.wide = this.b[:0], 0, false
	case !this.wide:
		this.own()
		this.b, this.n16 = this.b[:n], n
	default:
		this.toSlow().setLength(n)
	}
}

func (this *strBuilder_) setCharAt(i int32, c uint16) {
	if this.slow != nil || this.wide || c >= 0x80 {
		this.toSlow().setCharAt(i, c)
		return
	}
	this.own()
	this.b[i] = byte(c)
}

func (this *strBuilder_) deleteCharAt(i int32) *strBuilder_ {
	if this.slow != nil || this.wide {
		this.toSlow().deleteCharAt(i)
		return this
	}
	this.own()
	this.b = append(this.b[:i], this.b[i+1:]...)
	this.n16--
	return this
}

func (this *strBuilder_) delete(a int32, b int32) *strBuilder_ {
	if this.slow != nil || this.wide {
		this.toSlow().delete(a, b)
		return this
	}
	if b > this.n16 {
		b = this.n16
	}
	this.own()
	this.b = append(this.b[:a], this.b[b:]...)
	this.n16 = int32(len(this.b))
	return this
}

func (this *strBuilder_) insert_int_String(i int32, s string) *strBuilder_ {
	if this.slow != nil || this.wide || !allASCII_(s) {
		this.toSlow().insert_int_String(i, s)
		return this
	}
	this.own()
	this.b = append(this.b[:i], append([]byte(s), this.b[i:]...)...)
	this.n16 = int32(len(this.b))
	return this
}

func (this *strBuilder_) insert(i int32, c uint16) *strBuilder_ {
	return this.insert_int_String(i, String_valueOf_char(c))
}

func (this *strBuilder_) substring_int(a int32) string { return this.substring(a, this.length()) }

func (this *strBuilder_) substring(a int32, b int32) string {
	switch {
	case this.slow != nil:
		return this.slow.substring(a, b)
	case !this.wide:
		return string(this.b[a:b])
	}
	return this.toSlow().substring(a, b)
}

func (this *strBuilder_) indexOf(s string) int32 { return String_indexOf_String(this.toString(), s) }

func (this *strBuilder_) lastIndexOf(s string) int32 {
	return String_lastIndexOf_String(this.toString(), s)
}

func (this *strBuilder_) toString() string {
	if this.slow != nil {
		return this.slow.toString()
	}
	return this.str()
}

// appendToStr_ is Joiner.appendTo for a strBuilder_ (the step sbuild calls it for builders it
// converts): the same appends.
func (this *Joiner) appendToStr_(sb *strBuilder_, parts Iterable) *strBuilder_ {
	if sb == nil {
		panic(new_NullPointerException_0().self())
	}
	it := parts.iterator()
	if it.hasNext() {
		sb.append(joinPart_(it.next()))
		for it.hasNext() {
			sb.append(this.sep_)
			sb.append(joinPart_(it.next()))
		}
	}
	return sb
}

// strAppendable_ is a *strBuilder_ where Java passes the StringBuilder as an Appendable
// (post-flip step sbuild): its Appendable methods append to the builder and return the same
// Appendable, as StringBuilder's return the builder; it reads as a CharSequence as the
// builder does.
type strAppendable_ struct{ b *strBuilder_ }

// appendable_ is builder b as an Appendable (nil for a nil builder, as Java's null).
func appendable_(b *strBuilder_) Appendable {
	if b == nil {
		return nil
	}
	return strAppendable_{b}
}

func (a strAppendable_) append_CharSequence(v any) Appendable {
	a.b.append_CharSequence(v)
	return a
}

func (a strAppendable_) append_CharSequence_s(v string) Appendable {
	a.b.append_CharSequence_s(v)
	return a
}

func (a strAppendable_) append_CharSequence_int_int(v any, s int32, e int32) Appendable {
	a.b.append_CharSequence_int_int(v, s, e)
	return a
}

func (a strAppendable_) append_char(c uint16) Appendable {
	a.b.append_char(c)
	return a
}

func (a strAppendable_) toString() string      { return a.b.toString() }
func (a strAppendable_) length() int32         { return a.b.length() }
func (a strAppendable_) charAt(i int32) uint16 { return a.b.charAt(i) }
