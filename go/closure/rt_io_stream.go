package closure

// The byte streams of java.io, and System.in / System.out / System.err.
//
// InputStream and OutputStream are the base structs of every stream; a
// subclass embeds one (directly or through FilterInputStream /
// FilterOutputStream). A stream written in Go supplies its behaviour as an
// io.Reader / io.Writer in the base struct (src_ / dst_); a stream that
// overrides the Java methods instead (translated code, or another part of the
// runtime) is reached through self_, like every overridable method.

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"syscall"
)

// ---- errors of the operating system

// osMessage_ is the text the JDK shows for an operating system error
// ("No such file or directory").
func osMessage_(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		m := errno.Error()
		if m != "" {
			return strings.ToUpper(m[:1]) + m[1:]
		}
	}
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}

// throwIO_ throws the IOException of a failed read or write.
func throwIO_(err error) {
	panic(new_IOException_String(osMessage_(err)).self())
}

// ---- java.io.InputStream

const (
	busyRead_ = 1 << iota
	busyReadArr_
	busyClose_
	busyWrite_
	busyWriteArr_
	busyFlush_
	busyAvailable_
)

type InputStream struct {
	self_   any
	src_    io.Reader    // where a stream implemented here reads from
	closer_ func()       // what closing it does
	avail_  func() int32 // available(), if known
	busy_   uint8        // the overridable methods being dispatched to self_
	closed_ bool
}

func (this *InputStream) as_InputStream() *InputStream { return this }

func to_InputStream(v any) *InputStream {
	if v == nil {
		return nil
	}
	return v.(interface{ as_InputStream() *InputStream }).as_InputStream()
}

func (this *InputStream) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

// read returns the next byte (0..255) or -1 at the end of the stream.
func (this *InputStream) read() int32 {
	if this.busy_&busyRead_ == 0 {
		if s, ok := this.self_.(interface{ read() int32 }); ok {
			this.busy_ |= busyRead_
			defer func() { this.busy_ &^= busyRead_ }()
			return s.read()
		}
	}
	if this.src_ == nil {
		panic(new_UnsupportedOperationException_String("InputStream.read() is not implemented").self())
	}
	var one [1]byte
	if this.fill_(one[:]) <= 0 {
		return -1
	}
	return int32(one[0])
}

// fill_ reads at least one byte from src_ into p; it returns -1 at the end.
func (this *InputStream) fill_(p []byte) int {
	if this.closed_ {
		panic(new_IOException_String("Stream closed").self())
	}
	for {
		n, err := this.src_.Read(p)
		if n > 0 {
			return n
		}
		if err == io.EOF {
			return -1
		}
		if err != nil {
			throwIO_(err)
		}
	}
}

func (this *InputStream) read_byteArr_int_int(b []int8, off int32, n int32) int32 {
	if this.busy_&busyReadArr_ == 0 {
		if s, ok := this.self_.(interface {
			read_byteArr_int_int([]int8, int32, int32) int32
		}); ok {
			this.busy_ |= busyReadArr_
			defer func() { this.busy_ &^= busyReadArr_ }()
			return s.read_byteArr_int_int(b, off, n)
		}
	}
	if off < 0 || n < 0 || int(off)+int(n) > len(b) {
		panic(new_IndexOutOfBoundsException_0().self())
	}
	if n == 0 {
		return 0
	}
	if this.src_ != nil {
		return int32(this.fill_(u8_(b[off : off+n])))
	}
	// A stream that only defines read().
	c := this.read()
	if c < 0 {
		return -1
	}
	b[off] = int8(c)
	var i int32 = 1
	for ; i < n; i++ {
		c = this.read()
		if c < 0 {
			break
		}
		b[off+i] = int8(c)
	}
	return i
}

func (this *InputStream) read_byteArr(b []int8) int32 {
	return this.read_byteArr_int_int(b, 0, int32(len(b)))
}

func (this *InputStream) readNBytes_byteArr_int_int(b []int8, off int32, n int32) int32 {
	var got int32
	for got < n {
		k := this.read_byteArr_int_int(b, off+got, n-got)
		if k < 0 {
			break
		}
		got += k
	}
	return got
}

