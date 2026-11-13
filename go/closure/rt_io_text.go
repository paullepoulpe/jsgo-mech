package closure

// java.nio.charset and the character streams of java.io (Reader, Writer and
// their subclasses).
//
// A Java String is a Go string holding UTF-8 in which a lone surrogate is kept
// as its three-byte encoding (see rt_string.go). Decoding bytes produces that
// representation; encoding pairs up surrogates that were stored separately and
// replaces unpaired ones, as the JDK's encoders do.

import (
	"strings"
	"unicode/utf8"
	"unsafe"
)

// ---- byte slices

// u8_ views a Java byte[] as Go bytes (no copy).
func u8_(b []int8) []byte {
	if len(b) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&b[0])), len(b))
}

// i8_ views Go bytes as a Java byte[] (no copy).
func i8_(b []byte) []int8 {
	if len(b) == 0 {
		return []int8{}
	}
	return unsafe.Slice((*int8)(unsafe.Pointer(&b[0])), len(b))
}

// strBytes_ views a string as bytes that must not be modified.
func strBytes_(s string) []byte {
	if len(s) == 0 {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// ---- java.nio.charset.Charset

const (
	csUTF8_ = iota
	csASCII_
	csLatin1_
	csUTF16_
	csUTF16BE_
	csUTF16LE_
	csCP1252_
)

type Charset struct {
	name_    string
	kind_    int
	aliases_ []string
}

var StandardCharsets_UTF_8 = &Charset{name_: "UTF-8", kind_: csUTF8_, aliases_: []string{"UTF8", "unicode-1-1-utf-8"}}
var StandardCharsets_US_ASCII = &Charset{name_: "US-ASCII", kind_: csASCII_, aliases_: []string{
	"ASCII", "ISO646-US", "ANSI_X3.4-1968", "ANSI_X3.4-1986", "ISO_646.irv:1991", "646", "iso-ir-6", "us", "cp367", "IBM367", "csASCII", "default", "ascii7", "ISO_646.irv:1983"}}
var StandardCharsets_ISO_8859_1 = &Charset{name_: "ISO-8859-1", kind_: csLatin1_, aliases_: []string{
	"ISO8859_1", "ISO_8859_1", "ISO8859-1", "8859_1", "latin1", "l1", "ISO_8859-1", "ISO_8859-1:1987", "iso-ir-100", "IBM819", "IBM-819", "cp819", "819", "csISOLatin1"}}
var StandardCharsets_UTF_16 = &Charset{name_: "UTF-16", kind_: csUTF16_, aliases_: []string{"UTF_16", "utf16", "unicode", "UnicodeBig"}}
var StandardCharsets_UTF_16BE = &Charset{name_: "UTF-16BE", kind_: csUTF16BE_, aliases_: []string{"UTF_16BE", "ISO-10646-UCS-2", "X-UTF-16BE", "UnicodeBigUnmarked"}}
var StandardCharsets_UTF_16LE = &Charset{name_: "UTF-16LE", kind_: csUTF16LE_, aliases_: []string{"UTF_16LE", "X-UTF-16LE", "UnicodeLittleUnmarked"}}
var charsetCP1252_ = &Charset{name_: "windows-1252", kind_: csCP1252_, aliases_: []string{"cp1252", "cp5348", "ibm-1252", "ibm1252"}}

var charsets_ = func() map[string]*Charset {
	m := map[string]*Charset{}
	for _, cs := range []*Charset{StandardCharsets_UTF_8, StandardCharsets_US_ASCII, StandardCharsets_ISO_8859_1,
		StandardCharsets_UTF_16, StandardCharsets_UTF_16BE, StandardCharsets_UTF_16LE, charsetCP1252_} {
		m[strings.ToLower(cs.name_)] = cs
		for _, a := range cs.aliases_ {
			m[strings.ToLower(a)] = cs
		}
	}
	return m
}()

type StandardCharsets struct{}

// UnsupportedCharsetException is java.nio.charset.UnsupportedCharsetException;
// its message is the name of the charset.
type UnsupportedCharsetException struct{ IllegalArgumentException }

func Charset_forName(name string) *Charset {
	if name == nullstr_ {
		panic(new_IllegalArgumentException_String("Null charset name").self())
	}
	if cs := charsets_[strings.ToLower(name)]; cs != nil {
		return cs
	}
	e := &UnsupportedCharsetException{}
	e.self_ = e
	e.init_IllegalArgumentException_String(name)
	panic(e.self())
}

func Charset_isSupported(name string) bool {
	if name == nullstr_ {
		panic(new_IllegalArgumentException_String("Null charset name").self())
	}
	return charsets_[strings.ToLower(name)] != nil
}

func Charset_defaultCharset() *Charset { return StandardCharsets_UTF_8 }

func (this *Charset) name() string        { return this.name_ }
func (this *Charset) displayName() string { return this.name_ }
func (this *Charset) toString() string    { return this.name_ }
func (this *Charset) canEncode() bool     { return true }
func (this *Charset) hashCode() int32     { return String_hashCode(this.name_) }

func (this *Charset) equals(o any) bool {
	c, ok := o.(*Charset)
	return ok && c != nil && c.name_ == this.name_
}

func (this *Charset) compareTo(o any) int32 {
	a, b := strings.ToLower(this.name_), strings.ToLower(o.(*Charset).name_)
	return int32(strings.Compare(a, b))
}

func (this *Charset) newEncoder() *CharsetEncoder {
	return &CharsetEncoder{cs_: this, malformed_: CodingErrorAction_REPORT, unmappable_: CodingErrorAction_REPORT}
}

// ---- CodingErrorAction, CharsetEncoder

type CodingErrorAction struct {
	name_ string
}

var CodingErrorAction_IGNORE = &CodingErrorAction{"IGNORE"}
var CodingErrorAction_REPLACE = &CodingErrorAction{"REPLACE"}
var CodingErrorAction_REPORT = &CodingErrorAction{"REPORT"}

func (this *CodingErrorAction) toString() string { return this.name_ }

type CharsetEncoder struct {
	cs_         *Charset
	malformed_  *CodingErrorAction
	unmappable_ *CodingErrorAction
}

func (this *CharsetEncoder) charset() *Charset { return this.cs_ }

func (this *CharsetEncoder) onMalformedInput(a *CodingErrorAction) *CharsetEncoder {
	if a == nil {
		panic(new_IllegalArgumentException_String("Null action").self())
	}
	this.malformed_ = a
	return this
}

func (this *CharsetEncoder) onUnmappableCharacter(a *CodingErrorAction) *CharsetEncoder {
	if a == nil {
		panic(new_IllegalArgumentException_String("Null action").self())
	}
	this.unmappable_ = a
	return this
}

// canEncode says whether the single char can be encoded; a surrogate on its
// own never can.
func (this *CharsetEncoder) canEncode(c uint16) bool {
	if c >= 0xD800 && c < 0xE000 {
		return false
	}
	return this.cs_.canEncodeCP_(int32(c))
}

func (cs *Charset) canEncodeCP_(cp int32) bool {
	switch cs.kind_ {
	case csASCII_:
		return cp < 0x80
	case csLatin1_:
		return cp < 0x100
	case csCP1252_:
		_, ok := cp1252Byte_(cp)
		return ok
	}
	return true
}

// cp1252High_ are the characters of windows-1252 bytes 0x80..0x9F (0 where the
// byte is unassigned).
var cp1252High_ = [32]uint16{
	0x20AC, 0, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0, 0x017D, 0,
	0, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0, 0x017E, 0x0178,
}

func cp1252Byte_(cp int32) (byte, bool) {
	if cp < 0x80 || cp >= 0xA0 && cp < 0x100 {
		return byte(cp), true
	}
	for i, u := range cp1252High_ {
		if u != 0 && int32(u) == cp {
			return byte(0x80 + i), true
		}
	}
	return 0, false
}

// ---- encoding

const (
	encReplace_ = iota
	encReport_
	encIgnore_
)

// encoder_ turns strings into bytes of one charset. A high surrogate at the
// end of one string is held back so that it can pair with a low surrogate at
// the start of the next, as a Java stream encoder does.
type encoder_ struct {
	cs_      *Charset
	mode_    int
	pending_ int32 // a held-back high surrogate, or 0
	bom_     bool  // UTF-16: the byte order mark has been written
}

func newEncoder_(cs *Charset) *encoder_ {
	if cs == nil {
		panic(new_NullPointerException_String("charset").self())
	}
	return &encoder_{cs_: cs}
}

func encoderOf_(e *CharsetEncoder) *encoder_ {
	enc := newEncoder_(e.cs_)
	switch {
	case e.unmappable_ == CodingErrorAction_REPORT || e.malformed_ == CodingErrorAction_REPORT:
		enc.mode_ = encReport_
	case e.unmappable_ == CodingErrorAction_IGNORE:
		enc.mode_ = encIgnore_
	}
	return enc
}

func isHigh_(cp int32) bool { return cp >= 0xD800 && cp < 0xDC00 }
func isLow_(cp int32) bool  { return cp >= 0xDC00 && cp < 0xE000 }

// plain_ reports whether s can be written as it is: UTF-8 without any
// surrogate stored as a three-byte sequence.
func (e *encoder_) plain_(s string) bool {
	if e.pending_ != 0 {
		return false
	}
	switch e.cs_.kind_ {
	case csUTF8_:
		return strings.IndexByte(s, 0xED) < 0
	case csASCII_, csLatin1_, csCP1252_:
		return isASCII_(s)
	}
	return false
}

// encode_ appends the encoding of s to dst.
func (e *encoder_) encode_(dst []byte, s string) []byte {
	if e.plain_(s) {
		return append(dst, s...)
	}
	for i := 0; i < len(s); {
		var cp int32
		if s[i] < 0x80 {
			cp = int32(s[i])
			i++
		} else {
			var size int
			cp, size = decode_(s, i)
			i += size
		}
		if e.pending_ != 0 {
			hi := e.pending_
			e.pending_ = 0
			if isLow_(cp) {
				dst = e.put_(dst, 0x10000+(hi-0xD800)<<10+(cp-0xDC00))
				continue
			}
			dst = e.bad_(dst, true)
		}
		switch {
		case isHigh_(cp):
			e.pending_ = cp
		case isLow_(cp):
			dst = e.bad_(dst, true)
		default:
			dst = e.put_(dst, cp)
		}
	}
	return dst
}

// finish_ appends what the end of input produces: the replacement for a high
// surrogate that never got its pair.
func (e *encoder_) finish_(dst []byte) []byte {
	if e.pending_ != 0 {
		e.pending_ = 0
		dst = e.bad_(dst, true)
	}
	return dst
}

func (e *encoder_) put16_(dst []byte, u uint16) []byte {
	if e.cs_.kind_ == csUTF16LE_ {
		return append(dst, byte(u), byte(u>>8))
	}
	return append(dst, byte(u>>8), byte(u))
}

func (e *encoder_) put_(dst []byte, cp int32) []byte {
	switch e.cs_.kind_ {
	case csUTF8_:
		return appendCP_(dst, cp)
	case csUTF16_, csUTF16BE_, csUTF16LE_:
		if e.cs_.kind_ == csUTF16_ && !e.bom_ {
			e.bom_ = true
			dst = append(dst, 0xFE, 0xFF)
		}
		if cp >= 0x10000 {
			cp -= 0x10000
			dst = e.put16_(dst, uint16(0xD800+cp>>10))
			return e.put16_(dst, uint16(0xDC00+cp&0x3FF))
		}
		return e.put16_(dst, uint16(cp))
	case csCP1252_:
		if b, ok := cp1252Byte_(cp); ok {
			return append(dst, b)
		}
	default:
		if e.cs_.canEncodeCP_(cp) {
			return append(dst, byte(cp))
		}
	}
	return e.bad_(dst, false)
}

// bad_ handles a character that cannot be encoded, or (malformed) an
// unpaired surrogate.
func (e *encoder_) bad_(dst []byte, malformed bool) []byte {
	switch e.mode_ {
	case encReport_:
		if malformed {
			panic(new_MalformedInputException_(1).self())
		}
		panic(new_UnmappableCharacterException_(1).self())
	case encIgnore_:
		return dst
	}
	switch e.cs_.kind_ {
	case csUTF16_, csUTF16BE_, csUTF16LE_:
		return e.put_(dst, 0xFFFD)
	}
	return append(dst, '?')
}

func String_getBytes(s string, cs *Charset) []int8 {
	if s == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	e := newEncoder_(cs)
	return i8_(e.finish_(e.encode_(make([]byte, 0, len(s)), s)))
}

// ---- decoding

// decodeUTF8_ decodes as the JDK does: every malformed sequence (its maximal
// valid prefix) becomes one U+FFFD, and so does an encoded surrogate.
func decodeUTF8_(b []byte) (s string, malformed bool) {
	if utf8.Valid(b) {
		return string(b), false
	}
	out := make([]byte, 0, len(b)+16)
	for i := 0; i < len(b); {
		c := b[i]
		if c < 0x80 {
			out = append(out, c)
			i++
			continue
		}
		need := 0
		switch {
		case c >= 0xC2 && c <= 0xDF:
			need = 1
		case c >= 0xE0 && c <= 0xEF:
			need = 2
		case c >= 0xF0 && c <= 0xF4:
			need = 3
		}
		lo, hi := byte(0x80), byte(0xBF)
		switch c {
		case 0xE0:
			lo = 0xA0
		case 0xF0:
			lo = 0x90
		case 0xF4:
			hi = 0x8F
		}
		n, ok := 1, need > 0
		for k := 0; ok && k < need; k++ {
			if i+n >= len(b) {
				ok = false
				break
			}
			x := b[i+n]
			l, h := byte(0x80), byte(0xBF)
			if k == 0 {
				l, h = lo, hi
			}
			if x < l || x > h {
				ok = false
				break
			}
			n++
		}
		if ok && c == 0xED && b[i+1] >= 0xA0 {
			ok = false // a surrogate: the whole sequence is one error
		}
		if !ok {
			out = append(out, 0xEF, 0xBF, 0xBD)
			malformed = true
		} else {
			out = append(out, b[i:i+n]...)
		}
		i += n
	}
	return string(out), malformed
}

func decodeUTF16_(b []byte, kind int) (string, bool) {
	little := kind == csUTF16LE_
	if kind == csUTF16_ && len(b) >= 2 {
		if b[0] == 0xFE && b[1] == 0xFF {
			b = b[2:]
		} else if b[0] == 0xFF && b[1] == 0xFE {
			b = b[2:]
			little = true
		}
	}
	malformed := false
	units := make([]uint16, 0, len(b)/2+1)
	for i := 0; i+1 < len(b); i += 2 {
		if little {
			units = append(units, uint16(b[i])|uint16(b[i+1])<<8)
		} else {
			units = append(units, uint16(b[i])<<8|uint16(b[i+1]))
		}
	}
	// Unpaired surrogates are malformed input.
	for i := 0; i < len(units); i++ {
		u := int32(units[i])
		if isHigh_(u) && i+1 < len(units) && isLow_(int32(units[i+1])) {
			i++
		} else if isHigh_(u) || isLow_(u) {
			units[i] = 0xFFFD
			malformed = true
		}
	}
	if len(b)%2 == 1 {
		units = append(units, 0xFFFD)
		malformed = true
	}
	return fromUnits_(units), malformed
}

// decode_ decodes bytes to a string, replacing malformed input with U+FFFD;
// the second result says whether anything was replaced.
func (cs *Charset) decode_(b []byte) (string, bool) {
	switch cs.kind_ {
	case csUTF8_:
		return decodeUTF8_(b)
	case csUTF16_, csUTF16BE_, csUTF16LE_:
		return decodeUTF16_(b, cs.kind_)
	}
	ascii := true
	for _, c := range b {
		if c >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return string(b), false
	}
	malformed := false
	out := make([]byte, 0, len(b)+len(b)/2)
	for _, c := range b {
		cp := int32(c)
		if c >= 0x80 {
			switch cs.kind_ {
			case csASCII_:
				cp = 0xFFFD
				malformed = true
			case csCP1252_:
				if c < 0xA0 {
					cp = int32(cp1252High_[c-0x80])
					if cp == 0 {
						cp = 0xFFFD
						malformed = true
					}
				}
			}
		}
		out = appendCP_(out, cp)
	}
	return string(out), malformed
}

// decodeStrict_ decodes like a decoder that reports errors: malformed input
// throws MalformedInputException (a CharacterCodingException).
func (cs *Charset) decodeStrict_(b []byte) string {
	s, malformed := cs.decode_(b)
	if malformed {
		panic(new_MalformedInputException_(1).self())
	}
	return s
}

func new_String_byteArr_Charset(b []int8, cs *Charset) string {
	if cs == nil {
		panic(new_NullPointerException_String("charset").self())
	}
	s, _ := cs.decode_(u8_(b))
	return s
}

// ---- CharacterCodingException

type CharacterCodingException struct {
	IOException
}

func (this *CharacterCodingException) as_CharacterCodingException() *CharacterCodingException {
	return this
}

func to_CharacterCodingException(v any) *CharacterCodingException {
	if v == nil {
		return nil
	}
	return v.(interface {
		as_CharacterCodingException() *CharacterCodingException
	}).as_CharacterCodingException()
}

func is_CharacterCodingException(v any) bool {
	_, ok := v.(interface {
		as_CharacterCodingException() *CharacterCodingException
	})
	return ok
}

func (this *CharacterCodingException) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *CharacterCodingException) up_IOException() *IOException {
	if this == nil {
		return nil
	}
	return &this.IOException
}

func (this *CharacterCodingException) init_CharacterCodingException() {
	this.IOException.init_IOException_0()
}

func new_CharacterCodingException() *CharacterCodingException {
	this := &CharacterCodingException{}
	this.self_ = this
	this.init_CharacterCodingException()
	return this
}

// MalformedInputException is java.nio.charset.MalformedInputException.
type MalformedInputException struct {
	CharacterCodingException
	inputLength_ int32
}

func new_MalformedInputException_(n int32) *MalformedInputException {
	this := &MalformedInputException{inputLength_: n}
	this.self_ = this
	this.init_CharacterCodingException()
	return this
}

func (this *MalformedInputException) getInputLength() int32 { return this.inputLength_ }

func (this *MalformedInputException) getMessage() string {
	return "Input length = " + istr_(int64(this.inputLength_))
}

// UnmappableCharacterException is java.nio.charset.UnmappableCharacterException.
type UnmappableCharacterException struct {
	CharacterCodingException
	inputLength_ int32
}

func new_UnmappableCharacterException_(n int32) *UnmappableCharacterException {
	this := &UnmappableCharacterException{inputLength_: n}
	this.self_ = this
	this.init_CharacterCodingException()
	return this
}

func (this *UnmappableCharacterException) getInputLength() int32 { return this.inputLength_ }

func (this *UnmappableCharacterException) getMessage() string {
	return "Input length = " + istr_(int64(this.inputLength_))
}

// ---- java.io.Reader
//
// Every reader here works on the decoded text as one string, which it obtains
// from its source the first time it is read.

type Reader struct {
	self_   any
	load_   func() string // produces the text, once
	closer_ func()
	s_      string
	pos_    int    // byte offset of the next character
	low_    uint16 // second half of a surrogate pair whose first half was read
	mark_   int
	closed_ bool
}

func (this *Reader) as_Reader() *Reader { return this }

func to_Reader(v any) *Reader {
	if v == nil {
		return nil
	}
	return v.(interface{ as_Reader() *Reader }).as_Reader()
}

func is_Reader(v any) bool {
	_, ok := v.(interface{ as_Reader() *Reader })
	return ok
}

func (this *Reader) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *Reader) ensure_() {
	if this.closed_ {
		panic(new_IOException_String("Stream closed").self())
	}
	if this.load_ != nil {
		f := this.load_
		this.load_ = nil
		this.s_ = f()
		this.pos_ = 0
	}
}

