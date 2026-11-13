package closure

// com.google.gson.stream.JsonWriter (Gson 2.9.1 behaviour).

import (
	"strings"
)

const (
	gsonEmptyArray_       = 1
	gsonNonemptyArray_    = 2
	gsonEmptyObject_      = 3
	gsonDanglingName_     = 4
	gsonNonemptyObject_   = 5
	gsonEmptyDocument_    = 6
	gsonNonemptyDocument_ = 7
	gsonClosed_           = 8
)

type JsonWriter struct {
	out_            func(string)
	flush_          func()
	close_          func()
	stack_          []int
	indent_         string // "" means compact
	separator_      string
	lenient_        bool
	htmlSafe_       bool
	deferredName_   string
	hasDeferred_    bool
	serializeNulls_ bool
}

func newJsonWriterFunc_(out func(string), flush func(), closeFn func()) *JsonWriter {
	return &JsonWriter{
		out_:            out,
		flush_:          flush,
		close_:          closeFn,
		stack_:          []int{gsonEmptyDocument_},
		separator_:      ":",
		serializeNulls_: true,
	}
}

// newJsonWriterSB_ is a JsonWriter that appends to a Go string builder.
func newJsonWriterSB_(sb *strings.Builder) *JsonWriter {
	return newJsonWriterFunc_(func(s string) { sb.WriteString(s) }, func() {}, func() {})
}

// gsonWriterSink_ adapts a java.io.Writer (or any Appendable) object.
func gsonWriterSink_(target any) (func(string), func(), func()) {
	var out func(string)
	switch t := target.(type) {
	case interface{ write(string) }:
		out = t.write
	case interface{ append_CharSequence(any) Appendable }:
		out = func(s string) { t.append_CharSequence(s) }
	default:
		panic(new_IllegalArgumentException_String("JsonWriter: unsupported sink " + goTypeName_(target)).self())
	}
	flush := func() {}
	if f, ok := target.(interface{ flush() }); ok {
		flush = f.flush
	}
	closeFn := func() {}
	if c, ok := target.(interface{ close() }); ok {
		closeFn = c.close
	}
	return out, flush, closeFn
}

func new_JsonWriter(p0 *Writer) *JsonWriter {
	if p0 == nil {
		panic(new_NullPointerException_String("out == null").self())
	}
	var target any = p0
	if s := p0.self(); s != nil {
		target = s
	}
	return newJsonWriterFunc_(gsonWriterSink_(target))
}

func (this *JsonWriter) setIndent(indent string) {
	if len(indent) == 0 {
		this.indent_ = ""
		this.separator_ = ":"
	} else {
		this.indent_ = indent
		this.separator_ = ": "
	}
}

func (this *JsonWriter) setLenient(lenient bool)               { this.lenient_ = lenient }
func (this *JsonWriter) isLenient() bool                       { return this.lenient_ }
func (this *JsonWriter) setHtmlSafe(htmlSafe bool)             { this.htmlSafe_ = htmlSafe }
func (this *JsonWriter) isHtmlSafe() bool                      { return this.htmlSafe_ }
func (this *JsonWriter) setSerializeNulls(serializeNulls bool) { this.serializeNulls_ = serializeNulls }
func (this *JsonWriter) getSerializeNulls() bool               { return this.serializeNulls_ }

func (this *JsonWriter) peek_() int {
	if len(this.stack_) == 0 {
		panic(new_IllegalStateException_String("JsonWriter is closed.").self())
	}
	return this.stack_[len(this.stack_)-1]
}

func (this *JsonWriter) replaceTop_(scope int) { this.stack_[len(this.stack_)-1] = scope }

func (this *JsonWriter) beginArray() *JsonWriter {
	this.writeDeferredName_()
	return this.open_(gsonEmptyArray_, "[")
}

func (this *JsonWriter) endArray() *JsonWriter {
	return this.closeScope_(gsonEmptyArray_, gsonNonemptyArray_, "]")
}

func (this *JsonWriter) beginObject() *JsonWriter {
	this.writeDeferredName_()
	return this.open_(gsonEmptyObject_, "{")
}

