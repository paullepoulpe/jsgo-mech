package closure

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// strBuilder_ is a java.lang.StringBuilder that is only appended to and read whole (the
// post-flip step sbuild gives it to local builders used only so): the bytes of the appended
// strings in a strings.Builder, with no UTF-16 copy and no copy at toString.
//
// StringBuilder holds UTF-16 units, and toString encodes them again, pairing surrogates. For
// strings that are valid UTF-8 (no lone surrogate, no stray byte) that is the concatenation
// of their bytes. From the first string that is not, the builder continues as a StringBuilder
// holding what was appended so far, so the result is StringBuilder's in every case.
//
// It is a value: a local variable of this type is a builder, and is never copied.
type strBuilder_ struct {
	b    strings.Builder
	wide bool           // some appended character is not ASCII
	slow *StringBuilder // the UTF-16 builder, from the first string that is not valid UTF-8
}

// toSlow continues the builder as a StringBuilder.
func (this *strBuilder_) toSlow() *StringBuilder {
	if this.slow == nil {
		this.slow = new_StringBuilder_String(this.b.String())
	}
	return this.slow
}

func (this *strBuilder_) grow(n int32) {
	if n > 0 {
		this.b.Grow(int(n))
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
	}
	this.b.WriteString(s)
	return this
}

func (this *strBuilder_) append_char(c uint16) *strBuilder_ {
	switch {
	case this.slow != nil || (c >= 0xD800 && c < 0xE000):
		this.toSlow().appendUnit(c)
	case c < 0x80:
		this.b.WriteByte(byte(c))
	default:
		this.wide = true
		this.b.WriteRune(rune(c))
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
	this.b.Write(strconv.AppendInt(buf[:0], i, 10))
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

func (this *strBuilder_) append_CharSequence_int_int(v any, a int32, b int32) *strBuilder_ {
	return this.append(String_substring(csString_(v), a, b))
}

// length is the number of UTF-16 units.
func (this *strBuilder_) length() int32 {
	switch {
	case this.slow != nil:
		return this.slow.length()
	case !this.wide:
		return int32(this.b.Len())
	}
	return String_length(this.b.String())
}

func (this *strBuilder_) isEmpty() bool {
	if this.slow != nil {
		return this.slow.isEmpty()
	}
	return this.b.Len() == 0
}

func (this *strBuilder_) toString() string {
	if this.slow != nil {
		return this.slow.toString()
	}
	return this.b.String()
}