// rest_ consumes and returns everything not yet read.
func (this *Reader) rest_() string {
	this.ensure_()
	s := this.s_[this.pos_:]
	this.pos_ = len(this.s_)
	if this.low_ != 0 {
		s = surrogateBytes_(this.low_) + s
		this.low_ = 0
	}
	return s
}

// readAll_ is the remaining text of any Reader.
func readAll_(r *Reader) string {
	if r == nil {
		panic(new_NullPointerException_0().self())
	}
	return r.rest_()
}

// readerOf_ finds the Reader in a Readable.
func readerOf_(v any) *Reader {
	if v == nil {
		panic(new_NullPointerException_0().self())
	}
	return to_Reader(v)
}

// read returns the next UTF-16 code unit, or -1 at the end.
func (this *Reader) read() int32 {
	this.ensure_()
	if this.low_ != 0 {
		c := this.low_
		this.low_ = 0
		return int32(c)
	}
	if this.pos_ >= len(this.s_) {
		return -1
	}
	c := this.s_[this.pos_]
	if c < 0x80 {
		this.pos_++
		return int32(c)
	}
	cp, size := decode_(this.s_, this.pos_)
	this.pos_ += size
	if cp >= 0x10000 {
		cp -= 0x10000
		this.low_ = uint16(0xDC00 + cp&0x3FF)
		return 0xD800 + cp>>10
	}
	return cp
}

