package closure

// com.google.gson.stream.JsonReader (Gson 2.9.1 behaviour).

import (
	"math"
	"strings"
)

// ---- JsonToken

type JsonToken struct {
	Enum
}

func newJsonToken_(name string, ordinal int32) *JsonToken {
	t := &JsonToken{}
	t.self_ = t
	t.init_Enum(name, ordinal)
	return t
}

var JsonToken_BEGIN_ARRAY = newJsonToken_("BEGIN_ARRAY", 0)
var JsonToken_END_ARRAY = newJsonToken_("END_ARRAY", 1)
var JsonToken_BEGIN_OBJECT = newJsonToken_("BEGIN_OBJECT", 2)
var JsonToken_END_OBJECT = newJsonToken_("END_OBJECT", 3)
var JsonToken_NAME = newJsonToken_("NAME", 4)
var JsonToken_STRING = newJsonToken_("STRING", 5)
var JsonToken_NUMBER = newJsonToken_("NUMBER", 6)
var JsonToken_BOOLEAN = newJsonToken_("BOOLEAN", 7)
var JsonToken_NULL = newJsonToken_("NULL", 8)
var JsonToken_END_DOCUMENT = newJsonToken_("END_DOCUMENT", 9)

var JsonToken_values_ = []*JsonToken{JsonToken_BEGIN_ARRAY, JsonToken_END_ARRAY, JsonToken_BEGIN_OBJECT, JsonToken_END_OBJECT, JsonToken_NAME, JsonToken_STRING, JsonToken_NUMBER, JsonToken_BOOLEAN, JsonToken_NULL, JsonToken_END_DOCUMENT}

// ---- exceptions private to the reader

// MalformedJsonException is com.google.gson.stream.MalformedJsonException.
type MalformedJsonException struct {
	IOException
}

func new_MalformedJsonException_String(p0 string) *MalformedJsonException {
	this := &MalformedJsonException{}
	this.self_ = this
	this.IOException.init_IOException_String(p0)
	return this
}

// gsonEOFException_ is java.io.EOFException as the reader throws it.
type gsonEOFException_ struct {
	IOException
}

func newGsonEOFException_(msg string) *gsonEOFException_ {
	this := &gsonEOFException_{}
	this.self_ = this
	this.IOException.init_IOException_String(msg)
	return this
}

func init() {
	javaNames_["MalformedJsonException"] = "com.google.gson.stream.MalformedJsonException"
	javaNames_["gsonEOFException_"] = "java.io.EOFException"
	javaNames_["JsonToken"] = "com.google.gson.stream.JsonToken"
}

// ---- JsonReader

const (
	gsonPeekedNone_ = iota
	gsonPeekedBeginObject_
	gsonPeekedEndObject_
	gsonPeekedBeginArray_
	gsonPeekedEndArray_
	gsonPeekedTrue_
	gsonPeekedFalse_
	gsonPeekedNull_
	gsonPeekedSingleQuoted_
	gsonPeekedDoubleQuoted_
	gsonPeekedUnquoted_
	gsonPeekedBuffered_
	gsonPeekedSingleQuotedName_
	gsonPeekedDoubleQuotedName_
	gsonPeekedUnquotedName_
	gsonPeekedLong_
	gsonPeekedNumber_
	gsonPeekedEOF_
)

type JsonReader struct {
	src_      func() int32 // next UTF-16 unit, or -1
	closeSrc_ func()
	eof_      bool
	buf_      []uint16
	pos_      int
	lenient_  bool

	lineNumber_ int
	lineStart_  int

	peeked_       int
	peekedLong_   int64
	peekedNumLen_ int
	peekedString_ string

	stack_       []int
	pathNames_   []string
	pathIndices_ []int
}

func newJsonReaderFunc_(src func() int32, closeFn func()) *JsonReader {
	return &JsonReader{
		src_:         src,
		closeSrc_:    closeFn,
		stack_:       []int{gsonEmptyDocument_},
		pathNames_:   []string{nullstr_},
		pathIndices_: []int{0},
	}
}

// newJsonReaderFromString_ reads JSON text held in a string.
func newJsonReaderFromString_(s string) *JsonReader {
	r := newJsonReaderFunc_(func() int32 { return -1 }, func() {})
	r.buf_ = String_toCharArray(s)
	r.eof_ = true
	return r
}