func (this *InputStream) readAllBytes() []int8 { return i8_(readAllBytes_(this)) }

func (this *InputStream) skip(n int64) int64 {
	var done int64
	buf := make([]int8, 4096)
	for done < n {
		want := n - done
		if want > int64(len(buf)) {
			want = int64(len(buf))
		}
		k := this.read_byteArr_int_int(buf, 0, int32(want))
		if k < 0 {
			break
		}
		done += int64(k)
	}
	return done
}

func (this *InputStream) available() int32 {
	if this.busy_&busyAvailable_ == 0 {
		if s, ok := this.self_.(interface{ available() int32 }); ok {
			this.busy_ |= busyAvailable_
			defer func() { this.busy_ &^= busyAvailable_ }()
			return s.available()
		}
	}
	if this.avail_ != nil {
		return this.avail_()
	}
	return 0
}

func (this *InputStream) markSupported() bool { return false }

func (this *InputStream) mark(limit int32) {}

func (this *InputStream) reset() {
	panic(new_IOException_String("mark/reset not supported").self())
}

func (this *InputStream) transferTo(out *OutputStream) int64 {
	var total int64
	buf := make([]int8, 16384)
	for {
		n := this.read_byteArr_int_int(buf, 0, int32(len(buf)))
		if n < 0 {
			return total
		}
		out.write_byteArr_int_int(buf, 0, n)
		total += int64(n)
	}
}

func (this *InputStream) close() {
	if this.busy_&busyClose_ == 0 {
		if s, ok := this.self_.(interface{ close() }); ok {
			this.busy_ |= busyClose_
			defer func() { this.busy_ &^= busyClose_ }()
			s.close()
			return
		}
	}
	if this.closed_ {
		return
	}
	this.closed_ = true
	if this.closer_ != nil {
		this.closer_()
	}
}

// inReader_ presents an InputStream as an io.Reader.
type inReader_ struct{ in *InputStream }

func (r inReader_) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	n := r.in.read_byteArr_int_int(i8_(p), 0, int32(len(p)))
	if n < 0 {
		return 0, io.EOF
	}
	return int(n), nil
}

// readAllBytes_ reads a stream to its end.
func readAllBytes_(in *InputStream) []byte {
	if in == nil {
		panic(new_NullPointerException_0().self())
	}
	b, err := io.ReadAll(inReader_{in})
	if err != nil {
		throwIO_(err)
	}
	return b
}

// newInputStream_ is a plain InputStream that reads from r.
func newInputStream_(r io.Reader, closer func()) *InputStream {
	this := &InputStream{src_: r, closer_: closer}
	this.self_ = this
	return this
}

// ---- ByteArrayInputStream

type ByteArrayInputStream struct {
	InputStream
	r_ *bytes.Reader
}

func (this *ByteArrayInputStream) up_InputStream() *InputStream {
	if this == nil {
		return nil
	}
	return &this.InputStream
}

func new_ByteArrayInputStream(b []int8) *ByteArrayInputStream {
	this := &ByteArrayInputStream{r_: bytes.NewReader(u8_(b))}
	this.self_ = this
	this.src_ = this.r_
	this.avail_ = func() int32 { return int32(this.r_.Len()) }
	return this
}

// close has no effect on a ByteArrayInputStream.
func (this *ByteArrayInputStream) close() {}

// ---- FileInputStream

type FileInputStream struct {
	InputStream
	f_ *os.File
}

func (this *FileInputStream) up_InputStream() *InputStream {
	if this == nil {
		return nil
	}
	return &this.InputStream
}

func fileNotFound_(name string, err error) any {
	return new_FileNotFoundException_String(name + " (" + osMessage_(err) + ")").self()
}