func (this *Reader) read_charArr_int_int(buf []uint16, off int32, n int32) int32 {
	if off < 0 || n < 0 || int(off)+int(n) > len(buf) {
		panic(new_IndexOutOfBoundsException_0().self())
	}
	this.ensure_()
	if n == 0 {
		return 0
	}
	var i int32
	for i < n {
		c := this.read()
		if c < 0 {
			break
		}
		buf[off+i] = uint16(c)
		i++
	}
	if i == 0 {
		return -1
	}
	return i
}

func (this *Reader) read_charArr(buf []uint16) int32 {
	return this.read_charArr_int_int(buf, 0, int32(len(buf)))
}

func (this *Reader) skip(n int64) int64 {
	if n < 0 {
		panic(new_IllegalArgumentException_String("skip value is negative").self())
	}
	var i int64
	for i < n && this.read() >= 0 {
		i++
	}
	return i
}

func (this *Reader) ready() bool {
	this.ensure_()
	return true
}

func (this *Reader) markSupported() bool { return true }

func (this *Reader) mark(readAheadLimit int32) {
	this.ensure_()
	this.mark_ = this.pos_
}

func (this *Reader) reset() {
	this.ensure_()
	this.pos_ = this.mark_
	this.low_ = 0
}

func (this *Reader) transferTo(w *Writer) int64 {
	s := this.rest_()
	w.write(s)
	return int64(String_length(s))
}

