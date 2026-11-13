package closure

// com.google.javascript.jscomp.base.Io, the compiler's only door to the operating system, written
// by hand on Go's standard library (the Java class is listed in tools/j2go/handwritten.txt and is
// not translated). The doc comment of each member of
// java/src/com/google/javascript/jscomp/base/Io.java is its whole contract, error messages
// included; the comments here only say what is particular to this side. What is built on top
// (charsets, text files, text sinks, paths, globs, zip archives, URIs) is translated Java.
//
// go/closure/testdata/io holds what the Java class returns for a few thousand inputs
// (tools/iotable), and rt_io_test.go checks this file against it.

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/base64"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unicode/utf8"
	"unsafe"
)

func init() {
	// A failed write to standard output or standard error is ignored, a closed pipe included.
	signal.Ignore(syscall.SIGPIPE)
}

// u8_ views a Java byte[] as Go bytes, and i8_ Go bytes as a Java byte[] (no copy).
func u8_(b []int8) []byte {
	if len(b) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&b[0])), len(b))
}

func i8_(b []byte) []int8 {
	if len(b) == 0 {
		return []int8{}
	}
	return unsafe.Slice((*int8)(unsafe.Pointer(&b[0])), len(b))
}

// ioErr_ is the error Outcome of a failed operation: a plain IOException with the message.
func ioErr_(message string) *Outcome_base {
	return Outcome_base_err(new_IOException_String(message).as_Throwable())
}

// ---- UTF-8

// Io_utf8Bytes: a string here holds a surrogate that Java's string has on its own as three bytes
// (rt_string.go), so two halves that were put next to each other are joined first.
func Io_utf8Bytes(s string) []int8 {
	if strings.IndexByte(s, 0xED) < 0 {
		return i8_([]byte(s)) // no surrogate stored as three bytes
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		cp, size := decode_(s, i)
		i += size
		switch {
		case cp >= 0xD800 && cp < 0xDC00 && i < len(s):
			if low, lowSize := decode_(s, i); low >= 0xDC00 && low < 0xE000 {
				out = appendCP_(out, 0x10000+(cp-0xD800)<<10+(low-0xDC00))
				i += lowSize
				continue
			}
			out = append(out, '?')
		case cp >= 0xD800 && cp < 0xE000:
			out = append(out, '?')
		default:
			out = append(out, s[i-size:i]...)
		}
	}
	return i8_(out)
}

// Io_utf8String: Go's notion of a well-formed sequence is the contract's.
func Io_utf8String(bytes []int8) string {
	b := u8_(bytes)
	if utf8.Valid(b) {
		return string(b)
	}
	out := make([]byte, 0, len(b)+8)
	for i := 0; i < len(b); {
		if r, size := utf8.DecodeRune(b[i:]); r == utf8.RuneError && size == 1 {
			out = append(out, "�"...)
			i++
		} else {
			out = append(out, b[i:i+size]...)
			i += size
		}
	}
	return string(out)
}

func Io_utf8StringStrict(bytes []int8) *Optional {
	if b := u8_(bytes); utf8.Valid(b) {
		return Optional_of(string(b))
	}
	return Optional_empty()
}

// ---- Files and directories

func Io_readBytes(path string) *Outcome_base {
	if !Io_exists(path) {
		return ioErr_(path + ": No such file")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ioErr_(path + ": Cannot read file")
	}
	return Outcome_base_ok_Object(i8_(b))
}

func Io_writeBytes(path string, bytes []int8) *Outcome_base {
	if os.WriteFile(path, u8_(bytes), 0o666) != nil {
		return ioErr_(path + ": Cannot write file")
	}
	return Outcome_base_ok_0()
}