// gsonReadAll_, when set by the integrator, returns the whole remaining text of
// a java.io.Reader object. It is optional: without it the reader pulls
// characters one at a time through Reader.read().
var gsonReadAll_ func(r any) string

func new_JsonReader(p0 *Reader) *JsonReader {
	if p0 == nil {
		panic(new_NullPointerException_String("in == null").self())
	}
	var target any = p0
	if s := p0.self(); s != nil {
		target = s
	}
	closeFn := func() {}
	if c, ok := target.(interface{ close() }); ok {
		closeFn = c.close
	}
	if gsonReadAll_ != nil {
		r := newJsonReaderFromString_(gsonReadAll_(target))
		r.closeSrc_ = closeFn
		return r
	}
	rd := target.(interface{ read() int32 })
	return newJsonReaderFunc_(rd.read, closeFn)
}

func (this *JsonReader) setLenient(lenient bool) { this.lenient_ = lenient }
func (this *JsonReader) isLenient() bool         { return this.lenient_ }

// at_ is the unit at absolute buffer index i, or -1 past the end of input.
func (this *JsonReader) at_(i int) int32 {
	for i >= len(this.buf_) {
		if this.eof_ {
			return -1
		}
		c := this.src_()
		if c < 0 {
			this.eof_ = true
			return -1
		}
		this.buf_ = append(this.buf_, uint16(c))
	}
	return int32(this.buf_[i])
}

func (this *JsonReader) top_() int { return this.stack_[len(this.stack_)-1] }

func (this *JsonReader) setTop_(scope int) { this.stack_[len(this.stack_)-1] = scope }

func (this *JsonReader) push_(scope int) {
	this.stack_ = append(this.stack_, scope)
	this.pathNames_ = append(this.pathNames_, nullstr_)
	this.pathIndices_ = append(this.pathIndices_, 0)
}

func (this *JsonReader) pop_() {
	n := len(this.stack_) - 1
	this.stack_ = this.stack_[:n]
	this.pathNames_ = this.pathNames_[:n]
	this.pathIndices_ = this.pathIndices_[:n]
}

func (this *JsonReader) beginArray() {
	p := this.peeked_
	if p == gsonPeekedNone_ {
		p = this.doPeek_()
	}
	if p == gsonPeekedBeginArray_ {
		this.push_(gsonEmptyArray_)
		this.pathIndices_[len(this.stack_)-1] = 0
		this.peeked_ = gsonPeekedNone_
	} else {
		panic(new_IllegalStateException_String("Expected BEGIN_ARRAY but was " + this.peek().name() + this.locationString_()).self())
	}
}

func (this *JsonReader) endArray() {
	p := this.peeked_
	if p == gsonPeekedNone_ {
		p = this.doPeek_()
	}
	if p == gsonPeekedEndArray_ {
		this.pop_()
		this.pathIndices_[len(this.stack_)-1]++
		this.peeked_ = gsonPeekedNone_
	} else {
		panic(new_IllegalStateException_String("Expected END_ARRAY but was " + this.peek().name() + this.locationString_()).self())
	}
}

func (this *JsonReader) beginObject() {
	p := this.peeked_
	if p == gsonPeekedNone_ {
		p = this.doPeek_()
	}
	if p == gsonPeekedBeginObject_ {
		this.push_(gsonEmptyObject_)
		this.peeked_ = gsonPeekedNone_
	} else {
		panic(new_IllegalStateException_String("Expected BEGIN_OBJECT but was " + this.peek().name() + this.locationString_()).self())
	}
}

func (this *JsonReader) endObject() {
	p := this.peeked_
	if p == gsonPeekedNone_ {
		p = this.doPeek_()
	}
	if p == gsonPeekedEndObject_ {
		this.pop_()
		this.pathNames_[len(this.stack_)-1] = nullstr_
		this.pathIndices_[len(this.stack_)-1]++
		this.peeked_ = gsonPeekedNone_
	} else {
		panic(new_IllegalStateException_String("Expected END_OBJECT but was " + this.peek().name() + this.locationString_()).self())
	}
}

func (this *JsonReader) hasNext() bool {
	p := this.peeked_
	if p == gsonPeekedNone_ {
		p = this.doPeek_()
	}
	return p != gsonPeekedEndObject_ && p != gsonPeekedEndArray_ && p != gsonPeekedEOF_
}