func (this *Reader) close() {
	if this.closed_ {
		return
	}
	this.closed_ = true
	this.load_ = nil
	this.s_ = ""
	if this.closer_ != nil {
		this.closer_()
	}
}

// readLine_ is BufferedReader.readLine on any reader.
func (this *Reader) readLine_() string {
	this.ensure_()
	if this.low_ != 0 {
		this.low_ = 0
	}
	s := this.s_
	if this.pos_ >= len(s) {
		return nullstr_
	}
	start := this.pos_
	i := strings.IndexAny(s[start:], "\r\n")
	if i < 0 {
		this.pos_ = len(s)
		return s[start:]
	}
	end := start + i
	this.pos_ = end + 1
	if s[end] == '\r' && this.pos_ < len(s) && s[this.pos_] == '\n' {
		this.pos_++
	}
	return s[start:end]
}

type StringReader struct {
	Reader
}

func (this *StringReader) up_Reader() *Reader {
	if this == nil {
		return nil
	}
	return &this.Reader
}

func new_StringReader(s string) *StringReader {
	if s == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	this := &StringReader{}
	this.self_ = this
	this.s_ = s
	return this
}

type InputStreamReader struct {
	Reader
	cs_ *Charset
}

func (this *InputStreamReader) up_Reader() *Reader {
	if this == nil {
		return nil
	}
	return &this.Reader
}