func new_FileInputStream(name string) *FileInputStream {
	if name == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	f, err := os.Open(name)
	if err == nil {
		if st, e := f.Stat(); e == nil && st.IsDir() {
			f.Close()
			err = syscall.EISDIR
		}
	}
	if err != nil {
		panic(fileNotFound_(name, err))
	}
	this := &FileInputStream{f_: f}
	this.self_ = this
	this.src_ = f
	this.closer_ = func() { f.Close() }
	this.avail_ = func() int32 {
		st, e1 := f.Stat()
		pos, e2 := f.Seek(0, io.SeekCurrent)
		if e1 != nil || e2 != nil || st.Size() <= pos {
			return 0
		}
		if st.Size()-pos > 0x7fffffff {
			return 0x7fffffff
		}
		return int32(st.Size() - pos)
	}
	return this
}

func new_FileInputStream_File(f *File) *FileInputStream { return new_FileInputStream(f.path_) }

// ---- FilterInputStream, BufferedInputStream

type FilterInputStream struct {
	InputStream
	in_ *InputStream
}

func (this *FilterInputStream) as_FilterInputStream() *FilterInputStream { return this }

func (this *FilterInputStream) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *FilterInputStream) up_InputStream() *InputStream {
	if this == nil {
		return nil
	}
	return &this.InputStream
}

func (this *FilterInputStream) init_FilterInputStream(in *InputStream) {
	this.in_ = in
	if in != nil {
		this.src_ = inReader_{in}
		this.closer_ = in.close
		this.avail_ = in.available
	}
}

type BufferedInputStream struct {
	FilterInputStream
}

func new_BufferedInputStream(in *InputStream) *BufferedInputStream {
	return new_BufferedInputStream_InputStream_int(in, 8192)
}

func new_BufferedInputStream_InputStream_int(in *InputStream, size int32) *BufferedInputStream {
	if size <= 0 {
		panic(new_IllegalArgumentException_String("Buffer size <= 0").self())
	}
	this := &BufferedInputStream{}
	this.self_ = this
	this.init_FilterInputStream(in)
	br := bufio.NewReaderSize(inReader_{in}, int(size))
	this.src_ = br
	this.avail_ = func() int32 { return int32(br.Buffered()) + in.available() }
	return this
}

// ---- ObjectInputStream (Java serialization is not supported)

type ObjectInputStream struct {
	InputStream
}

// ---- java.io.OutputStream

type OutputStream struct {
	self_    any
	dst_     io.Writer // where a stream implemented here writes to
	flusher_ func()
	closer_  func()
	busy_    uint8
	closed_  bool
}

func (this *OutputStream) as_OutputStream() *OutputStream { return this }

func to_OutputStream(v any) *OutputStream {
	if v == nil {
		return nil
	}
	return v.(interface{ as_OutputStream() *OutputStream }).as_OutputStream()
}

func (this *OutputStream) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *OutputStream) put_(p []byte) {
	if this.closed_ {
		panic(new_IOException_String("Stream closed").self())
	}
	if _, err := this.dst_.Write(p); err != nil {
		throwIO_(err)
	}
}

// write writes the low eight bits of b.
func (this *OutputStream) write(b int32) {
	if this.busy_&busyWrite_ == 0 {
		if s, ok := this.self_.(interface{ write(int32) }); ok {
			this.busy_ |= busyWrite_
			defer func() { this.busy_ &^= busyWrite_ }()
			s.write(b)
			return
		}
	}
	if this.dst_ == nil {
		panic(new_UnsupportedOperationException_String("OutputStream.write(int) is not implemented").self())
	}
	one := [1]byte{byte(b)}
	this.put_(one[:])
}

func (this *OutputStream) write_byteArr_int_int(b []int8, off int32, n int32) {
	if this.busy_&busyWriteArr_ == 0 {
		if s, ok := this.self_.(interface {
			write_byteArr_int_int([]int8, int32, int32)
		}); ok {
			this.busy_ |= busyWriteArr_
			defer func() { this.busy_ &^= busyWriteArr_ }()
			s.write_byteArr_int_int(b, off, n)
			return
		}
	}
	if off < 0 || n < 0 || int(off)+int(n) > len(b) {
		panic(new_IndexOutOfBoundsException_0().self())
	}
	if this.dst_ != nil {
		if n > 0 {
			this.put_(u8_(b[off : off+n]))
		}
		return
	}
	// A stream that only defines write(int).
	for i := int32(0); i < n; i++ {
		this.write(int32(b[off+i]))
	}
}