func (this *JsonWriter) endObject() *JsonWriter {
	return this.closeScope_(gsonEmptyObject_, gsonNonemptyObject_, "}")
}

func (this *JsonWriter) open_(empty int, bracket string) *JsonWriter {
	this.beforeValue_()
	this.stack_ = append(this.stack_, empty)
	this.out_(bracket)
	return this
}

func (this *JsonWriter) closeScope_(empty int, nonempty int, bracket string) *JsonWriter {
	context := this.peek_()
	if context != nonempty && context != empty {
		panic(new_IllegalStateException_String("Nesting problem.").self())
	}
	if this.hasDeferred_ {
		panic(new_IllegalStateException_String("Dangling name: " + this.deferredName_).self())
	}
	this.stack_ = this.stack_[:len(this.stack_)-1]
	if context == nonempty {
		this.newline_()
	}
	this.out_(bracket)
	return this
}

func (this *JsonWriter) name(p0 string) *JsonWriter {
	if p0 == nullstr_ {
		panic(new_NullPointerException_String("name == null").self())
	}
	if this.hasDeferred_ {
		panic(new_IllegalStateException_0().self())
	}
	if len(this.stack_) == 0 {
		panic(new_IllegalStateException_String("JsonWriter is closed.").self())
	}
	this.deferredName_ = p0
	this.hasDeferred_ = true
	return this
}

func (this *JsonWriter) writeDeferredName_() {
	if this.hasDeferred_ {
		this.beforeName_()
		this.string_(this.deferredName_)
		this.hasDeferred_ = false
		this.deferredName_ = ""
	}
}

func (this *JsonWriter) value(p0 string) *JsonWriter {
	if p0 == nullstr_ {
		return this.nullValue()
	}
	this.writeDeferredName_()
	this.beforeValue_()
	this.string_(p0)
	return this
}

func (this *JsonWriter) jsonValue(p0 string) *JsonWriter {
	if p0 == nullstr_ {
		return this.nullValue()
	}
	this.writeDeferredName_()
	this.beforeValue_()
	this.out_(p0)
	return this
}

func (this *JsonWriter) nullValue() *JsonWriter {
	if this.hasDeferred_ {
		if this.serializeNulls_ {
			this.writeDeferredName_()
		} else {
			this.hasDeferred_ = false
			this.deferredName_ = ""
			return this
		}
	}
	this.beforeValue_()
	this.out_("null")
	return this
}

func (this *JsonWriter) value_boolean(p0 bool) *JsonWriter {
	this.writeDeferredName_()
	this.beforeValue_()
	if p0 {
		this.out_("true")
	} else {
		this.out_("false")
	}
	return this
}

// value_Boolean takes a boxed Boolean (nil writes null).
func (this *JsonWriter) value_Boolean(p0 any) *JsonWriter {
	if p0 == nil {
		return this.nullValue()
	}
	return this.value_boolean(p0.(bool))
}

func (this *JsonWriter) value_long(p0 int64) *JsonWriter {
	this.writeDeferredName_()
	this.beforeValue_()
	this.out_(Long_toString(p0))
	return this
}

// value_Number takes a boxed Number (nil writes null).
func (this *JsonWriter) value_Number(p0 any) *JsonWriter {
	if p0 == nil {
		return this.nullValue()
	}
	this.writeDeferredName_()
	s := gsonNumberString_(p0)
	if s == "-Infinity" || s == "Infinity" || s == "NaN" {
		if !this.lenient_ {
			panic(new_IllegalArgumentException_String("Numeric values must be finite, but was " + s).self())
		}
	}
	this.beforeValue_()
	this.out_(s)
	return this
}

// gsonNumberString_ is Number.toString() for a boxed number.
func gsonNumberString_(n any) string {
	switch x := n.(type) {
	case int32:
		return Long_toString(int64(x))
	case int64:
		return Long_toString(x)
	case int16:
		return Long_toString(int64(x))
	case int8:
		return Long_toString(int64(x))
	case float64:
		return Double_toString(x)
	case float32:
		return Float_toString(x)
	case gsonLazyNumber_:
		return string(x)
	}
	return objstr_(n)
}