func new_InputStreamReader(in *InputStream, cs *Charset) *InputStreamReader {
	if in == nil {
		panic(new_NullPointerException_0().self())
	}
	if cs == nil {
		panic(new_NullPointerException_String("charset").self())
	}
	this := &InputStreamReader{cs_: cs}
	this.self_ = this
	this.load_ = func() string {
		s, _ := cs.decode_(readAllBytes_(in))
		return s
	}
	this.closer_ = in.close
	return this
}

// newStrictReader_ is a reader whose decoder reports malformed input
// (java.nio.file.Files.newBufferedReader).
func newStrictReader_(in *InputStream, cs *Charset) *InputStreamReader {
	this := new_InputStreamReader(in, cs)
	this.load_ = func() string { return cs.decodeStrict_(readAllBytes_(in)) }
	return this
}

func (this *InputStreamReader) getEncoding() string {
	if this.closed_ {
		return nullstr_
	}
	if this.cs_ == StandardCharsets_UTF_8 {
		return "UTF8"
	}
	return this.cs_.name_
}

type BufferedReader struct {
	Reader
}

func (this *BufferedReader) up_Reader() *Reader {
	if this == nil {
		return nil
	}
	return &this.Reader
}

func new_BufferedReader(r *Reader) *BufferedReader {
	if r == nil {
		panic(new_NullPointerException_0().self())
	}
	this := &BufferedReader{}
	this.self_ = this
	this.load_ = r.rest_
	this.closer_ = r.close
	return this
}

