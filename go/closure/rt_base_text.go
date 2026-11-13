package closure

// CharMatcher, Splitter, Joiner, Escaper, BaseEncoding (Guava); StringTokenizer,
// Random, Locale, AtomicIntegerArray (java.util); charset conversions of String.

import (
	"encoding/base64"
	"strings"
	"unicode/utf8"
)

// ---- CharMatcher

type CharMatcher struct {
	f    func(uint16) bool
	desc string
}

func (this *CharMatcher) matches(c uint16) bool { return this.f(c) }

func (this *CharMatcher) apply(c any) bool { return this.f(c.(uint16)) }

func (this *CharMatcher) test(c any) bool { return this.f(c.(uint16)) }

func (this *CharMatcher) toString() string { return this.desc }

func (this *CharMatcher) collapseFrom(seq any, repl uint16) string {
	u := String_toCharArray(csString_(seq))
	out := make([]uint16, 0, len(u))
	changed := false
	for i := 0; i < len(u); i++ {
		c := u[i]
		if !this.f(c) {
			out = append(out, c)
			continue
		}
		j := i + 1
		for j < len(u) && this.f(u[j]) {
			j++
		}
		if c != repl || j > i+1 {
			changed = true
		}
		out = append(out, repl)
		i = j - 1
	}
	if !changed {
		return csString_(seq)
	}
	return fromUnits_(out)
}

func (this *CharMatcher) lastIndexIn(seq any) int32 {
	u := String_toCharArray(csString_(seq))
	for i := len(u) - 1; i >= 0; i-- {
		if this.f(u[i]) {
			return int32(i)
		}
	}
	return -1
}

func (this *CharMatcher) matchesAllOf(seq any) bool {
	for _, c := range String_toCharArray(csString_(seq)) {
		if !this.f(c) {
			return false
		}
	}
	return true
}

func (this *CharMatcher) or(o *CharMatcher) *CharMatcher {
	f, g := this.f, o.f
	return &CharMatcher{func(c uint16) bool { return f(c) || g(c) }, "CharMatcher.or(" + this.desc + ", " + o.desc + ")"}
}

func (this *CharMatcher) trimFrom(seq any) string {
	s := csString_(seq)
	u := String_toCharArray(s)
	a, b := 0, len(u)
	for a < b && this.f(u[a]) {
		a++
	}
	for b > a && this.f(u[b-1]) {
		b--
	}
	if a == 0 && b == len(u) {
		return s
	}
	return fromUnits_(u[a:b])
}

func showChar_(c uint16) string {
	const hex = "0123456789ABCDEF"
	return "\\u" + string([]byte{hex[c>>12], hex[(c>>8)&15], hex[(c>>4)&15], hex[c&15]})
}

func CharMatcher_anyOf(seq any) *CharMatcher {
	u := String_toCharArray(csString_(seq))
	switch len(u) {
	case 0:
		return &CharMatcher{func(uint16) bool { return false }, "CharMatcher.none()"}
	case 1:
		return CharMatcher_is(u[0])
	}
	set := make(map[uint16]bool, len(u))
	for _, c := range u {
		set[c] = true
	}
	sorted := append([]uint16(nil), u...)
	// Two characters are shown in the given order, more are shown sorted.
	for i := 1; i < len(sorted) && len(sorted) > 2; i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	var d strings.Builder
	d.WriteString("CharMatcher.anyOf(\"")
	for _, c := range sorted {
		d.WriteString(showChar_(c))
	}
	d.WriteString("\")")
	return &CharMatcher{func(c uint16) bool { return set[c] }, d.String()}
}

func CharMatcher_inRange(a uint16, b uint16) *CharMatcher {
	if b < a {
		panic(new_IllegalArgumentException_0().self())
	}
	return &CharMatcher{func(c uint16) bool { return a <= c && c <= b }, "CharMatcher.inRange('" + showChar_(a) + "', '" + showChar_(b) + "')"}
}

func CharMatcher_is(m uint16) *CharMatcher {
	return &CharMatcher{func(c uint16) bool { return c == m }, "CharMatcher.is('" + showChar_(m) + "')"}
}