func Io_exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func Io_isDirectory(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func Io_lastModified(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.ModTime().UnixNano()
}

func Io_mkdirs(path string) { os.MkdirAll(path, 0o777) }

func Io_listFiles(directory string) List {
	if directory == "" {
		directory = "."
	}
	result := new_ArrayList_0()
	listFilesInto_(directory, "", nil, result)
	return result
}

// listFilesInto_ adds the files under the directory; around holds the real paths of the
// directories being listed, outermost first.
func listFilesInto_(directory string, prefix string, around []string, result *ArrayList) {
	real, err := filepath.EvalSymlinks(directory)
	if err == nil {
		real, err = filepath.Abs(real)
	}
	entries, listErr := os.ReadDir(directory)
	if err != nil || listErr != nil {
		return
	}
	for _, a := range around {
		if a == real {
			return
		}
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	sort.Slice(names, func(i, j int) bool { return String_compareTo(names[i], names[j]) < 0 })
	for _, name := range names {
		if entry := directory + "/" + name; Io_isDirectory(entry) {
			listFilesInto_(entry, prefix+name+"/", append(around, real), result)
		} else {
			result.add(any(prefix + name))
		}
	}
}

func Io_workingDirectory() string {
	cwd, _ := syscall.Getwd() // not os.Getwd, which may answer with a symbolic link from $PWD
	return cwd
}

// ---- Byte sinks and standard input

type Io_Sink struct {
	file   *os.File
	path   string // of a file; empty for a standard stream
	closed bool
	failed bool
}

// There is one sink for each standard stream, so that closing it closes it for everyone.
var stdout_ = &Io_Sink{file: os.Stdout}
var stderr_ = &Io_Sink{file: os.Stderr}

func Io_stdout() *Io_Sink { return stdout_ }
func Io_stderr() *Io_Sink { return stderr_ }

// printErr_ writes text to standard error, for the runtime's own messages.
func printErr_(text string) { stderr_.write(Io_utf8Bytes(text)) }

func Io_openFile(path string, appendTo bool) *Outcome_base {
	mode := os.O_TRUNC
	if appendTo {
		mode = os.O_APPEND
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|mode, 0o666)
	if err != nil {
		return ioErr_(path + ": Cannot write file")
	}
	return Outcome_base_ok_Object(&Io_Sink{file: file, path: path})
}

func (this *Io_Sink) write(bytes []int8) {
	if this.closed {
		return
	}
	if _, err := this.file.Write(u8_(bytes)); err != nil {
		this.failed = true
	}
}

func (this *Io_Sink) close() *Outcome_base {
	if this.closed {
		return Outcome_base_ok_0()
	}
	this.closed = true
	if this.file.Close() != nil {
		this.failed = true
	}
	if this.failed && this.path != "" {
		return ioErr_(this.path + ": Cannot write file")
	}
	return Outcome_base_ok_0()
}

type Io_Input struct{ file *os.File }

func Io_stdin() *Io_Input { return &Io_Input{os.Stdin} }

func (this *Io_Input) readBytes() *Outcome_base {
	b, err := io.ReadAll(this.file)
	if err != nil {
		return ioErr_("stdin: Cannot read file")
	}
	return Outcome_base_ok_Object(i8_(b))
}

// ---- Codecs

func Io_inflate(deflated []int8) *Outcome_base {
	data, err := io.ReadAll(flate.NewReader(bytes.NewReader(u8_(deflated))))
	if err != nil {
		return ioErr_("Malformed compressed data")
	}
	return Outcome_base_ok_Object(i8_(data))
}

// Io_gzip does not produce the bytes Java's zlib does; the contract leaves them to the platform.
func Io_gzip(data []int8) []int8 {
	var out bytes.Buffer
	w, _ := gzip.NewWriterLevel(&out, gzip.BestSpeed)
	w.Write(u8_(data))
	w.Close()
	return i8_(out.Bytes())
}

func Io_gunzip(gzipped []int8) *Outcome_base {
	r, err := gzip.NewReader(bytes.NewReader(u8_(gzipped)))
	if err != nil {
		return ioErr_("Malformed gzip data")
	}
	r.Multistream(false) // bytes after the member are not looked at
	data, err := io.ReadAll(r)
	if err != nil {
		return ioErr_("Malformed gzip data")
	}
	return Outcome_base_ok_Object(i8_(data))
}

func Io_base64Encode(bytes []int8) string { return base64.StdEncoding.EncodeToString(u8_(bytes)) }

func Io_base64Decode(text string) []int8 {
	// Go's decoder skips line breaks; the contract has no such exception.
	if !strings.ContainsAny(text, "\r\n") {
		if b, err := base64.RawStdEncoding.DecodeString(strings.TrimRight(text, "=")); err == nil {
			return i8_(b)
		}
	}
	panic(new_IllegalArgumentException_String("Malformed base64 text").self())
}

// ---- Files bundled with the compiler; the report of an unexpected failure

// Io_readResource reads from the files embedded in the binary (rt_io_resources.go).
func Io_readResource(name string) *Optional {
	if b, ok := resourceBytes_(name); ok {
		return Optional_of(i8_(b))
	}
	return Optional_empty()
}

// Io_stackTraceText is the throwable and its causes, as the JVM
// prints them. A throwable does not record where it was thrown, so there are no "at" lines
// (nor suppressed throwables: the runtime has no addSuppressed).
func Io_stackTraceText(t *Throwable) string {
	var b strings.Builder
	b.WriteString(throwableString_(t) + "\n")
	seen := map[*Throwable]bool{t: true}
	for c := t.cause_; c != nil; c = c.cause_ {
		if seen[c] {
			b.WriteString("Caused by: [CIRCULAR REFERENCE: " + throwableString_(c) + "]\n")
			break
		}
		seen[c] = true
		b.WriteString("Caused by: " + throwableString_(c) + "\n")
	}
	return b.String()
}