func new_BufferedReader_Reader_int(r *Reader, size int32) *BufferedReader {
	if size <= 0 {
		panic(new_IllegalArgumentException_String("Buffer size <= 0").self())
	}
	return new_BufferedReader(r)
}

// readLine returns the next line without its terminator (\n, \r or \r\n), or
// null at the end of the text.
func (this *BufferedReader) readLine() string { return this.readLine_() }

// ---- java.io.Writer

type Writer struct {
	self_     any
	sink_     func(s string)
	flusher_  func()
	closer_   func()
	closed_   bool
	keepOpen_ bool
}

func (this *Writer) as_Writer() *Writer { return this }

func to_Writer(v any) *Writer {
	if v == nil {
		return nil
	}
	return v.(interface{ as_Writer() *Writer }).as_Writer()
}

func is_Writer(v any) bool {
	_, ok := v.(interface{ as_Writer() *Writer })
	return ok
}

func (this *Writer) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *Writer) appendable_() Appendable {
	if a, ok := this.self_.(Appendable); ok {
		return a
	}
	return this
}

func (this *Writer) write(s string) {
	if s == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	if this.closed_ {
		panic(new_IOException_String("Stream closed").self())
	}
	if this.sink_ != nil && len(s) > 0 {
		this.sink_(s)
	}
}