func (this *JsonReader) peek() *JsonToken {
	p := this.peeked_
	if p == gsonPeekedNone_ {
		p = this.doPeek_()
	}
	switch p {
	case gsonPeekedBeginObject_:
		return JsonToken_BEGIN_OBJECT
	case gsonPeekedEndObject_:
		return JsonToken_END_OBJECT
	case gsonPeekedBeginArray_:
		return JsonToken_BEGIN_ARRAY
	case gsonPeekedEndArray_:
		return JsonToken_END_ARRAY
	case gsonPeekedSingleQuotedName_, gsonPeekedDoubleQuotedName_, gsonPeekedUnquotedName_:
		return JsonToken_NAME
	case gsonPeekedTrue_, gsonPeekedFalse_:
		return JsonToken_BOOLEAN
	case gsonPeekedNull_:
		return JsonToken_NULL
	case gsonPeekedSingleQuoted_, gsonPeekedDoubleQuoted_, gsonPeekedUnquoted_, gsonPeekedBuffered_:
		return JsonToken_STRING
	case gsonPeekedLong_, gsonPeekedNumber_:
		return JsonToken_NUMBER
	case gsonPeekedEOF_:
		return JsonToken_END_DOCUMENT
	}
	panic(new_AssertionError_0().self())
}

func (this *JsonReader) doPeek_() int {
	peekStack := this.top_()
	switch peekStack {
	case gsonEmptyArray_:
		this.setTop_(gsonNonemptyArray_)
	case gsonNonemptyArray_:
		c := this.nextNonWhitespace_(true)
		switch c {
		case ']':
			this.peeked_ = gsonPeekedEndArray_
			return this.peeked_
		case ';':
			this.checkLenient_()
		case ',':
		default:
			panic(this.syntaxError_("Unterminated array"))
		}
	case gsonEmptyObject_, gsonNonemptyObject_:
		this.setTop_(gsonDanglingName_)
		if peekStack == gsonNonemptyObject_ {
			c := this.nextNonWhitespace_(true)
			switch c {
			case '}':
				this.peeked_ = gsonPeekedEndObject_
				return this.peeked_
			case ';':
				this.checkLenient_()
			case ',':
			default:
				panic(this.syntaxError_("Unterminated object"))
			}
		}
		c := this.nextNonWhitespace_(true)
		switch c {
		case '"':
			this.peeked_ = gsonPeekedDoubleQuotedName_
			return this.peeked_
		case '\'':
			this.checkLenient_()
			this.peeked_ = gsonPeekedSingleQuotedName_
			return this.peeked_
		case '}':
			if peekStack != gsonNonemptyObject_ {
				this.peeked_ = gsonPeekedEndObject_
				return this.peeked_
			}
			panic(this.syntaxError_("Expected name"))
		default:
			this.checkLenient_()
			this.pos_--
			if this.isLiteral_(c) {
				this.peeked_ = gsonPeekedUnquotedName_
				return this.peeked_
			}
			panic(this.syntaxError_("Expected name"))
		}
	case gsonDanglingName_:
		this.setTop_(gsonNonemptyObject_)
		c := this.nextNonWhitespace_(true)
		switch c {
		case ':':
		case '=':
			this.checkLenient_()
			if this.at_(this.pos_) == '>' {
				this.pos_++
			}
		default:
			panic(this.syntaxError_("Expected ':'"))
		}
	case gsonEmptyDocument_:
		if this.lenient_ {
			this.consumeNonExecutePrefix_()
		}
		this.setTop_(gsonNonemptyDocument_)
	case gsonNonemptyDocument_:
		c := this.nextNonWhitespace_(false)
		if c == -1 {
			this.peeked_ = gsonPeekedEOF_
			return this.peeked_
		}
		this.checkLenient_()
		this.pos_--
	case gsonClosed_:
		panic(new_IllegalStateException_String("JsonReader is closed").self())
	}

	c := this.nextNonWhitespace_(true)
	switch c {
	case ']', ';', ',':
		if c == ']' && peekStack == gsonEmptyArray_ {
			this.peeked_ = gsonPeekedEndArray_
			return this.peeked_
		}
		if peekStack == gsonEmptyArray_ || peekStack == gsonNonemptyArray_ {
			this.checkLenient_()
			this.pos_--
			this.peeked_ = gsonPeekedNull_
			return this.peeked_
		}
		panic(this.syntaxError_("Unexpected value"))
	case '\'':
		this.checkLenient_()
		this.peeked_ = gsonPeekedSingleQuoted_
		return this.peeked_
	case '"':
		this.peeked_ = gsonPeekedDoubleQuoted_
		return this.peeked_
	case '[':
		this.peeked_ = gsonPeekedBeginArray_
		return this.peeked_
	case '{':
		this.peeked_ = gsonPeekedBeginObject_
		return this.peeked_
	default:
		this.pos_--
	}

	if r := this.peekKeyword_(); r != gsonPeekedNone_ {
		return r
	}
	if r := this.peekNumber_(); r != gsonPeekedNone_ {
		return r
	}
	if !this.isLiteral_(this.at_(this.pos_)) {
		panic(this.syntaxError_("Expected value"))
	}
	this.checkLenient_()
	this.peeked_ = gsonPeekedUnquoted_
	return this.peeked_
}