func CharMatcher_isNot(m uint16) *CharMatcher {
	return &CharMatcher{func(c uint16) bool { return c != m }, "CharMatcher.isNot('" + showChar_(m) + "')"}
}

var charMatcherLetterOrDigit_ = &CharMatcher{Character_isLetterOrDigit, "CharMatcher.javaLetterOrDigit()"}

func CharMatcher_javaLetterOrDigit() *CharMatcher { return charMatcherLetterOrDigit_ }

// guavaWhitespace_ is CharMatcher.whitespace(): the Unicode White_Space set.
func guavaWhitespace_(c uint16) bool {
	switch {
	case c >= 0x09 && c <= 0x0D, c == 0x20, c == 0x85, c == 0xA0, c == 0x1680:
		return true
	case c >= 0x2000 && c <= 0x200A, c == 0x2028, c == 0x2029, c == 0x202F, c == 0x205F, c == 0x3000:
		return true
	}
	return false
}

var charMatcherWhitespace_ = &CharMatcher{guavaWhitespace_, "CharMatcher.whitespace()"}

func CharMatcher_whitespace() *CharMatcher { return charMatcherWhitespace_ }

// ---- Splitter

type Splitter struct {
	sep_   string
	limit_ int32
	omit_  bool
	trim_  bool
}

func Splitter_on(c uint16) *Splitter { return &Splitter{sep_: String_valueOf_char(c)} }

func Splitter_on_String(sep string) *Splitter {
	if sep == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	if sep == "" {
		argFail_("The separator may not be the empty string.")
	}
	return &Splitter{sep_: sep}
}

func (this *Splitter) limit(n int32) *Splitter {
	if n <= 0 {
		argFail_("must be greater than zero: " + istr_(int64(n)))
	}
	c := *this
	c.limit_ = n
	return &c
}

func (this *Splitter) omitEmptyStrings() *Splitter {
	c := *this
	c.omit_ = true
	return &c
}

func (this *Splitter) trimResults() *Splitter {
	c := *this
	c.trim_ = true
	return &c
}

func wsRune_(r rune) bool { return r < 0x10000 && guavaWhitespace_(uint16(r)) }

func (this *Splitter) splitAll_(s string) []any {
	var out []any
	limit := this.limit_
	offset := 0
	for offset != -1 {
		start := offset
		var end int
		if p := strings.Index(s[offset:], this.sep_); p < 0 {
			end = len(s)
			offset = -1
		} else {
			end = offset + p
			offset = end + len(this.sep_)
		}
		if this.trim_ {
			for start < end {
				r, n := utf8.DecodeRuneInString(s[start:end])
				if !wsRune_(r) {
					break
				}
				start += n
			}
			for end > start {
				r, n := utf8.DecodeLastRuneInString(s[start:end])
				if !wsRune_(r) {
					break
				}
				end -= n
			}
		}
		if this.omit_ && start == end {
			continue
		}
		if limit == 1 {
			end = len(s)
			offset = -1
			if this.trim_ {
				for end > start {
					r, n := utf8.DecodeLastRuneInString(s[start:end])
					if !wsRune_(r) {
						break
					}
					end -= n
				}
			}
		} else {
			limit--
		}
		out = append(out, s[start:end])
	}
	if out == nil {
		out = []any{}
	}
	return out
}

func (this *Splitter) split(seq any) Iterable { return listOf_(this.splitAll_(csString_(seq))) }

func (this *Splitter) splitToList(seq any) List { return listOf_(this.splitAll_(csString_(seq))) }

// ---- Joiner

type Joiner struct {
	sep_ string
}

func Joiner_on(sep string) *Joiner {
	if sep == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	return &Joiner{sep}
}

func Joiner_on_char(c uint16) *Joiner { return &Joiner{String_valueOf_char(c)} }

func joinPart_(v any) string {
	switch x := v.(type) {
	case nil:
		panic(new_NullPointerException_0().self())
	case string:
		return x
	}
	return Object_toString(v)
}

