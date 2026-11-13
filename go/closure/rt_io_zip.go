package closure

// java.util.zip on Go's archive/zip and compress/flate.

import (
	"archive/zip"
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"strings"
)

func init() {
	javaNames_["ZipException"] = "java.util.zip.ZipException"
	javaNames_["EOFException"] = "java.io.EOFException"
}

// ZipException is java.util.zip.ZipException.
type ZipException struct{ IOException }

func new_ZipException_String(m string) *ZipException {
	this := &ZipException{}
	this.self_ = this
	this.init_IOException_String(m)
	return this
}

// EOFException is java.io.EOFException.
type EOFException struct{ IOException }

func new_EOFException_String(m string) *EOFException {
	this := &EOFException{}
	this.self_ = this
	this.init_IOException_String(m)
	return this
}

// inflateReader_ turns the errors of a decompressor into Java exceptions.
type inflateReader_ struct {
	r io.Reader
}

func (r inflateReader_) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if err != nil && err != io.EOF {
		panic(inflateError_(err))
	}
	return n, err
}

func inflateError_(err error) any {
	switch {
	case errors.Is(err, io.ErrUnexpectedEOF):
		return new_EOFException_String("Unexpected end of ZLIB input stream").self()
	case errors.Is(err, gzip.ErrChecksum):
		return new_ZipException_String("Corrupt GZIP trailer").self()
	case errors.Is(err, gzip.ErrHeader):
		return new_ZipException_String("Not in GZIP format").self()
	case errors.Is(err, zip.ErrChecksum):
		return new_ZipException_String("invalid entry CRC").self()
	}
	return new_ZipException_String(err.Error()).self()
}

// ---- ZipEntry

type ZipEntry struct {
	name_ string
	f_    *zip.File
	size_ int64
}

func new_ZipEntry(name string) *ZipEntry {
	if name == nullstr_ {
		panic(new_NullPointerException_String("name").self())
	}
	return &ZipEntry{name_: name, size_: -1}
}

func zipEntryOf_(f *zip.File, cs *Charset) *ZipEntry {
	name := f.Name
	if f.NonUTF8 && cs != nil {
		name, _ = cs.decode_([]byte(name))
	} else {
		name, _ = decodeUTF8_([]byte(name))
	}
	return &ZipEntry{name_: name, f_: f, size_: int64(f.UncompressedSize64)}
}

func (this *ZipEntry) getName() string   { return this.name_ }
func (this *ZipEntry) toString() string  { return this.name_ }
func (this *ZipEntry) hashCode() int32   { return String_hashCode(this.name_) }
func (this *ZipEntry) isDirectory() bool { return strings.HasSuffix(this.name_, "/") }

// getSize is the uncompressed size, or -1 if it is not known.
func (this *ZipEntry) getSize() int64 { return this.size_ }

func (this *ZipEntry) getCompressedSize() int64 {
	if this.f_ == nil {
		return -1
	}
	return int64(this.f_.CompressedSize64)
}

func (this *ZipEntry) getCrc() int64 {
	if this.f_ == nil {
		return -1
	}
	return int64(this.f_.CRC32)
}

func (this *ZipEntry) getTime() int64 {
	if this.f_ == nil {
		return -1
	}
	return this.f_.Modified.UnixMilli()
}

func (this *ZipEntry) getMethod() int32 {
	if this.f_ == nil {
		return -1
	}
	return int32(this.f_.Method)
}

// ---- ZipFile

type ZipFile struct {
	name_    string
	file_    *os.File
	r_       *zip.Reader
	entries_ map[string]*ZipEntry
	closed_  bool
}

func new_ZipFile(file *File) *ZipFile { return new_ZipFile_String(file.path_) }

func new_ZipFile_String(name string) *ZipFile {
	f, err := os.Open(name)
	if err != nil {
		panic(nioError_(name, err))
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		panic(nioError_(name, err))
	}
	r, err := zip.NewReader(f, st.Size())
	if err != nil {
		f.Close()
		if st.Size() == 0 {
			panic(new_ZipException_String("zip file is empty").self())
		}
		panic(new_ZipException_String("zip END header not found").self())
	}
	this := &ZipFile{name_: name, file_: f, r_: r, entries_: map[string]*ZipEntry{}}
	for _, zf := range r.File {
		e := zipEntryOf_(zf, nil)
		if _, dup := this.entries_[e.name_]; !dup {
			this.entries_[e.name_] = e
		}
	}
	return this
}

func (this *ZipFile) ensureOpen_() {
	if this.closed_ {
		panic(new_IllegalStateException_String("zip file closed").self())
	}
}

func (this *ZipFile) getName() string { return this.name_ }

func (this *ZipFile) size() int32 {
	this.ensureOpen_()
	return int32(len(this.r_.File))
}

// getEntry finds the entry with the name, or the directory entry "name/".
func (this *ZipFile) getEntry(name string) *ZipEntry {
	if name == nullstr_ {
		panic(new_NullPointerException_String("name").self())
	}
	this.ensureOpen_()
	if e := this.entries_[name]; e != nil {
		return e
	}
	if !strings.HasSuffix(name, "/") {
		return this.entries_[name+"/"]
	}
	return nil
}