func (this *JsonReader) peekKeyword_() int {
	c := this.at_(this.pos_)
	var keyword, upper string
	var peeking int
	switch c {
	case 't', 'T':
		keyword, upper, peeking = "true", "TRUE", gsonPeekedTrue_
	case 'f', 'F':
		keyword, upper, peeking = "false", "FALSE", gsonPeekedFalse_
	case 'n', 'N':
		keyword, upper, peeking = "null", "NULL", gsonPeekedNull_
	default:
		return gsonPeekedNone_
	}
	n := len(keyword)
	for i := 1; i < n; i++ {
		c = this.at_(this.pos_ + i)
		if c != int32(keyword[i]) && c != int32(upper[i]) {
			return gsonPeekedNone_
		}
	}
	if c := this.at_(this.pos_ + n); c != -1 && this.isLiteral_(c) {
		return gsonPeekedNone_
	}
	this.pos_ += n
	this.peeked_ = peeking
	return peeking
}

func (this *JsonReader) peekNumber_() int {
	const (
		numNone = iota
		numSign
		numDigit
		numDecimal
		numFractionDigit
		numExpE
		numExpSign
		numExpDigit
	)
	const minIncompleteInteger = math.MinInt64 / 10
	var value int64
	negative := false
	fitsInLong := true
	last := numNone
	i := 0
loop:
	for ; ; i++ {
		c := this.at_(this.pos_ + i)
		if c == -1 {
			break
		}
		switch c {
		case '-':
			if last == numNone {
				negative = true
				last = numSign
				continue
			} else if last == numExpE {
				last = numExpSign
				continue
			}
			return gsonPeekedNone_
		case '+':
			if last == numExpE {
				last = numExpSign
				continue
			}
			return gsonPeekedNone_
		case 'e', 'E':
			if last == numDigit || last == numFractionDigit {
				last = numExpE
				continue
			}
			return gsonPeekedNone_
		case '.':
			if last == numDigit {
				last = numDecimal
				continue
			}
			return gsonPeekedNone_
		default:
			if c < '0' || c > '9' {
				if !this.isLiteral_(c) {
					break loop
				}
				return gsonPeekedNone_
			}
			if last == numSign || last == numNone {
				value = -int64(c - '0')
				last = numDigit
			} else if last == numDigit {
				if value == 0 {
					return gsonPeekedNone_
				}
				newValue := value*10 - int64(c-'0')
				fitsInLong = fitsInLong && (value > minIncompleteInteger || (value == minIncompleteInteger && newValue < value))
				value = newValue
			} else if last == numDecimal {
				last = numFractionDigit
			} else if last == numExpE || last == numExpSign {
				last = numExpDigit
			}
		}
	}
	if last == numDigit && fitsInLong && (value != math.MinInt64 || negative) && (value != 0 || !negative) {
		if negative {
			this.peekedLong_ = value
		} else {
			this.peekedLong_ = -value
		}
		this.pos_ += i
		this.peeked_ = gsonPeekedLong_
		return this.peeked_
	} else if last == numDigit || last == numFractionDigit || last == numExpDigit {
		this.peekedNumLen_ = i
		this.peeked_ = gsonPeekedNumber_
		return this.peeked_
	}
	return gsonPeekedNone_
}

func (this *JsonReader) isLiteral_(c int32) bool {
	switch c {
	case '/', '\\', ';', '#', '=':
		this.checkLenient_()
		return false
	case '{', '}', '[', ']', ':', ',', ' ', '\t', '\f', '\r', '\n':
		return false
	}
	return true
}