func (this *OutputStream) write_byteArr(b []int8) {
	this.write_byteArr_int_int(b, 0, int32(len(b)))
}

func (this *OutputStream) flush() {
	if this.busy_&busyFlush_ == 0 {
		if s, ok := this.self_.(interface{ flush() }); ok {
			this.busy_ |= busyFlush_
			defer func() { this.busy_ &^= busyFlush_ }()
			s.flush()
			return
		}
	}
	if this.flusher_ != nil && !this.closed_ {
		this.flusher_()
	}
}

func (this *OutputStream) close() {
	if this.busy_&busyClose_ == 0 {
		if s, ok := this.self_.(interface{ close() }); ok {
			this.busy_ |= busyClose_
			defer func() { this.busy_ &^= busyClose_ }()
			s.close()
			return
		}
	}
	if this.closed_ {
		return
	}
	defer func() { this.closed_ = true }()
	if this.closer_ != nil {
		this.closer_()
	}
}

// writeBytes_ writes Go bytes to a stream.
func writeBytes_(out *OutputStream, p []byte) {
	if len(p) > 0 {
		out.write_byteArr_int_int(i8_(p), 0, int32(len(p)))
	}
}

// outWriter_ presents an OutputStream as an io.Writer.
type outWriter_ struct{ out *OutputStream }

func (w outWriter_) Write(p []byte) (int, error) {
	writeBytes_(w.out, p)
	return len(p), nil
}

// newOutputStream_ is a plain OutputStream that writes to w.
func newOutputStream_(w io.Writer, flusher func(), closer func()) *OutputStream {
	this := &OutputStream{dst_: w, flusher_: flusher, closer_: closer}
	this.self_ = this
	return this
}

// ---- ByteArrayOutputStream

type ByteArrayOutputStream struct {
	OutputStream
	buf_ bytes.Buffer
}

func (this *ByteArrayOutputStream) up_OutputStream() *OutputStream {
	if this == nil {
		return nil
	}
	return &this.OutputStream
}

func new_ByteArrayOutputStream() *ByteArrayOutputStream {
	this := &ByteArrayOutputStream{}
	this.self_ = this
	this.dst_ = &this.buf_
	return this
}

func (this *ByteArrayOutputStream) size() int32 { return int32(this.buf_.Len()) }

func (this *ByteArrayOutputStream) reset() { this.buf_.Reset() }

func (this *ByteArrayOutputStream) toByteArray() []int8 {
	return i8_(append([]byte(nil), this.buf_.Bytes()...))
}

func (this *ByteArrayOutputStream) toString_Charset(cs *Charset) string {
	s, _ := cs.decode_(this.buf_.Bytes())
	return s
}

func (this *ByteArrayOutputStream) toString() string {
	return this.toString_Charset(StandardCharsets_UTF_8)
}

func (this *ByteArrayOutputStream) writeTo(out *OutputStream) { writeBytes_(out, this.buf_.Bytes()) }

// close has no effect on a ByteArrayOutputStream.
func (this *ByteArrayOutputStream) close() {}

// ---- FileOutputStream

type FileOutputStream struct {
	OutputStream
	f_ *os.File
}

func (this *FileOutputStream) up_OutputStream() *OutputStream {
	if this == nil {
		return nil
	}
	return &this.OutputStream
}

func newFileOutputStream_(f *os.File) *FileOutputStream {
	this := &FileOutputStream{f_: f}
	this.self_ = this
	this.dst_ = f
	this.closer_ = func() {
		if err := f.Close(); err != nil {
			throwIO_(err)
		}
	}
	return this
}

func new_FileOutputStream_String_boolean(name string, appendTo bool) *FileOutputStream {
	if name == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if appendTo {
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}
	f, err := os.OpenFile(name, flags, 0o666)
	if err != nil {
		panic(fileNotFound_(name, err))
	}
	return newFileOutputStream_(f)
}

func new_FileOutputStream(name string) *FileOutputStream {
	return new_FileOutputStream_String_boolean(name, false)
}