func (this *JsonWriter) flush() {
	if len(this.stack_) == 0 {
		panic(new_IllegalStateException_String("JsonWriter is closed.").self())
	}
	this.flush_()
}

func (this *JsonWriter) close() {
	this.close_()
	size := len(this.stack_)
	if size > 1 || (size == 1 && this.stack_[0] != gsonNonemptyDocument_) {
		panic(new_IOException_String("Incomplete document").self())
	}
	this.stack_ = this.stack_[:0]
}

func (this *JsonWriter) newline_() {
	if this.indent_ == "" {
		return
	}
	var sb strings.Builder
	sb.WriteByte('\n')
	for i := 1; i < len(this.stack_); i++ {
		sb.WriteString(this.indent_)
	}
	this.out_(sb.String())
}

func (this *JsonWriter) beforeName_() {
	context := this.peek_()
	if context == gsonNonemptyObject_ {
		this.out_(",")
	} else if context != gsonEmptyObject_ {
		panic(new_IllegalStateException_String("Nesting problem.").self())
	}
	this.newline_()
	this.replaceTop_(gsonDanglingName_)
}

func (this *JsonWriter) beforeValue_() {
	switch this.peek_() {
	case gsonNonemptyDocument_:
		if !this.lenient_ {
			panic(new_IllegalStateException_String("JSON must have only one top-level value.").self())
		}
		this.replaceTop_(gsonNonemptyDocument_)
	case gsonEmptyDocument_:
		this.replaceTop_(gsonNonemptyDocument_)
	case gsonEmptyArray_:
		this.replaceTop_(gsonNonemptyArray_)
		this.newline_()
	case gsonNonemptyArray_:
		this.out_(",")
		this.newline_()
	case gsonDanglingName_:
		this.out_(this.separator_)
		this.replaceTop_(gsonNonemptyObject_)
	default:
		panic(new_IllegalStateException_String("Nesting problem.").self())
	}
}

const gsonHex_ = "0123456789abcdef"

// gsonQuote_ is a JSON string literal as JsonWriter writes it.
func gsonQuote_(s string, htmlSafe bool) string {
	var sb strings.Builder
	sb.Grow(len(s) + 2)
	sb.WriteByte('"')
	last := 0
	n := len(s)
	for i := 0; i < n; i++ {
		c := s[i]
		var rep string
		switch {
		case c == '"':
			rep = "\\\""
		case c == '\\':
			rep = "\\\\"
		case c == '\t':
			rep = "\\t"
		case c == '\b':
			rep = "\\b"
		case c == '\n':
			rep = "\\n"
		case c == '\r':
			rep = "\\r"
		case c == '\f':
			rep = "\\f"
		case c < 0x20:
			rep = "\\u00" + string(gsonHex_[c>>4]) + string(gsonHex_[c&15])
		case c < 0x80:
			if !htmlSafe {
				continue
			}
			switch c {
			case '<':
				rep = "\\u003c"
			case '>':
				rep = "\\u003e"
			case '&':
				rep = "\\u0026"
			case '=':
				rep = "\\u003d"
			case '\'':
				rep = "\\u0027"
			default:
				continue
			}
		case c == 0xE2 && i+2 < n && s[i+1] == 0x80 && (s[i+2] == 0xA8 || s[i+2] == 0xA9):
			if s[i+2] == 0xA8 {
				rep = "\\u2028"
			} else {
				rep = "\\u2029"
			}
			sb.WriteString(s[last:i])
			sb.WriteString(rep)
			i += 2
			last = i + 1
			continue
		default:
			continue
		}
		sb.WriteString(s[last:i])
		sb.WriteString(rep)
		last = i + 1
	}
	sb.WriteString(s[last:])
	sb.WriteByte('"')
	return sb.String()
}

func (this *JsonWriter) string_(s string) {
	this.out_(gsonQuote_(s, this.htmlSafe_))
}