func (this *Joiner) joinIter_(it Iterator) string {
	if !it.hasNext() {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(joinPart_(it.next()))
	for it.hasNext() {
		sb.WriteString(this.sep_)
		sb.WriteString(joinPart_(it.next()))
	}
	return sb.String()
}

func (this *Joiner) joinSlice_(parts []any) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return joinPart_(parts[0])
	}
	var sb strings.Builder
	for i, p := range parts {
		if i > 0 {
			sb.WriteString(this.sep_)
		}
		sb.WriteString(joinPart_(p))
	}
	return sb.String()
}

func (this *Joiner) appendTo(sb *StringBuilder, parts Iterable) *StringBuilder {
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

func (this *Joiner) join(parts Iterable) string { return this.joinIter_(parts.iterator()) }

func (this *Joiner) join_ObjectArr(parts []any) string { return this.joinSlice_(parts) }

func (this *Joiner) join_Object_Object_ObjectArr(a any, b any, rest []any) string {
	if rest == nil {
		panic(new_NullPointerException_0().self())
	}
	all := make([]any, 0, len(rest)+2)
	all = append(all, a, b)
	all = append(all, rest...)
	return this.joinSlice_(all)
}

// ---- Escaper, UnicodeEscaper, PercentEscaper

type Escaper struct {
	self_ any
}

func (this *Escaper) as_Escaper() *Escaper { return this }

func (this *Escaper) init_Escaper() {}

func (this *Escaper) escape(s string) string {
	return this.self_.(interface{ escape(string) string }).escape(s)
}

func (this *Escaper) asFunction() Function_base {
	return &Function_base_fn{func(v any) any { return strobj_(this.escape(str_(v))) }}
}

type UnicodeEscaper struct {
	Escaper
}

func (this *UnicodeEscaper) up_Escaper() *Escaper {
	if this == nil {
		return nil
	}
	return &this.Escaper
}

func (this *UnicodeEscaper) init_UnicodeEscaper() {}

type PercentEscaper struct {
	UnicodeEscaper
	safe_         [128]bool
	plusForSpace_ bool
}

func new_PercentEscaper(safeChars string, plusForSpace bool) *PercentEscaper {
	if safeChars == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	this := &PercentEscaper{plusForSpace_: plusForSpace}
	this.self_ = this
	for i := 0; i < len(safeChars); i++ {
		c := safeChars[i]
		if c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			argFail_("Alphanumeric characters are always 'safe' and should not be explicitly specified")
		}
	}
	if plusForSpace && strings.Contains(safeChars, " ") {
		argFail_("plusForSpace cannot be specified when space is a 'safe' character")
	}
	for c := 0; c < 128; c++ {
		this.safe_[c] = c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
	}
	for i := 0; i < len(safeChars); i++ {
		if c := safeChars[i]; c < 128 {
			this.safe_[c] = true
		}
	}
	return this
}

func (this *PercentEscaper) escape(s string) string {
	if s == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	i := 0
	for i < len(s) && s[i] < 128 && this.safe_[s[i]] {
		i++
	}
	if i == len(s) {
		return s
	}
	const hex = "0123456789ABCDEF"
	b := make([]byte, 0, len(s)+16)
	b = append(b, s[:i]...)
	for ; i < len(s); i++ {
		c := s[i]
		switch {
		case c < 128 && this.safe_[c]:
			b = append(b, c)
		case c == ' ' && this.plusForSpace_:
			b = append(b, '+')
		default:
			b = append(b, '%', hex[c>>4], hex[c&15])
		}
	}
	return string(b)
}

// ---- BaseEncoding

type BaseEncoding struct {
	enc *base64.Encoding
}

var base64Encoding_ = &BaseEncoding{base64.StdEncoding}

func BaseEncoding_base64() *BaseEncoding { return base64Encoding_ }

func (this *BaseEncoding) encode(b []int8) string {
	raw := make([]byte, len(b))
	for i, x := range b {
		raw[i] = byte(x)
	}
	return this.enc.EncodeToString(raw)
}