// ---- FilterOutputStream, BufferedOutputStream

type FilterOutputStream struct {
	OutputStream
	out_ *OutputStream
}

func (this *FilterOutputStream) as_FilterOutputStream() *FilterOutputStream { return this }

func (this *FilterOutputStream) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *FilterOutputStream) up_OutputStream() *OutputStream {
	if this == nil {
		return nil
	}
	return &this.OutputStream
}

// init_FilterOutputStream makes the stream pass everything on to out. (The JDK
// forwards an array write one byte at a time; here it is passed on whole.)
func (this *FilterOutputStream) init_FilterOutputStream(out *OutputStream) {
	this.out_ = out
	if out == nil {
		return
	}
	this.dst_ = outWriter_{out}
	this.flusher_ = out.flush
	this.closer_ = func() {
		// close() flushes this stream (through an overriding flush, if any)
		// and then closes the underlying one.
		defer out.close()
		this.OutputStream.flush()
	}
}

type BufferedOutputStream struct {
	FilterOutputStream
}

func new_BufferedOutputStream(out *OutputStream) *BufferedOutputStream {
	return new_BufferedOutputStream_OutputStream_int(out, 8192)
}

func new_BufferedOutputStream_OutputStream_int(out *OutputStream, size int32) *BufferedOutputStream {
	if size <= 0 {
		panic(new_IllegalArgumentException_String("Buffer size <= 0").self())
	}
	this := &BufferedOutputStream{}
	this.self_ = this
	this.init_FilterOutputStream(out)
	bw := bufio.NewWriterSize(outWriter_{out}, int(size))
	this.dst_ = bw
	this.flusher_ = func() {
		if err := bw.Flush(); err != nil {
			throwIO_(err)
		}
		out.flush()
	}
	return this
}

// ---- PrintStream

type PrintStream struct {
	FilterOutputStream
	enc_       *encoder_
	autoFlush_ bool
	trouble_   bool
}

func (this *PrintStream) up_FilterOutputStream() *FilterOutputStream {
	if this == nil {
		return nil
	}
	return &this.FilterOutputStream
}

func newPrintStream_(out *OutputStream, autoFlush bool, cs *Charset) *PrintStream {
	if out == nil {
		panic(new_NullPointerException_String("Null output stream").self())
	}
	this := &PrintStream{enc_: newEncoder_(cs), autoFlush_: autoFlush}
	this.self_ = this
	this.init_FilterOutputStream(out)
	return this
}

func new_PrintStream(out *OutputStream) *PrintStream {
	return newPrintStream_(out, false, StandardCharsets_UTF_8)
}

func new_PrintStream_String(name string) *PrintStream {
	return newPrintStream_(new_FileOutputStream(name).up_OutputStream(), false, StandardCharsets_UTF_8)
}

// guard_ runs one output operation the way PrintStream does: an IOException
// is not thrown but remembered for checkError().
func (this *PrintStream) guard_(f func()) {
	defer func() {
		if r := recover(); r != nil {
			if is_IOException(r) {
				this.trouble_ = true
				return
			}
			panic(r)
		}
	}()
	if this.out_ == nil || this.closed_ {
		this.trouble_ = true
		return
	}
	f()
}

func (this *PrintStream) emit_(s string) {
	this.guard_(func() {
		if this.enc_.plain_(s) {
			writeBytes_(this.out_, strBytes_(s))
		} else {
			writeBytes_(this.out_, this.enc_.encode_(nil, s))
		}
		if this.autoFlush_ && strings.IndexByte(s, '\n') >= 0 {
			this.out_.flush()
		}
	})
}

func (this *PrintStream) write(b int32) {
	this.guard_(func() {
		this.out_.write(b)
		if this.autoFlush_ && byte(b) == '\n' {
			this.out_.flush()
		}
	})
}

func (this *PrintStream) write_byteArr_int_int(b []int8, off int32, n int32) {
	this.guard_(func() {
		this.out_.write_byteArr_int_int(b, off, n)
		if this.autoFlush_ {
			this.out_.flush()
		}
	})
}

func (this *PrintStream) flush() {
	this.guard_(func() { this.out_.flush() })
}