// getInputStream reads the contents of an entry. Like the JDK it does not
// verify the checksum.
func (this *ZipFile) getInputStream(entry *ZipEntry) *InputStream {
	if entry == nil {
		panic(new_NullPointerException_String("entry").self())
	}
	this.ensureOpen_()
	e := this.entries_[entry.name_]
	if e == nil {
		return nil
	}
	raw, err := e.f_.OpenRaw()
	if err != nil {
		panic(new_ZipException_String(err.Error()).self())
	}
	switch e.f_.Method {
	case zip.Store:
		return newInputStream_(raw, nil)
	case zip.Deflate:
		fr := flate.NewReader(bufio.NewReader(raw))
		// The size in the directory bounds what is read, as it does in the JDK.
		return newInputStream_(io.LimitReader(inflateReader_{fr}, int64(e.f_.UncompressedSize64)), nil)
	}
	panic(new_ZipException_String("invalid compression method").self())
}

func (this *ZipFile) close() {
	if this.closed_ {
		return
	}
	this.closed_ = true
	this.file_.Close()
}

// ---- InflaterInputStream, ZipInputStream, GZIPInputStream

type InflaterInputStream struct {
	FilterInputStream
}

func (this *InflaterInputStream) up_FilterInputStream() *FilterInputStream {
	if this == nil {
		return nil
	}
	return &this.FilterInputStream
}

// ZipInputStream reads the archive from its stream when the first entry is
// asked for, and then serves the entries in the order of the archive.
type ZipInputStream struct {
	InflaterInputStream
	cs_      *Charset
	files_   []*zip.File
	loaded_  bool
	next_    int
	cur_     io.ReadCloser
	entryEOF bool
}

func (this *ZipInputStream) up_InflaterInputStream() *InflaterInputStream {
	if this == nil {
		return nil
	}
	return &this.InflaterInputStream
}

type zipEntryReader_ struct{ z *ZipInputStream }

func (r zipEntryReader_) Read(p []byte) (int, error) {
	z := r.z
	if z.cur_ == nil || z.entryEOF {
		return 0, io.EOF
	}
	n, err := z.cur_.Read(p)
	if err == io.EOF {
		z.entryEOF = true
		if n > 0 {
			err = nil
		}
	} else if err != nil {
		panic(inflateError_(err))
	}
	return n, err
}

func new_ZipInputStream_InputStream_Charset(in *InputStream, cs *Charset) *ZipInputStream {
	if in == nil {
		panic(new_NullPointerException_String("in is null").self())
	}
	if cs == nil {
		panic(new_NullPointerException_String("charset is null").self())
	}
	this := &ZipInputStream{cs_: cs}
	this.self_ = this
	this.init_FilterInputStream(in)
	this.src_ = zipEntryReader_{this}
	this.avail_ = func() int32 {
		if this.cur_ == nil || this.entryEOF {
			return 0
		}
		return 1
	}
	return this
}

func new_ZipInputStream(in *InputStream) *ZipInputStream {
	return new_ZipInputStream_InputStream_Charset(in, StandardCharsets_UTF_8)
}

func (this *ZipInputStream) closeEntry() {
	if this.closed_ {
		panic(new_IOException_String("Stream closed").self())
	}
	if this.cur_ != nil {
		this.cur_.Close()
		this.cur_ = nil
	}
	this.entryEOF = true
}

// getNextEntry moves to the next entry, or returns null after the last one
// (and for a stream that is not a zip archive).
func (this *ZipInputStream) getNextEntry() *ZipEntry {
	if this.closed_ {
		panic(new_IOException_String("Stream closed").self())
	}
	if !this.loaded_ {
		this.loaded_ = true
		data := readAllBytes_(this.in_)
		if r, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err == nil {
			this.files_ = r.File
		}
	}
	if this.cur_ != nil {
		this.cur_.Close()
		this.cur_ = nil
	}
	if this.next_ >= len(this.files_) {
		this.entryEOF = true
		return nil
	}
	f := this.files_[this.next_]
	this.next_++
	rc, err := f.Open()
	if err != nil {
		panic(new_ZipException_String(err.Error()).self())
	}
	this.cur_ = rc
	this.entryEOF = false
	return zipEntryOf_(f, this.cs_)
}

type GZIPInputStream struct {
	InflaterInputStream
}

func (this *GZIPInputStream) up_InflaterInputStream() *InflaterInputStream {
	if this == nil {
		return nil
	}
	return &this.InflaterInputStream
}

func new_GZIPInputStream(in *InputStream, size int32) *GZIPInputStream {
	if in == nil {
		panic(new_NullPointerException_0().self())
	}
	if size <= 0 {
		panic(new_IllegalArgumentException_String("buffer size <= 0").self())
	}
	this := &GZIPInputStream{}
	this.self_ = this
	this.init_FilterInputStream(in)
	zr, err := gzip.NewReader(bufio.NewReaderSize(inReader_{in}, int(size)))
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			panic(new_EOFException_String(nullstr_).self())
		}
		panic(new_ZipException_String("Not in GZIP format").self())
	}
	this.src_ = inflateReader_{zr}
	this.avail_ = nil
	return this
}