func (this *JsonReader) nextName() string {
	p := this.peeked_
	if p == gsonPeekedNone_ {
		p = this.doPeek_()
	}
	var result string
	switch p {
	case gsonPeekedUnquotedName_:
		result = this.nextUnquotedValue_()
	case gsonPeekedSingleQuotedName_:
		result = this.nextQuotedValue_('\'')
	case gsonPeekedDoubleQuotedName_:
		result = this.nextQuotedValue_('"')
	default:
		panic(new_IllegalStateException_String("Expected a name but was " + this.peek().name() + this.locationString_()).self())
	}
	this.peeked_ = gsonPeekedNone_
	this.pathNames_[len(this.stack_)-1] = result
	return result
}

func (this *JsonReader) takeNumberText_() string {
	s := fromUnits_(this.buf_[this.pos_ : this.pos_+this.peekedNumLen_])
	this.pos_ += this.peekedNumLen_
	return s
}

func (this *JsonReader) nextString() string {
	p := this.peeked_
	if p == gsonPeekedNone_ {
		p = this.doPeek_()
	}
	var result string
	switch p {
	case gsonPeekedUnquoted_:
		result = this.nextUnquotedValue_()
	case gsonPeekedSingleQuoted_:
		result = this.nextQuotedValue_('\'')
	case gsonPeekedDoubleQuoted_:
		result = this.nextQuotedValue_('"')
	case gsonPeekedBuffered_:
		result = this.peekedString_
		this.peekedString_ = ""
	case gsonPeekedLong_:
		result = Long_toString(this.peekedLong_)
	case gsonPeekedNumber_:
		result = this.takeNumberText_()
	default:
		panic(new_IllegalStateException_String("Expected a string but was " + this.peek().name() + this.locationString_()).self())
	}
	this.peeked_ = gsonPeekedNone_
	this.pathIndices_[len(this.stack_)-1]++
	return result
}

func (this *JsonReader) nextBoolean() bool {
	p := this.peeked_
	if p == gsonPeekedNone_ {
		p = this.doPeek_()
	}
	if p == gsonPeekedTrue_ {
		this.peeked_ = gsonPeekedNone_
		this.pathIndices_[len(this.stack_)-1]++
		return true
	} else if p == gsonPeekedFalse_ {
		this.peeked_ = gsonPeekedNone_
		this.pathIndices_[len(this.stack_)-1]++
		return false
	}
	panic(new_IllegalStateException_String("Expected a boolean but was " + this.peek().name() + this.locationString_()).self())
}

func (this *JsonReader) nextNull() {
	p := this.peeked_
	if p == gsonPeekedNone_ {
		p = this.doPeek_()
	}
	if p == gsonPeekedNull_ {
		this.peeked_ = gsonPeekedNone_
		this.pathIndices_[len(this.stack_)-1]++
	} else {
		panic(new_IllegalStateException_String("Expected null but was " + this.peek().name() + this.locationString_()).self())
	}
}

func (this *JsonReader) nextDouble() float64 {
	p := this.peeked_
	if p == gsonPeekedNone_ {
		p = this.doPeek_()
	}
	if p == gsonPeekedLong_ {
		this.peeked_ = gsonPeekedNone_
		this.pathIndices_[len(this.stack_)-1]++
		return float64(this.peekedLong_)
	}
	if p == gsonPeekedNumber_ {
		this.peekedString_ = this.takeNumberText_()
	} else if p == gsonPeekedSingleQuoted_ || p == gsonPeekedDoubleQuoted_ {
		q := int32('"')
		if p == gsonPeekedSingleQuoted_ {
			q = '\''
		}
		this.peekedString_ = this.nextQuotedValue_(q)
	} else if p == gsonPeekedUnquoted_ {
		this.peekedString_ = this.nextUnquotedValue_()
	} else if p != gsonPeekedBuffered_ {
		panic(new_IllegalStateException_String("Expected a double but was " + this.peek().name() + this.locationString_()).self())
	}
	this.peeked_ = gsonPeekedBuffered_
	result := Double_parseDouble(this.peekedString_)
	if !this.lenient_ && (math.IsNaN(result) || math.IsInf(result, 0)) {
		panic(new_MalformedJsonException_String("JSON forbids NaN and infinities: " + Double_toString(result) + this.locationString_()).self())
	}
	this.peekedString_ = ""
	this.peeked_ = gsonPeekedNone_
	this.pathIndices_[len(this.stack_)-1]++
	return result
}