func (this *PrintStream) close() {
	if this.closed_ {
		return
	}
	this.guard_(func() {
		if b := this.enc_.finish_(nil); len(b) > 0 {
			writeBytes_(this.out_, b)
		}
		this.out_.close()
	})
	this.closed_ = true
}

func (this *PrintStream) checkError() bool {
	if !this.closed_ {
		this.flush()
	}
	return this.trouble_
}

func (this *PrintStream) print(s string)         { this.emit_(sstr_(s)) }
func (this *PrintStream) print_Object(v any)     { this.emit_(objstr_(v)) }
func (this *PrintStream) print_char(c uint16)    { this.emit_(String_valueOf_char(c)) }
func (this *PrintStream) print_int(i int32)      { this.emit_(istr_(int64(i))) }
func (this *PrintStream) print_long(i int64)     { this.emit_(istr_(i)) }
func (this *PrintStream) print_boolean(b bool)   { this.emit_(boolstr_(b)) }
func (this *PrintStream) print_double(d float64) { this.emit_(dstr_(d)) }

func (this *PrintStream) println(s string)         { this.emit_(sstr_(s) + "\n") }
func (this *PrintStream) println_0()               { this.emit_("\n") }
func (this *PrintStream) println_Object(v any)     { this.emit_(objstr_(v) + "\n") }
func (this *PrintStream) println_char(c uint16)    { this.emit_(String_valueOf_char(c) + "\n") }
func (this *PrintStream) println_int(i int32)      { this.emit_(istr_(int64(i)) + "\n") }
func (this *PrintStream) println_long(i int64)     { this.emit_(istr_(i) + "\n") }
func (this *PrintStream) println_boolean(b bool)   { this.emit_(boolstr_(b) + "\n") }
func (this *PrintStream) println_double(d float64) { this.emit_(dstr_(d) + "\n") }

func (this *PrintStream) format(f string, args []any) *PrintStream {
	this.emit_(String_format(f, args))
	return this
}

func (this *PrintStream) printf(f string, args []any) *PrintStream { return this.format(f, args) }

func (this *PrintStream) appendable_() Appendable {
	if a, ok := this.self_.(Appendable); ok {
		return a
	}
	return this
}

func (this *PrintStream) append_CharSequence(v any) Appendable {
	if v == nil {
		this.emit_("null")
	} else {
		this.emit_(csString_(v))
	}
	return this.appendable_()
}

func (this *PrintStream) append_CharSequence_int_int(v any, a int32, b int32) Appendable {
	s := "null"
	if v != nil {
		s = csString_(v)
	}
	this.emit_(String_substring(s, a, b))
	return this.appendable_()
}

func (this *PrintStream) append_char(c uint16) Appendable {
	this.emit_(String_valueOf_char(c))
	return this.appendable_()
}

// ---- System.in, System.out, System.err

type System struct{}

var stdoutBuf_ = bufio.NewWriterSize(os.Stdout, 65536)
var stderrBuf_ = bufio.NewWriterSize(os.Stderr, 8192)

func stdStream_(w *bufio.Writer, autoFlush bool) *PrintStream {
	flush := func() {
		if err := w.Flush(); err != nil {
			throwIO_(err)
		}
	}
	out := newOutputStream_(w, flush, flush)
	return newPrintStream_(out, autoFlush, StandardCharsets_UTF_8)
}

// System_out is fully buffered and flushed when the process exits; System_err
// is flushed at the end of every line.
var System_out = stdStream_(stdoutBuf_, false)
var System_err = stdStream_(stderrBuf_, true)
var System_in = newInputStream_(bufio.NewReaderSize(os.Stdin, 65536), nil)

func init() {
	flushStd_ = func() {
		System_out.flush()
		System_err.flush()
		stdoutBuf_.Flush()
		stderrBuf_.Flush()
	}
}

func (this *ObjectInputStream) defaultReadObject() {
	panic(new_UnsupportedOperationException_String("Java serialization is not supported").self())
}

func new_PrintStream_File(f *File) *PrintStream { return new_PrintStream_String(f.path_) }