func (this *BaseEncoding) decode(seq any) []int8 {
	s := strings.TrimRight(csString_(seq), "=")
	if len(s)%4 == 1 {
		argFail_("com.google.common.io.BaseEncoding$DecodingException: Invalid input length " + istr_(int64(len(s))))
	}
	raw, err := this.enc.WithPadding(base64.NoPadding).DecodeString(s)
	if err != nil {
		bad := "?"
		if e, ok := err.(base64.CorruptInputError); ok && int(e) < len(s) {
			r, _ := utf8.DecodeRuneInString(s[int(e):])
			bad = string(r)
		}
		argFail_("com.google.common.io.BaseEncoding$DecodingException: Unrecognized character: " + bad)
	}
	out := make([]int8, len(raw))
	for i, x := range raw {
		out[i] = int8(x)
	}
	return out
}

// ---- Chars

func Chars_asList(a []uint16) List {
	items := make([]any, len(a))
	for i, c := range a {
		items[i] = c
	}
	return listOf_(items)
}

// ---- StringTokenizer

type StringTokenizer struct {
	s      string
	delims string
	pos    int
}

func new_StringTokenizer(s string, delims string) *StringTokenizer {
	if s == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	return &StringTokenizer{s: s, delims: delims}
}

func (this *StringTokenizer) skip_() {
	for this.pos < len(this.s) {
		r, n := utf8.DecodeRuneInString(this.s[this.pos:])
		if !strings.ContainsRune(this.delims, r) {
			break
		}
		this.pos += n
	}
}

func (this *StringTokenizer) hasMoreTokens() bool {
	this.skip_()
	return this.pos < len(this.s)
}

func (this *StringTokenizer) hasMoreElements() bool { return this.hasMoreTokens() }

func (this *StringTokenizer) nextToken() string {
	this.skip_()
	if this.pos >= len(this.s) {
		panic(new_NoSuchElementException_0().self())
	}
	start := this.pos
	for this.pos < len(this.s) {
		r, n := utf8.DecodeRuneInString(this.s[this.pos:])
		if strings.ContainsRune(this.delims, r) {
			break
		}
		this.pos += n
	}
	return this.s[start:this.pos]
}

// ---- Locale

type Locale struct {
	tag string
}

var Locale_ROOT = &Locale{""}

// ---- AtomicIntegerArray

type AtomicIntegerArray struct {
	a []int32
}

func new_AtomicIntegerArray(n int32) *AtomicIntegerArray {
	return &AtomicIntegerArray{make([]int32, n)}
}

func (this *AtomicIntegerArray) get(i int32) int32 { return this.a[i] }

func (this *AtomicIntegerArray) set(i int32, v int32) { this.a[i] = v }

func (this *AtomicIntegerArray) length() int32 { return int32(len(this.a)) }

func (this *AtomicIntegerArray) updateAndGet(i int32, f IntUnaryOperator) int32 {
	v := f.applyAsInt(this.a[i])
	this.a[i] = v
	return v
}

func (this *AtomicIntegerArray) toString() string {
	parts := make([]string, len(this.a))
	for i, v := range this.a {
		parts[i] = istr_(int64(v))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ---- java.util.Random (the documented linear congruential generator)

type Random struct {
	seed int64
}

// ---- String <-> bytes

func Chars_toArray(c Collection) []uint16 {
	items := c.toArray_0()
	out := make([]uint16, len(items))
	for i, v := range items {
		if v == nil {
			panic(new_NullPointerException_0().self())
		}
		out[i] = v.(uint16)
	}
	return out
}

func (this *Random) nextInt_int(bound int32) int32 {
	if bound <= 0 {
		argFail_("bound must be positive")
	}
	r := this.next_(31)
	m := bound - 1
	if bound&m == 0 {
		return int32((int64(bound) * int64(r)) >> 31)
	}
	for u := r; ; u = this.next_(31) {
		r = u % bound
		if u-r+m >= 0 {
			return r
		}
	}
}

func (this *Random) next_(bits uint) int32 {
	this.seed = (this.seed*0x5DEECE66D + 0xB) & (1<<48 - 1)
	return int32(this.seed >> (48 - bits))
}