// gsonTryParseLong_ is Long.parseLong without the exception.
func gsonTryParseLong_(s string) (int64, bool) {
	if s == "" || len(s) > 20 {
		return 0, false
	}
	i := 0
	neg := false
	if s[0] == '-' || s[0] == '+' {
		neg = s[0] == '-'
		i = 1
		if len(s) == 1 {
			return 0, false
		}
	}
	var v uint64
	for ; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		nv := v*10 + uint64(c-'0')
		if nv < v || v > math.MaxUint64/10 {
			return 0, false
		}
		v = nv
	}
	if neg {
		if v > 1<<63 {
			return 0, false
		}
		return -int64(v), true
	}
	if v > math.MaxInt64 {
		return 0, false
	}
	return int64(v), true
}

func (this *JsonReader) nextLong() int64 {
	p := this.peeked_
	if p == gsonPeekedNone_ {
		p = this.doPeek_()
	}
	if p == gsonPeekedLong_ {
		this.peeked_ = gsonPeekedNone_
		this.pathIndices_[len(this.stack_)-1]++
		return this.peekedLong_
	}
	if p == gsonPeekedNumber_ {
		this.peekedString_ = this.takeNumberText_()
	} else if p == gsonPeekedSingleQuoted_ || p == gsonPeekedDoubleQuoted_ || p == gsonPeekedUnquoted_ {
		if p == gsonPeekedUnquoted_ {
			this.peekedString_ = this.nextUnquotedValue_()
		} else if p == gsonPeekedSingleQuoted_ {
			this.peekedString_ = this.nextQuotedValue_('\'')
		} else {
			this.peekedString_ = this.nextQuotedValue_('"')
		}
		if v, ok := gsonTryParseLong_(this.peekedString_); ok {
			this.peeked_ = gsonPeekedNone_
			this.pathIndices_[len(this.stack_)-1]++
			return v
		}
	} else {
		panic(new_IllegalStateException_String("Expected a long but was " + this.peek().name() + this.locationString_()).self())
	}
	this.peeked_ = gsonPeekedBuffered_
	asDouble := Double_parseDouble(this.peekedString_)
	result := f2i_[int64](asDouble)
	if float64(result) != asDouble {
		panic(new_NumberFormatException_String("Expected a long but was " + this.peekedString_ + this.locationString_()).self())
	}
	this.peekedString_ = ""
	this.peeked_ = gsonPeekedNone_
	this.pathIndices_[len(this.stack_)-1]++
	return result
}

func (this *JsonReader) nextInt() int32 {
	p := this.peeked_
	if p == gsonPeekedNone_ {
		p = this.doPeek_()
	}
	if p == gsonPeekedLong_ {
		result := int32(this.peekedLong_)
		if this.peekedLong_ != int64(result) {
			panic(new_NumberFormatException_String("Expected an int but was " + Long_toString(this.peekedLong_) + this.locationString_()).self())
		}
		this.peeked_ = gsonPeekedNone_
		this.pathIndices_[len(this.stack_)-1]++
		return result
	}
	if p == gsonPeekedNumber_ {
		this.peekedString_ = this.takeNumberText_()
	} else if p == gsonPeekedSingleQuoted_ || p == gsonPeekedDoubleQuoted_ || p == gsonPeekedUnquoted_ {
		if p == gsonPeekedUnquoted_ {
			this.peekedString_ = this.nextUnquotedValue_()
		} else if p == gsonPeekedSingleQuoted_ {
			this.peekedString_ = this.nextQuotedValue_('\'')
		} else {
			this.peekedString_ = this.nextQuotedValue_('"')
		}
		if v, ok := gsonTryParseLong_(this.peekedString_); ok && v >= math.MinInt32 && v <= math.MaxInt32 {
			this.peeked_ = gsonPeekedNone_
			this.pathIndices_[len(this.stack_)-1]++
			return int32(v)
		}
	} else {
		panic(new_IllegalStateException_String("Expected an int but was " + this.peek().name() + this.locationString_()).self())
	}
	this.peeked_ = gsonPeekedBuffered_
	asDouble := Double_parseDouble(this.peekedString_)
	result := f2i_[int32](asDouble)
	if float64(result) != asDouble {
		panic(new_NumberFormatException_String("Expected an int but was " + this.peekedString_ + this.locationString_()).self())
	}
	this.peekedString_ = ""
	this.peeked_ = gsonPeekedNone_
	this.pathIndices_[len(this.stack_)-1]++
	return result
}