// write_int writes one char (a UTF-16 code unit).
func (this *Writer) write_int(c int32) { this.write(String_valueOf_char(uint16(c))) }

func (this *Writer) write_charArr(buf []uint16) { this.write(fromUnits_(buf)) }

func (this *Writer) write_charArr_int_int(buf []uint16, off int32, n int32) {
	if off < 0 || n < 0 || int(off)+int(n) > len(buf) {
		panic(new_IndexOutOfBoundsException_0().self())
	}
	this.write(fromUnits_(buf[off : off+n]))
}

func (this *Writer) write_String_int_int(s string, off int32, n int32) {
	this.write(String_substring(s, off, off+n))
}

func (this *Writer) append_CharSequence(v any) Appendable {
	if v == nil {
		this.write("null")
	} else {
		this.write(csString_(v))
	}
	return this.appendable_()
}

func (this *Writer) append_CharSequence_int_int(v any, a int32, b int32) Appendable {
	s := "null"
	if v != nil {
		s = csString_(v)
	}
	this.write(String_substring(s, a, b))
	return this.appendable_()
}

func (this *Writer) append_char(c uint16) Appendable {
	this.write(String_valueOf_char(c))
	return this.appendable_()
}

func (this *Writer) flush() {
	if this.closed_ {
		panic(new_IOException_String("Stream closed").self())
	}
	if this.flusher_ != nil {
		this.flusher_()
	}
}

func (this *Writer) close() {
	if this.closed_ || this.keepOpen_ {
		return
	}
	defer func() { this.closed_ = true }()
	if this.closer_ != nil {
		this.closer_()
	}
}

type StringWriter struct {
	Writer
	sb_ *StringBuilder
}

func (this *StringWriter) up_Writer() *Writer {
	if this == nil {
		return nil
	}
	return &this.Writer
}

