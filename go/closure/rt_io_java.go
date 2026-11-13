package closure

// What is left of java.io now that the compiler does its I/O through Io (rt_io.go): the
// in-memory objects that translated code still constructs to hand text or bytes to a library
// (a StringReader or StringWriter for Gson's JsonReader and JsonWriter, a ByteArrayInputStream
// or StringReader for a SAX InputSource), OutputStream as the superclass of protobuf's
// ByteString.Output, and ObjectInputStream, which Java serialization would pass to
// JSChunk.readObject. They hold their content and do nothing else.

import "strings"

type Reader struct{ self_ any }

func (this *Reader) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

type StringReader struct {
	Reader
	chars_ []uint16
	next_  int
}

func new_StringReader(s string) *StringReader {
	if s == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	this := &StringReader{chars_: String_toCharArray(s)}
	this.self_ = this
	return this
}

func (this *StringReader) up_Reader() *Reader { return &this.Reader }

// read returns the next char, or -1 at the end.
func (this *StringReader) read() int32 {
	if this.next_ >= len(this.chars_) {
		return -1
	}
	this.next_++
	return int32(this.chars_[this.next_-1])
}

// rest_ is the text that has not been read.
func (this *StringReader) rest_() string { return fromUnits_(this.chars_[this.next_:]) }

type Writer struct{ self_ any }

func (this *Writer) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

type StringWriter struct {
	Writer
	text_ strings.Builder
}

func new_StringWriter() *StringWriter {
	this := &StringWriter{}
	this.self_ = this
	return this
}

func (this *StringWriter) up_Writer() *Writer { return &this.Writer }
func (this *StringWriter) write(s string)     { this.text_.WriteString(s) }
func (this *StringWriter) toString() string   { return this.text_.String() }

type InputStream struct {
	self_  any
	bytes_ []byte
}

type ByteArrayInputStream struct{ InputStream }

func new_ByteArrayInputStream(b []int8) *ByteArrayInputStream {
	this := &ByteArrayInputStream{InputStream{bytes_: u8_(b)}}
	this.self_ = this
	return this
}

func (this *ByteArrayInputStream) up_InputStream() *InputStream { return &this.InputStream }

// OutputStream is the superclass of protobuf's ByteString.Output (rt_proto.go).
type OutputStream struct{ self_ any }

func (this *OutputStream) write(b int32) { this.self_.(interface{ write(int32) }).write(b) }
func (this *OutputStream) flush()        {}

type ObjectInputStream struct{}

func (this *ObjectInputStream) defaultReadObject() {
	panic(new_UnsupportedOperationException_String("Java serialization is not supported").self())
}