func (this *JsonReader) close() {
	this.peeked_ = gsonPeekedNone_
	this.stack_ = []int{gsonClosed_}
	this.pathNames_ = []string{nullstr_}
	this.pathIndices_ = []int{0}
	this.closeSrc_()
}

func (this *JsonReader) skipValue() {
	count := 0
	for {
		p := this.peeked_
		if p == gsonPeekedNone_ {
			p = this.doPeek_()
		}
		switch p {
		case gsonPeekedBeginArray_:
			this.push_(gsonEmptyArray_)
			count++
		case gsonPeekedBeginObject_:
			this.push_(gsonEmptyObject_)
			count++
		case gsonPeekedEndArray_, gsonPeekedEndObject_:
			this.pop_()
			count--
		case gsonPeekedUnquotedName_, gsonPeekedUnquoted_:
			this.nextUnquotedValue_()
		case gsonPeekedSingleQuoted_, gsonPeekedSingleQuotedName_:
			this.nextQuotedValue_('\'')
		case gsonPeekedDoubleQuoted_, gsonPeekedDoubleQuotedName_:
			this.nextQuotedValue_('"')
		case gsonPeekedNumber_:
			this.pos_ += this.peekedNumLen_
		}
		this.peeked_ = gsonPeekedNone_
		if count <= 0 {
			break
		}
	}
	this.pathIndices_[len(this.stack_)-1]++
	this.pathNames_[len(this.stack_)-1] = "null"
}

func (this *JsonReader) nextQuotedValue_(quote int32) string {
	var out []uint16
	for {
		c := this.at_(this.pos_)
		if c == -1 {
			panic(this.syntaxError_("Unterminated string"))
		}
		this.pos_++
		if c == quote {
			return fromUnits_(out)
		} else if c == '\\' {
			out = append(out, this.readEscapeCharacter_())
			continue
		} else if c == '\n' {
			this.lineNumber_++
			this.lineStart_ = this.pos_
		}
		out = append(out, uint16(c))
	}
}

func (this *JsonReader) nextUnquotedValue_() string {
	start := this.pos_
	i := this.pos_
loop:
	for ; ; i++ {
		c := this.at_(i)
		switch c {
		case -1:
			break loop
		case '/', '\\', ';', '#', '=':
			this.checkLenient_()
			break loop
		case '{', '}', '[', ']', ':', ',', ' ', '\t', '\f', '\r', '\n':
			break loop
		}
	}
	this.pos_ = i
	return fromUnits_(this.buf_[start:i])
}

func (this *JsonReader) readEscapeCharacter_() uint16 {
	escaped := this.at_(this.pos_)
	if escaped == -1 {
		panic(this.syntaxError_("Unterminated escape sequence"))
	}
	this.pos_++
	switch escaped {
	case 'u':
		if this.at_(this.pos_+3) == -1 {
			panic(this.syntaxError_("Unterminated escape sequence"))
		}
		var result uint16
		for i := this.pos_; i < this.pos_+4; i++ {
			c := this.buf_[i]
			result <<= 4
			switch {
			case c >= '0' && c <= '9':
				result += c - '0'
			case c >= 'a' && c <= 'f':
				result += c - 'a' + 10
			case c >= 'A' && c <= 'F':
				result += c - 'A' + 10
			default:
				panic(new_NumberFormatException_String("\\u" + fromUnits_(this.buf_[this.pos_:this.pos_+4])).self())
			}
		}
		this.pos_ += 4
		return result
	case 't':
		return '\t'
	case 'b':
		return '\b'
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 'f':
		return '\f'
	case '\n':
		this.lineNumber_++
		this.lineStart_ = this.pos_
		return '\n'
	case '\'', '"', '\\', '/':
		return uint16(escaped)
	}
	panic(this.syntaxError_("Invalid escape sequence"))
}