func new_StringWriter() *StringWriter {
	this := &StringWriter{sb_: new_StringBuilder()}
	this.self_ = this
	this.sink_ = func(s string) { this.sb_.append(s) }
	this.keepOpen_ = true // closing a StringWriter has no effect
	return this
}

func new_StringWriter_int(n int32) *StringWriter {
	if n < 0 {
		panic(new_IllegalArgumentException_String("Negative buffer size").self())
	}
	return new_StringWriter()
}

func (this *StringWriter) toString() string { return this.sb_.toString() }

type OutputStreamWriter struct {
	Writer
	enc_ *encoder_
	buf_ []byte
	out_ *OutputStream
}

func (this *OutputStreamWriter) up_Writer() *Writer {
	if this == nil {
		return nil
	}
	return &this.Writer
}

func newOutputStreamWriter_(out *OutputStream, enc *encoder_) *OutputStreamWriter {
	if out == nil {
		panic(new_NullPointerException_0().self())
	}
	this := &OutputStreamWriter{enc_: enc, out_: out, buf_: make([]byte, 0, 8192)}
	this.self_ = this
	this.sink_ = func(s string) {
		if len(this.buf_) == 0 && len(s) >= 8192 && enc.plain_(s) {
			writeBytes_(out, strBytes_(s))
			return
		}
		this.buf_ = enc.encode_(this.buf_, s)
		if len(this.buf_) >= 8192 {
			this.flushBuffer_()
		}
	}
	this.flusher_ = func() {
		this.flushBuffer_()
		out.flush()
	}
	this.closer_ = func() {
		defer out.close()
		this.buf_ = enc.finish_(this.buf_)
		this.flushBuffer_()
	}
	return this
}

func (this *OutputStreamWriter) flushBuffer_() {
	if len(this.buf_) > 0 {
		b := this.buf_
		this.buf_ = this.buf_[:0]
		writeBytes_(this.out_, b)
	}
}

func new_OutputStreamWriter(out *OutputStream, cs *Charset) *OutputStreamWriter {
	return newOutputStreamWriter_(out, newEncoder_(cs))
}

func new_OutputStreamWriter_OutputStream_CharsetEncoder(out *OutputStream, enc *CharsetEncoder) *OutputStreamWriter {
	if enc == nil {
		panic(new_NullPointerException_String("charset encoder").self())
	}
	return newOutputStreamWriter_(out, encoderOf_(enc))
}

func (this *OutputStreamWriter) getEncoding() string {
	if this.closed_ {
		return nullstr_
	}
	if this.enc_.cs_ == StandardCharsets_UTF_8 {
		return "UTF8"
	}
	return this.enc_.cs_.name_
}

type BufferedWriter struct {
	Writer
	buf_ []byte
	w_   *Writer
}

func (this *BufferedWriter) up_Writer() *Writer {
	if this == nil {
		return nil
	}
	return &this.Writer
}

func new_BufferedWriter(w *Writer) *BufferedWriter {
	if w == nil {
		panic(new_NullPointerException_0().self())
	}
	this := &BufferedWriter{w_: w}
	this.self_ = this
	// Whole strings are buffered, so a character is never split between two
	// writes to the underlying writer.
	this.sink_ = func(s string) {
		if len(this.buf_) == 0 && len(s) >= 8192 {
			w.write(s)
			return
		}
		this.buf_ = append(this.buf_, s...)
		if len(this.buf_) >= 8192 {
			this.flushBuffer_()
		}
	}
	this.flusher_ = func() {
		this.flushBuffer_()
		w.flush()
	}
	this.closer_ = func() {
		defer w.close()
		this.flushBuffer_()
	}
	return this
}

func new_BufferedWriter_Writer_int(w *Writer, size int32) *BufferedWriter {
	if size <= 0 {
		panic(new_IllegalArgumentException_String("Buffer size <= 0").self())
	}
	return new_BufferedWriter(w)
}

func (this *BufferedWriter) flushBuffer_() {
	if len(this.buf_) > 0 {
		s := string(this.buf_)
		this.buf_ = this.buf_[:0]
		this.w_.write(s)
	}
}

func (this *BufferedWriter) newLine() { this.write("\n") }