func new_GZIPInputStream_InputStream(in *InputStream) *GZIPInputStream {
	return new_GZIPInputStream(in, 512)
}

// ---- Deflater, DeflaterOutputStream, GZIPOutputStream

type Deflater struct {
	level_ int32
}

const Deflater_DEFAULT_COMPRESSION int32 = -1
const Deflater_BEST_SPEED int32 = 1
const Deflater_BEST_COMPRESSION int32 = 9
const Deflater_NO_COMPRESSION int32 = 0

func new_Deflater_int(level int32) *Deflater {
	d := &Deflater{level_: -1}
	d.setLevel(level)
	return d
}

func (this *Deflater) setLevel(level int32) {
	if (level < 0 || level > 9) && level != -1 {
		panic(new_IllegalArgumentException_String("invalid compression level").self())
	}
	this.level_ = level
}

type DeflaterOutputStream struct {
	FilterOutputStream
	def       *Deflater
	fw_       *flate.Writer
	crc_      uint32
	total_    uint32
	gzip_     bool
	finished_ bool
}

func (this *DeflaterOutputStream) up_FilterOutputStream() *FilterOutputStream {
	if this == nil {
		return nil
	}
	return &this.FilterOutputStream
}

// deflateWriter_ compresses what is written to the stream. The compressor is
// created at the first write, with the level the Deflater has by then.
type deflateWriter_ struct{ s *DeflaterOutputStream }

func (w deflateWriter_) Write(p []byte) (int, error) {
	s := w.s
	if s.finished_ {
		panic(new_IOException_String("write beyond end of stream").self())
	}
	s.crc_ = crc32.Update(s.crc_, crc32.IEEETable, p)
	s.total_ += uint32(len(p))
	return s.deflater_().Write(p)
}

func (this *DeflaterOutputStream) deflater_() *flate.Writer {
	if this.fw_ == nil {
		fw, err := flate.NewWriter(outWriter_{this.out_}, int(this.def.level_))
		if err != nil {
			panic(new_IllegalArgumentException_String(err.Error()).self())
		}
		this.fw_ = fw
	}
	return this.fw_
}

// finish writes the rest of the compressed data (and the GZIP trailer)
// without closing the underlying stream.
func (this *DeflaterOutputStream) finish() {
	if this.finished_ {
		return
	}
	if err := this.deflater_().Close(); err != nil {
		throwIO_(err)
	}
	this.finished_ = true
	if this.gzip_ {
		c, n := this.crc_, this.total_
		writeBytes_(this.out_, []byte{byte(c), byte(c >> 8), byte(c >> 16), byte(c >> 24), byte(n), byte(n >> 8), byte(n >> 16), byte(n >> 24)})
	}
}

func (this *DeflaterOutputStream) init_deflate_(out *OutputStream, gz bool) {
	if out == nil {
		panic(new_NullPointerException_0().self())
	}
	this.init_FilterOutputStream(out)
	this.def = &Deflater{level_: -1}
	this.gzip_ = gz
	this.dst_ = deflateWriter_{this}
	this.closer_ = func() {
		defer out.close()
		this.finish()
	}
}

type GZIPOutputStream struct {
	DeflaterOutputStream
}

func (this *GZIPOutputStream) as_GZIPOutputStream() *GZIPOutputStream { return this }

func to_GZIPOutputStream(v any) *GZIPOutputStream {
	if v == nil {
		return nil
	}
	return v.(interface {
		as_GZIPOutputStream() *GZIPOutputStream
	}).as_GZIPOutputStream()
}

func is_GZIPOutputStream(v any) bool {
	_, ok := v.(interface {
		as_GZIPOutputStream() *GZIPOutputStream
	})
	return ok
}

func (this *GZIPOutputStream) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *GZIPOutputStream) up_DeflaterOutputStream() *DeflaterOutputStream {
	if this == nil {
		return nil
	}
	return &this.DeflaterOutputStream
}

// init_GZIPOutputStream_OutputStream_int writes the GZIP header at once, as
// the JDK does: no file name, no time stamp, unknown operating system.
func (this *GZIPOutputStream) init_GZIPOutputStream_OutputStream_int(out *OutputStream, size int32) {
	if size <= 0 {
		panic(new_IllegalArgumentException_String("buffer size <= 0").self())
	}
	this.init_deflate_(out, true)
	writeBytes_(out, []byte{0x1f, 0x8b, 8, 0, 0, 0, 0, 0, 0, 0xff})
}

func new_GZIPOutputStream_OutputStream_int(out *OutputStream, size int32) *GZIPOutputStream {
	this := &GZIPOutputStream{}
	this.self_ = this
	this.init_GZIPOutputStream_OutputStream_int(out, size)
	return this
}

func new_GZIPOutputStream_OutputStream(out *OutputStream) *GZIPOutputStream {
	return new_GZIPOutputStream_OutputStream_int(out, 512)
}