func (this *JsonReader) nextNonWhitespace_(throwOnEof bool) int32 {
	for {
		c := this.at_(this.pos_)
		if c == -1 {
			break
		}
		this.pos_++
		if c == '\n' {
			this.lineNumber_++
			this.lineStart_ = this.pos_
			continue
		} else if c == ' ' || c == '\r' || c == '\t' {
			continue
		}
		if c == '/' {
			peek := this.at_(this.pos_)
			if peek == -1 {
				return c
			}
			this.checkLenient_()
			switch peek {
			case '*':
				this.pos_++
				if !this.skipTo_("*/") {
					panic(this.syntaxError_("Unterminated comment"))
				}
				this.pos_ += 2
				continue
			case '/':
				this.pos_++
				this.skipToEndOfLine_()
				continue
			default:
				return c
			}
		} else if c == '#' {
			this.checkLenient_()
			this.skipToEndOfLine_()
		} else {
			return c
		}
	}
	if throwOnEof {
		panic(newGsonEOFException_("End of input" + this.locationString_()).self())
	}
	return -1
}

func (this *JsonReader) checkLenient_() {
	if !this.lenient_ {
		panic(this.syntaxError_("Use JsonReader.setLenient(true) to accept malformed JSON"))
	}
}

func (this *JsonReader) skipToEndOfLine_() {
	for {
		c := this.at_(this.pos_)
		if c == -1 {
			return
		}
		this.pos_++
		if c == '\n' {
			this.lineNumber_++
			this.lineStart_ = this.pos_
			return
		} else if c == '\r' {
			return
		}
	}
}

func (this *JsonReader) skipTo_(toFind string) bool {
	n := len(toFind)
outer:
	for ; this.at_(this.pos_+n-1) != -1; this.pos_++ {
		if this.buf_[this.pos_] == '\n' {
			this.lineNumber_++
			this.lineStart_ = this.pos_ + 1
			continue
		}
		for c := 0; c < n; c++ {
			if this.buf_[this.pos_+c] != uint16(toFind[c]) {
				continue outer
			}
		}
		return true
	}
	return false
}

func (this *JsonReader) toString() string {
	return "JsonReader" + this.locationString_()
}

func (this *JsonReader) locationString_() string {
	line := this.lineNumber_ + 1
	column := this.pos_ - this.lineStart_ + 1
	return " at line " + istr_(int64(line)) + " column " + istr_(int64(column)) + " path " + this.getPath()
}

func (this *JsonReader) getPath() string { return this.path_(false) }

// getPreviousPath is the path of the value that was consumed last.
func (this *JsonReader) getPreviousPath() string { return this.path_(true) }

func (this *JsonReader) path_(usePrevious bool) string {
	var sb strings.Builder
	sb.WriteByte('$')
	for i := 0; i < len(this.stack_); i++ {
		switch this.stack_[i] {
		case gsonEmptyArray_, gsonNonemptyArray_:
			idx := this.pathIndices_[i]
			if usePrevious && idx > 0 && i == len(this.stack_)-1 {
				idx--
			}
			sb.WriteByte('[')
			sb.WriteString(istr_(int64(idx)))
			sb.WriteByte(']')
		case gsonEmptyObject_, gsonDanglingName_, gsonNonemptyObject_:
			sb.WriteByte('.')
			if this.pathNames_[i] != nullstr_ {
				sb.WriteString(this.pathNames_[i])
			}
		}
	}
	return sb.String()
}

func (this *JsonReader) consumeNonExecutePrefix_() {
	this.nextNonWhitespace_(true)
	this.pos_--
	const prefix = ")]}'\n"
	for i := 0; i < len(prefix); i++ {
		if this.at_(this.pos_+i) != int32(prefix[i]) {
			return
		}
	}
	this.pos_ += len(prefix)
}

func (this *JsonReader) syntaxError_(message string) any {
	return new_MalformedJsonException_String(message + this.locationString_()).self()
}

// promoteNameToValue_ lets a pending property name be read as a string value
// (JsonReaderInternalAccess.promoteNameToValue).
func (this *JsonReader) promoteNameToValue_() {
	p := this.peeked_
	if p == gsonPeekedNone_ {
		p = this.doPeek_()
	}
	switch p {
	case gsonPeekedDoubleQuotedName_:
		this.peeked_ = gsonPeekedDoubleQuoted_
	case gsonPeekedSingleQuotedName_:
		this.peeked_ = gsonPeekedSingleQuoted_
	case gsonPeekedUnquotedName_:
		this.peeked_ = gsonPeekedUnquoted_
	default:
		panic(new_IllegalStateException_String("Expected a name but was " + this.peek().name() + this.locationString_()).self())
	}
}

func (this *JsonToken) up_Enum() *Enum {
	if this == nil {
		return nil
	}
	return &this.Enum
}
