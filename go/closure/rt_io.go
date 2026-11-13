package closure

// com.google.javascript.jscomp.base.Io, the compiler's only door to files, the console, zip and
// gzip, charsets, embedded resources and path/URI arithmetic, written by hand on Go's standard
// library (the Java class is listed in tools/j2go/handwritten.txt and is not translated).
//
// java/src/com/google/javascript/jscomp/base/Io.java is the specification: each function here
// does what the JDK call in the Java method of the same name does on Linux, including the class
// and message of the exception a failure produces. Expected failures come back as an Outcome;
// what Java throws unchecked is a panic carrying the exception object. Sections, as in Io.java:
// errors; charsets; whole files; glob; text sinks and source; zip and gzip; resources; paths;
// URIs; in-memory helpers. go/closure/testdata/io holds what Java returns for a few thousand
// inputs (tools/iotable), and rt_io_test.go checks this file against it.
//
// Known limits: only the charsets in charsets_ exist; file names are UTF-8 whatever the locale;
// gzip output is valid but not the bytes zlib produces; a corrupt deflate stream is reported in
// Go's words, not zlib's.

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"
	"unsafe"
)

// ---- Errors: the Java exception objects of failed operations

type FileNotFoundException struct{ IOException }
type EOFException struct{ IOException }
type ZipException struct{ IOException }
type FileSystemException struct{ IOException }
type NoSuchFileException struct{ FileSystemException }
type AccessDeniedException struct{ FileSystemException }
type CharacterCodingException struct{ IOException }
type MalformedInputException struct{ CharacterCodingException }
type UnmappableCharacterException struct{ CharacterCodingException }
type URISyntaxException struct{ Exception }
type InvalidPathException struct{ IllegalArgumentException }
type IllegalCharsetNameException struct{ IllegalArgumentException }
type UnsupportedCharsetException struct{ IllegalArgumentException }
type globSyntaxException_ struct{ IllegalArgumentException } // java.util.regex.PatternSyntaxException

func init() {
	for goName, javaName := range map[string]string{
		"FileNotFoundException": "java.io.", "EOFException": "java.io.", "ZipException": "java.util.zip.",
		"FileSystemException": "java.nio.file.", "NoSuchFileException": "java.nio.file.",
		"AccessDeniedException": "java.nio.file.", "InvalidPathException": "java.nio.file.",
		"CharacterCodingException": "java.nio.charset.", "MalformedInputException": "java.nio.charset.",
		"UnmappableCharacterException": "java.nio.charset.", "IllegalCharsetNameException": "java.nio.charset.",
		"UnsupportedCharsetException": "java.nio.charset.", "URISyntaxException": "java.net.",
	} {
		javaNames_[goName] = javaName + goName
	}
	javaNames_["globSyntaxException_"] = "java.util.regex.PatternSyntaxException"
	// The JVM ignores SIGPIPE: writing to a closed pipe is an error that System.out swallows.
	signal.Ignore(syscall.SIGPIPE)
}

// javaError_ turns e, a new exception struct, into the Java exception with that message.
func javaError_(e interface{ as_Throwable() *Throwable }, msg string) *Throwable {
	t := e.as_Throwable()
	t.self_, t.message_ = e, msg
	return t
}

// errnoText_ is the operating system's description of the error, as the JDK shows it
// ("No such file or directory").
func errnoText_(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		m := errno.Error()
		return strings.ToUpper(m[:1]) + m[1:]
	}
	return err.Error()
}

// fileError_ is what java.io.FileInputStream and FileOutputStream throw when they cannot open.
func fileError_(path string, err error) *Throwable {
	return javaError_(&FileNotFoundException{}, path+" ("+errnoText_(err)+")")
}

// ioError_ is the IOException of a failed read, write or close.
func ioError_(err error) *Throwable { return javaError_(&IOException{}, errnoText_(err)) }

// nioError_ is what java.nio.file throws for a failed operation on the path.
func nioError_(path string, err error) *Throwable {
	switch {
	case errors.Is(err, syscall.ENOENT):
		return javaError_(&NoSuchFileException{}, path)
	case errors.Is(err, syscall.EACCES):
		return javaError_(&AccessDeniedException{}, path)
	}
	return javaError_(&FileSystemException{}, path+": "+errnoText_(err))
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

// outcome_ is Outcome.err(failure) if there is a failure, else Outcome.ok(value).
func outcome_(value any, failure *Throwable) *Outcome_base {
	if failure != nil {
		return Outcome_base_err(failure)
	}
	return Outcome_base_ok_Object(value)
}

func listOfStrings_(items []string) *ArrayList {
	list := new_ArrayList_0()
	for _, s := range items {
		list.add(any(s))
	}
	return list
}

// ---- Charsets, by name

type charset_ struct {
	name    string
	kind    int
	aliases string // lower case, separated by spaces
}

const (
	csUTF8_ = iota
	csASCII_
	csLatin1_
	csCP1252_
	csUTF16_ // big endian unless a byte order mark says otherwise; writes the mark
	csUTF16BE_
	csUTF16LE_
)

// charsets_ are the charsets that exist here, with the names the JDK knows them by. The JDK has
// many more (Shift_JIS, ISO-8859-15, ...): for those isCharsetSupported is false.
var charsets_ = []*charset_{
	{"UTF-8", csUTF8_, "utf8 unicode-1-1-utf-8"},
	{"US-ASCII", csASCII_, "ascii iso646-us ansi_x3.4-1968 ansi_x3.4-1986 iso_646.irv:1991 646 iso-ir-6 us cp367 ibm367 csascii default ascii7 iso_646.irv:1983"},
	{"ISO-8859-1", csLatin1_, "iso8859_1 iso_8859_1 iso8859-1 8859_1 latin1 l1 iso_8859-1 iso_8859-1:1987 iso-ir-100 ibm819 ibm-819 cp819 819 csisolatin1"},
	{"windows-1252", csCP1252_, "cp1252 cp5348 ibm-1252 ibm1252"},
	{"UTF-16", csUTF16_, "utf_16 utf16 unicode unicodebig"},
	{"UTF-16BE", csUTF16BE_, "utf_16be iso-10646-ucs-2 x-utf-16be unicodebigunmarked"},
	{"UTF-16LE", csUTF16LE_, "utf_16le x-utf-16le unicodelittleunmarked"},
}

// cp1252High_ are the characters of windows-1252 bytes 0x80..0x9F (0: the byte is unassigned).
var cp1252High_ = [32]uint16{
	0x20AC, 0, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021, 0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0, 0x017D, 0,
	0, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014, 0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0, 0x017E, 0x0178,
}

var charsetByName_ = func() map[string]*charset_ {
	m := map[string]*charset_{}
	for _, cs := range charsets_ {
		m[strings.ToLower(cs.name)] = cs
		for _, alias := range strings.Fields(cs.aliases) {
			m[alias] = cs
		}
	}
	return m
}()

// findCharset_ is Charset.forName without the failure for an unknown name, which is nil here.
func findCharset_(name string) *charset_ {
	if name == nullstr_ {
		panic(new_IllegalArgumentException_String("Null charset name").self())
	}
	if cs := charsetByName_[strings.ToLower(name)]; cs != nil {
		return cs
	}
	for i, c := range name {
		if !(c < 0x80 && (unicode.IsLetter(c) || unicode.IsDigit(c)) || i > 0 && strings.ContainsRune("-+.:_", c)) {
			panic(javaError_(&IllegalCharsetNameException{}, name).self_)
		}
	}
	if name == "" {
		panic(javaError_(&IllegalCharsetNameException{}, name).self_)
	}
	return nil
}

func charset_of_(name string) *charset_ {
	cs := findCharset_(name)
	if cs == nil {
		panic(javaError_(&UnsupportedCharsetException{}, name).self_)
	}
	return cs
}

func Io_isCharsetSupported(charset string) bool { return findCharset_(charset) != nil }
func Io_charsetName(charset string) string      { return charset_of_(charset).name }

// byte1_ is the byte of the character in a single-byte charset.
func (cs *charset_) byte1_(cp int32) (byte, bool) {
	switch {
	case cp < 0x80, cs.kind == csLatin1_ && cp < 0x100, cs.kind == csCP1252_ && cp >= 0xA0 && cp < 0x100:
		return byte(cp), true
	case cs.kind == csCP1252_:
		for i, u := range cp1252High_ {
			if u != 0 && int32(u) == cp {
				return byte(0x80 + i), true
			}
		}
	}
	return 0, false
}

type Io_CharEncoder struct{ cs *charset_ }

func Io_newEncoder(charset string) *Io_CharEncoder { return &Io_CharEncoder{charset_of_(charset)} }

func (this *Io_CharEncoder) canEncode(c uint16) bool {
	if c >= 0xD800 && c < 0xE000 {
		return false // a surrogate on its own
	}
	_, ok := this.cs.byte1_(int32(c))
	return ok || this.cs.kind == csUTF8_ || this.cs.kind >= csUTF16_
}

// encoder_ turns text into bytes of a charset; what cannot be encoded becomes '?' (U+FFFD in
// UTF-16). A Go string holds a Java string's unpaired surrogate as three bytes (rt_string.go), so
// a pair can arrive in two halves: a high surrogate is held back until the next char is known.
type encoder_ struct {
	cs      *charset_
	pending int32 // the held-back high surrogate, or 0
	started bool  // UTF-16: the byte order mark is written
}

func isHigh_(cp int32) bool { return cp >= 0xD800 && cp < 0xDC00 }
func isLow_(cp int32) bool  { return cp >= 0xDC00 && cp < 0xE000 }

// encode appends the encoding of s to dst.
func (e *encoder_) encode(dst []byte, s string) []byte {
	plain := e.cs.kind == csUTF8_ && strings.IndexByte(s, 0xED) < 0 || e.cs.kind <= csCP1252_ && isASCII_(s)
	if plain && e.pending == 0 {
		return append(dst, s...) // nothing to convert: no surrogates stored as three bytes
	}
	for i := 0; i < len(s); {
		cp, size := decode_(s, i)
		i += size
		if hi := e.pending; hi != 0 {
			e.pending = 0
			if isLow_(cp) {
				dst = e.put(dst, 0x10000+(hi-0xD800)<<10+(cp-0xDC00))
				continue
			}
			dst = e.put(dst, -1)
		}
		if isHigh_(cp) {
			e.pending = cp
		} else {
			dst = e.put(dst, cp)
		}
	}
	return dst
}

// finish appends what the end of the text adds: the replacement of a held-back surrogate.
func (e *encoder_) finish(dst []byte) []byte {
	if e.pending != 0 {
		e.pending = 0
		dst = e.put(dst, -1)
	}
	return dst
}

// put appends one character; cp is -1 or a lone surrogate for what cannot be encoded.
func (e *encoder_) put(dst []byte, cp int32) []byte {
	bad := cp < 0 || isHigh_(cp) || isLow_(cp)
	if e.cs.kind >= csUTF16_ {
		if bad {
			cp = 0xFFFD
		}
		if e.cs.kind == csUTF16_ && !e.started {
			e.started = true
			dst = append(dst, 0xFE, 0xFF)
		}
		units := []uint16{uint16(cp)}
		if cp >= 0x10000 {
			units = []uint16{uint16(0xD800 + (cp-0x10000)>>10), uint16(0xDC00 + (cp-0x10000)&0x3FF)}
		}
		for _, u := range units {
			if e.cs.kind == csUTF16LE_ {
				dst = append(dst, byte(u), byte(u>>8))
			} else {
				dst = append(dst, byte(u>>8), byte(u))
			}
		}
		return dst
	}
	if e.cs.kind == csUTF8_ && !bad {
		return appendCP_(dst, cp)
	}
	if b, ok := e.cs.byte1_(cp); ok && !bad {
		return append(dst, b)
	}
	return append(dst, '?')
}

func (cs *charset_) encode(s string) []byte {
	e := encoder_{cs: cs}
	return e.finish(e.encode(make([]byte, 0, len(s)), s))
}

// decode is new String(bytes, charset): malformed input becomes U+FFFD. The second result is
// what a decoder that reports instead (Files.readString) throws for the first malformed input.
func (cs *charset_) decode(b []byte) (string, *Throwable) {
	var strict *Throwable
	malformed := func(offset int, length int) {
		if strict == nil {
			strict = javaError_(&MalformedInputException{}, "Input length = "+strconv.Itoa(length))
		}
	}
	switch cs.kind {
	case csUTF8_:
		return decodeUTF8_(b, malformed), strict
	case csUTF16_, csUTF16BE_, csUTF16LE_:
		return decodeUTF16_(b, cs.kind, malformed), strict
	}
	out := make([]byte, 0, len(b))
	for _, c := range b {
		cp := int32(c)
		switch {
		case c >= 0x80 && cs.kind == csASCII_:
			malformed(0, 1)
			cp = 0xFFFD
		case c >= 0x80 && c < 0xA0 && cs.kind == csCP1252_:
			if cp = int32(cp1252High_[c-0x80]); cp == 0 {
				if cp = 0xFFFD; strict == nil {
					strict = javaError_(&UnmappableCharacterException{}, "Input length = 1")
				}
			}
		}
		out = appendCP_(out, cp)
	}
	return string(out), strict
}

// decodeUTF8_ replaces as the JDK does: a lead byte with the continuation bytes that may follow
// it (an encoded surrogate counts as a whole) is one U+FFFD, every other stray byte another.
// malformed is told about each, with the length that a reporting JDK decoder names.
func decodeUTF8_(b []byte, malformed func(offset int, length int)) string {
	if utf8.Valid(b) {
		return string(b)
	}
	out := make([]byte, 0, len(b)+16)
	for i := 0; i < len(b); {
		if r, size := utf8.DecodeRune(b[i:]); r != utf8.RuneError || size == 3 {
			out = append(out, b[i:i+size]...)
			i += size
			continue
		}
		c := b[i]
		need, lo, hi := 0, byte(0x80), byte(0xBF) // continuation bytes, and the range of the first
		switch {
		case c >= 0xC2 && c <= 0xDF:
			need = 1
		case c == 0xE0:
			need, lo = 2, 0xA0
		case c >= 0xE1 && c <= 0xEF:
			need = 2
		case c == 0xF0:
			need, lo = 3, 0x90
		case c >= 0xF1 && c <= 0xF3:
			need = 3
		case c == 0xF4:
			need, hi = 3, 0x8F
		}
		n := 1 // the lead and as many of its continuation bytes as are right
		for n <= need && i+n < len(b) && b[i+n] >= lo && b[i+n] <= hi {
			n, lo, hi = n+1, 0x80, 0xBF
		}
		rest := len(b) - i - 1
		switch {
		case c >= 0xE0 && c <= 0xEF && rest >= 2:
			malformed(i, 3)
		case c >= 0xE0 && c <= 0xEF && rest == 1 && n == 1:
			malformed(i, 2)
		case c >= 0xF0 && c <= 0xF7 && rest >= 3:
			malformed(i, 4)
		default:
			malformed(i, 1)
		}
		out = append(out, 0xEF, 0xBF, 0xBD)
		i += n
	}
	return string(out)
}

func decodeUTF16_(b []byte, kind int, malformed func(offset int, length int)) string {
	order := binary.ByteOrder(binary.BigEndian)
	if kind == csUTF16LE_ {
		order = binary.LittleEndian
	} else if kind == csUTF16_ && len(b) >= 2 && (b[0] == 0xFE && b[1] == 0xFF || b[0] == 0xFF && b[1] == 0xFE) {
		if b[0] == 0xFF {
			order = binary.LittleEndian
		}
		b = b[2:]
	}
	units := make([]uint16, 0, len(b)/2+1)
	for len(b) > 0 {
		u := uint16(0xFFFD)
		switch {
		case len(b) == 1:
			malformed(0, 1)
			b = nil
		case isLow_(int32(order.Uint16(b))):
			malformed(0, 2)
			b = b[2:]
		case isHigh_(int32(order.Uint16(b))) && len(b) >= 4 && isLow_(int32(order.Uint16(b[2:]))):
			units = append(units, order.Uint16(b))
			u, b = order.Uint16(b[2:]), b[4:]
		case isHigh_(int32(order.Uint16(b))): // without its pair: the next unit goes with it
			malformed(0, min(len(b), 4))
			b = b[min(len(b), 4):]
		default:
			u, b = order.Uint16(b), b[2:]
		}
		units = append(units, u)
	}
	return fromUnits_(units)
}

func Io_utf8Bytes(s string) []int8  { return i8_(charsets_[csUTF8_].encode(s)) }
func Io_utf8String(b []int8) string { return decodeUTF8_(u8_(b), func(int, int) {}) }

// ---- Whole files and file facts

// filePath_ is new File(path).getPath(): repeated and trailing slashes are dropped.
func filePath_(path string) string {
	if path == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/")
	}
	return path
}

// openFile_ is new FileInputStream(path) or new FileOutputStream(path, append).
func openFile_(path string, flags int) (*os.File, *Throwable) {
	path = filePath_(path)
	if strings.IndexByte(path, 0) >= 0 {
		return nil, javaError_(&FileNotFoundException{}, "Invalid file path")
	}
	f, err := os.OpenFile(path, flags, 0o666)
	if err == nil && flags == os.O_RDONLY {
		if st, e := f.Stat(); e == nil && st.IsDir() {
			f.Close()
			err = syscall.EISDIR
		}
	}
	if err != nil {
		return nil, fileError_(path, err)
	}
	return f, nil
}

func readFile_(path string) ([]byte, *Throwable) {
	f, failure := openFile_(path, os.O_RDONLY)
	if failure != nil {
		return nil, failure
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, ioError_(err)
	}
	return b, nil
}

func Io_readBytes(path string) *Outcome_base {
	b, failure := readFile_(path)
	return outcome_(i8_(b), failure)
}

func Io_readText(path string, charset string) *Outcome_base {
	b, failure := readFile_(path)
	if failure != nil {
		return Outcome_base_err(failure)
	}
	s, _ := charset_of_(charset).decode(b)
	return outcome_(s, nil)
}

// Io_readTextStrict is Files.readString: java.nio.file's errors, and malformed input is one.
func Io_readTextStrict(path string, charset string) *Outcome_base {
	path = nioPath_(path)
	cs := charset_of_(charset)
	b, err := os.ReadFile(sysPath_(path))
	var pe *os.PathError
	if errors.As(err, &pe) && pe.Op == "read" {
		return Outcome_base_err(ioError_(err)) // a directory opens; reading it fails
	} else if err != nil {
		return Outcome_base_err(nioError_(path, err))
	}
	return outcome_(cs.decode(b))
}

func Io_isDecodingError(error_ *Throwable) bool {
	if error_ == nil {
		return false
	}
	switch error_.self_.(type) {
	case *CharacterCodingException, *MalformedInputException, *UnmappableCharacterException:
		return true
	}
	return false
}

func Io_writeText(path string, charset string, text string) *Outcome_base {
	return Io_writeBytes(path, i8_(charset_of_(charset).encode(text)))
}

func Io_writeBytes(path string, b []int8) *Outcome_base {
	f, failure := openFile_(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if failure != nil {
		return Outcome_base_err(failure)
	}
	_, err := f.Write(u8_(b))
	if e := f.Close(); err == nil {
		err = e
	}
	if err != nil {
		return Outcome_base_err(ioError_(err))
	}
	return Outcome_base_ok_0()
}

func Io_exists(path string) bool {
	_, err := os.Stat(filePath_(path))
	return err == nil
}

func Io_isDirectory(path string) bool {
	st, err := os.Stat(filePath_(path))
	return err == nil && st.IsDir()
}

// Io_mkdirs is File.mkdirs: true only if this call created the directory itself. When the plain
// mkdir fails, Java retries on the canonical path ("x/../a/b" does not create x).
func Io_mkdirs(path string) bool {
	path = filePath_(path)
	if Io_exists(path) {
		return false
	}
	if os.Mkdir(path, 0o777) == nil {
		return true
	}
	canonical := filepath.Clean(Io_pathToAbsolute(path))
	os.MkdirAll(filepath.Dir(canonical), 0o777)
	return os.Mkdir(canonical, 0o777) == nil
}

func Io_createNewFile(path string) *Outcome_base {
	f, err := os.OpenFile(filePath_(path), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o666)
	if errors.Is(err, syscall.EEXIST) {
		return Outcome_base_ok_Object(false)
	} else if err != nil {
		return Outcome_base_err(ioError_(err))
	}
	f.Close()
	return Outcome_base_ok_Object(true)
}

// dirNames_ lists a directory in the order the operating system returns the entries, which is
// the order the JDK iterates in (os.ReadDir would sort them).
func dirNames_(dir string) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Readdirnames(-1)
}

func Io_listDirectory(path string) List {
	path = filePath_(path)
	names, err := dirNames_(path)
	if err != nil {
		return nil
	}
	for i, n := range names {
		names[i] = strings.TrimSuffix(path, "/") + "/" + n
	}
	return listOfStrings_(names)
}

// ---- listFilesMatchingGlob: Files.walkFileTree and the default file system's glob matcher

func Io_listFilesMatchingGlob(dir string, glob string) *Outcome_base {
	matcher := globMatcher_(glob)
	var result []string
	var ancestors []os.FileInfo // the directories being walked, to notice a link back into one
	var walk func(path string)
	walk = func(path string) {
		st, err := os.Stat(sysPath_(path))
		if err != nil {
			st, err = os.Lstat(sysPath_(path)) // a link to nowhere is visited as a file
		}
		if err != nil {
			return // what cannot be visited is skipped
		}
		if !st.IsDir() {
			if matcher.MatchString(path) || matcher.MatchString(Io_pathNormalize(path)) {
				result = append(result, path)
			}
			return
		}
		for _, a := range ancestors {
			if os.SameFile(a, st) {
				return
			}
		}
		// A directory that cannot be listed is skipped. (Java skips one it cannot open, but an
		// error in the middle of a listing ends its walk with that error.)
		names, _ := dirNames_(sysPath_(path))
		ancestors = append(ancestors, st)
		for _, n := range names {
			walk(Io_pathResolve(path, n))
		}
		ancestors = ancestors[:len(ancestors)-1]
	}
	walk(nioPath_(dir))
	return Outcome_base_ok_Object(listOfStrings_(result))
}

// globMatcher_ is getPathMatcher("glob:" + glob) on Unix: the JDK's rewriting of the glob as a
// regular expression, error messages included, then that expression in Go's syntax.
func globMatcher_(glob string) *regexp.Regexp {
	fail := func(what string, index int, text string) {
		msg := what + " near index " + strconv.Itoa(index) + "\n" + text
		if index < int(String_length(text)) {
			msg += "\n" + strings.Repeat(" ", index) + "^"
		}
		panic(javaError_(&globSyntaxException_{}, msg).self_)
	}
	g := String_toCharArray(glob)
	next := func(i int) uint16 {
		if i < len(g) {
			return g[i]
		}
		return 0
	}
	re := []uint16{'^'}
	add := func(text string) { re = append(re, String_toCharArray(text)...) }
	literal := func(c uint16, meta string) {
		if c < 0x80 && strings.IndexByte(meta, byte(c)) >= 0 {
			re = append(re, '\\')
		}
		re = append(re, c)
	}
	inGroup, emptyClass := false, false
	i := 0
	for i < len(g) {
		c := g[i]
		i++
		switch c {
		case '\\':
			if i == len(g) {
				fail("No character to escape", i-1, glob)
			}
			literal(g[i], `\*?[{.^$+]|()`)
			i++
		case '[':
			add("[[^/]&&[") // a class never matches the separator
			if next(i) == '^' {
				add(`\^`) // not a negation in a glob
				i++
			} else {
				if next(i) == '!' {
					add("^")
					i++
				}
				emptyClass = emptyClass || next(i) == ']'
				if next(i) == '-' {
					add("-")
					i++
				}
			}
			hasRangeStart, last := false, uint16(0)
			for i < len(g) {
				c = g[i]
				i++
				if c == ']' {
					break
				}
				if c == '/' {
					fail("Explicit 'name separator' in class", i-1, glob)
				}
				if c == '\\' || c == '[' || c == '&' && next(i) == '&' {
					re = append(re, '\\')
				}
				re = append(re, c)
				if c != '-' {
					hasRangeStart, last = true, c
					continue
				}
				if !hasRangeStart {
					fail("Invalid range", i-1, glob)
				}
				c = next(i)
				i++
				if c == 0 || c == ']' {
					break
				}
				if c < last {
					fail("Invalid range", i-3, glob)
				}
				re = append(re, c)
				hasRangeStart = false
			}
			if c != ']' {
				fail("Missing ']", i-1, glob)
			}
			add("]]")
		case '{':
			if inGroup {
				fail("Cannot nest groups", i-1, glob)
			}
			add("(?:(?:")
			inGroup = true
		case '}':
			if inGroup {
				add("))")
			} else {
				add("}")
			}
			inGroup = false
		case ',':
			if inGroup {
				add(")|(?:")
			} else {
				add(",")
			}
		case '*':
			if next(i) == '*' {
				add(".*")
				i++
			} else {
				add("[^/]*")
			}
		case '?':
			add("[^/]")
		default:
			literal(c, `.^$+{[]|()`)
		}
	}
	if inGroup {
		fail("Missing '}", i-1, glob)
	}
	java := fromUnits_(append(re, '$'))
	// The same in Go: no class intersection (and none needed, '/' in a class having been refused);
	// Java's dot, which only "**" produces unescaped, stops at line ends.
	compiled, err := regexp.Compile(strings.NewReplacer("[[^/]&&[^", "[^/", "[[^/]&&[", "[", "]]", "]",
		`\\`, `\\`, `\.`, `\.`, ".*", `[^\n\r\x{85}\x{2028}\x{2029}]*`).Replace(java))
	if emptyClass || err != nil {
		// A class that starts with ']' does not end there for Java's regular expressions; this
		// reports the usual outcome (the JDK accepts a few such globs, with odd meanings).
		fail("Unclosed character class", int(String_length(java))-1, java)
	}
	return compiled
}

// ---- Text sinks and the text source

// console_ is one of the process's two output streams, as System.out and System.err are: text
// is encoded for the locale and written at once, a failed write is forgotten, and writing after
// close does nothing.
type console_ struct {
	file   *os.File
	enc    encoder_
	closed bool
}

// localeCharset_ is the charset the JDK gives System.out and System.err (stdout.encoding): the
// codeset of the locale in the environment, US-ASCII in the C locale.
func localeCharset_() *charset_ {
	for _, v := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if locale := os.Getenv(v); locale != "" {
			locale, _, _ = strings.Cut(locale, "@")
			if _, codeset, found := strings.Cut(locale, "."); found {
				if cs := charsetByName_[strings.ToLower(codeset)]; cs != nil {
					return cs
				}
				return charsets_[csUTF8_]
			}
			break
		}
	}
	return charsets_[csASCII_]
}

var stdout_ = &console_{file: os.Stdout, enc: encoder_{cs: localeCharset_()}}
var stderr_ = &console_{file: os.Stderr, enc: encoder_{cs: localeCharset_()}}

func (c *console_) write(p []byte) {
	if !c.closed {
		c.file.Write(p)
	}
}

// writerBuffer_ is the size of both buffers of a Java BufferedWriter(OutputStreamWriter).
const writerBuffer_ = 8192

// Io_Output is a text sink. There are three kinds: the console itself (Io_stdout, Io_stderr);
// a writer, which encodes with its own charset into a file or into the console's bytes; and the
// sink that drops everything (Io_nullOutput). A writer buffers as its Java counterpart does, so
// that what has reached the file or the console at any moment is the same: text waits until
// 8192 chars have gathered, their bytes until they would overflow 8192, or for flush or close.
type Io_Output struct {
	console *console_ // where the console kinds print; nil for a file writer and the null sink
	file    *os.File  // where a file writer writes
	enc     *encoder_ // non-nil for a writer
	chars   []byte    // writer: text not encoded yet
	units   int       //   and its length in UTF-16 units
	bytes   []byte    // writer: bytes not written yet
	closed  bool      // writer: closed, so that every further use is an error
	failure *Throwable
}

func Io_stdout() *Io_Output     { return &Io_Output{console: stdout_} }
func Io_stderr() *Io_Output     { return &Io_Output{console: stderr_} }
func Io_nullOutput() *Io_Output { return &Io_Output{} }

func Io_openOutput(path string, charset string, appendTo bool) *Outcome_base {
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if appendTo {
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}
	f, failure := openFile_(path, flags)
	return outcome_(&Io_Output{file: f, enc: &encoder_{cs: charset_of_(charset)}}, failure)
}

func (this *Io_Output) encodedAs(charset string) *Io_Output {
	if this.console == nil || this.enc != nil {
		panic(new_NullPointerException_0().self()) // Java: only a stream sink has bytes to write to
	}
	return &Io_Output{console: this.console, enc: &encoder_{cs: charset_of_(charset)}}
}

func (this *Io_Output) fail(failure *Throwable) {
	if this.failure == nil {
		this.failure = failure
	}
}

func (this *Io_Output) print(s string) {
	s = sstr_(s)
	switch {
	case this.enc == nil && this.console != nil:
		this.console.write(this.console.enc.encode(nil, s))
	case this.enc == nil:
	case this.closed:
		this.fail(javaError_(&IOException{}, "Stream closed"))
	default:
		from := 0 // s[from:i] is not in the buffer yet
		for i := 0; i < len(s); {
			cp, size := decode_(s, i)
			if cp >= 0x10000 && this.units == writerBuffer_-1 {
				// A surrogate pair across the end of the buffer: Java encodes the buffer without
				// it, and its second half is the first char of the next buffer.
				this.chars = append(this.chars, s[from:i]...)
				this.encodeChars()
				from = i
			}
			i += size
			this.units++
			if cp >= 0x10000 {
				this.units++
			}
			if this.units == writerBuffer_ {
				this.chars = append(this.chars, s[from:i]...)
				this.encodeChars()
				from = i
			}
		}
		this.chars = append(this.chars, s[from:]...)
	}
}

// encodeChars moves the waiting text into the byte buffer. When the next character does not
// fit there, the buffer is written out first.
func (this *Io_Output) encodeChars() {
	p := this.enc.encode(nil, string(this.chars))
	this.chars, this.units = this.chars[:0], 0
	for len(p) > 0 {
		n := min(len(p), writerBuffer_-len(this.bytes))
		for n > 0 && n < len(p) && (this.enc.cs.kind == csUTF8_ && p[n]&0xC0 == 0x80 || this.enc.cs.kind >= csUTF16_ && n%2 == 1) {
			n-- // the bytes of a character stay together
		}
		if n == 0 {
			this.writeBytes()
		}
		this.bytes, p = append(this.bytes, p[:n]...), p[n:]
	}
}

func (this *Io_Output) writeBytes() {
	if this.console != nil {
		this.console.write(this.bytes)
	} else if _, err := this.file.Write(this.bytes); err != nil {
		this.fail(ioError_(err))
	}
	this.bytes = this.bytes[:0]
}

func (this *Io_Output) println(s string) { this.print(sstr_(s) + "\n") }

func (this *Io_Output) printStackTrace(t *Throwable) { this.print(stackTraceText_(t)) }

func (this *Io_Output) append_CharSequence(s any) Appendable {
	this.print(objstr_(s))
	return this
}

func (this *Io_Output) append_CharSequence_int_int(s any, start int32, end int32) Appendable {
	this.print(String_substring(objstr_(s), start, end))
	return this
}

func (this *Io_Output) append_char(c uint16) Appendable {
	this.print(String_valueOf_char(c))
	return this
}

func (this *Io_Output) flush() {
	if this.closed {
		this.fail(javaError_(&IOException{}, "Stream closed"))
	} else if this.enc != nil {
		this.encodeChars()
		this.writeBytes()
	}
}

func (this *Io_Output) close() *Outcome_base {
	if this.enc != nil && !this.closed {
		this.encodeChars()
		this.bytes = this.enc.finish(this.bytes)
		this.writeBytes()
		this.closed = true
		if this.file != nil {
			if err := this.file.Close(); err != nil {
				this.fail(ioError_(err))
			}
		}
	}
	if this.console != nil && !this.console.closed {
		this.console.closed = true
		this.console.file.Close()
	}
	return outcome_(nil, this.failure)
}

// Io_Input is standard input.
type Io_Input struct{ file *os.File }

func Io_stdin() *Io_Input { return &Io_Input{os.Stdin} }

func (this *Io_Input) readText(charset string) *Outcome_base {
	b, err := io.ReadAll(this.file)
	if err != nil {
		return Outcome_base_err(ioError_(err))
	}
	s, _ := charset_of_(charset).decode(b)
	return outcome_(s, nil)
}

// ---- Zip and gzip

type Io_ZipEntryData struct {
	name_ string
	data_ []int8
}

func (this *Io_ZipEntryData) name() string { return this.name_ }
func (this *Io_ZipEntryData) data() []int8 { return this.data_ }

// readZip_ reads an archive the way java.util.zip.ZipInputStream does: front to back, entry by
// entry from the local headers, never looking at the central directory, and stopping quietly at
// the first thing that is not a local header (so a file that is no zip has no entries).
func readZip_(archive []byte, cs *charset_) ([]*Io_ZipEntryData, *Throwable) {
	le := binary.LittleEndian
	r := bytes.NewReader(archive)
	zipError := func(msg string) *Throwable { return javaError_(&ZipException{}, msg) }
	var entries []*Io_ZipEntryData
	for {
		var h [30]byte // signature, version, flags, method, time, date, crc, sizes, name and extra lengths
		if _, err := io.ReadFull(r, h[:]); err != nil || le.Uint32(h[:]) != 0x04034b50 {
			return entries, nil
		}
		flags, method := le.Uint16(h[6:]), le.Uint16(h[8:])
		crc, csize, size := le.Uint32(h[14:]), int64(le.Uint32(h[18:])), int64(le.Uint32(h[22:]))
		name := make([]byte, int(le.Uint16(h[26:]))+int(le.Uint16(h[28:])))
		if _, err := io.ReadFull(r, name); err != nil {
			return nil, javaError_(&EOFException{}, nullstr_)
		}
		name = name[:le.Uint16(h[26:])] // the rest is the extra field
		text, _ := cs.decode(name)
		if flags&0x800 != 0 || cs.kind == csUTF8_ { // a flag says that the name is UTF-8
			text = decodeUTF8_(name, func(offset int, length int) {
				panic(new_IllegalArgumentException_String("malformed input off : " + strconv.Itoa(offset) + ", length : " + strconv.Itoa(length)).self())
			})
		}
		described := flags&8 != 0 // crc and sizes follow the data instead
		switch {
		case flags&1 != 0:
			return nil, zipError("encrypted ZIP entry not supported")
		case described && method != zip.Deflate:
			return nil, zipError("only DEFLATED entries can have EXT descriptor")
		}
		var data []byte
		start := r.Len()
		switch method {
		case zip.Store:
			data = make([]byte, min(size, int64(r.Len())))
			r.Read(data)
			if int64(len(data)) < size {
				return nil, zipError("unexpected EOF")
			}
		case zip.Deflate:
			var err error
			if data, err = io.ReadAll(flate.NewReader(r)); errors.Is(err, io.ErrUnexpectedEOF) {
				return nil, javaError_(&EOFException{}, "Unexpected end of ZLIB input stream")
			} else if err != nil {
				return nil, zipError(err.Error())
			}
			read := int64(start - r.Len())
			if described {
				var d [16]byte // [signature,] crc, compressed size, size
				if _, err := io.ReadFull(r, d[:]); err != nil {
					return nil, javaError_(&EOFException{}, nullstr_)
				}
				f := d[4:]
				if le.Uint32(d[:]) != 0x08074b50 {
					f = d[:]
					r.Seek(-4, io.SeekCurrent)
				}
				crc, csize, size = le.Uint32(f), int64(le.Uint32(f[4:])), int64(le.Uint32(f[8:]))
			}
			if size != int64(len(data)) {
				return nil, zipError("invalid entry size (expected " + istr_(size) + " but got " + istr_(int64(len(data))) + " bytes)")
			}
			if csize != read {
				return nil, zipError("invalid entry compressed size (expected " + istr_(csize) + " but got " + istr_(read) + " bytes)")
			}
		default:
			return nil, zipError("invalid compression method")
		}
		if sum := crc32.ChecksumIEEE(data); sum != crc {
			return nil, zipError("invalid entry CRC (expected 0x" + strconv.FormatUint(uint64(crc), 16) + " but got 0x" + strconv.FormatUint(uint64(sum), 16) + ")")
		}
		entries = append(entries, &Io_ZipEntryData{text, i8_(data)})
	}
}

func Io_listZipEntryNames(zipPath string, charset string) *Outcome_base {
	archive, failure := readFile_(zipPath)
	if failure != nil {
		return Outcome_base_err(failure)
	}
	entries, failure := readZip_(archive, charset_of_(charset))
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name_
	}
	return outcome_(listOfStrings_(names), failure)
}

func Io_readZipEntries(zipBytes []int8, charset string) *Outcome_base {
	entries, failure := readZip_(u8_(zipBytes), charset_of_(charset))
	list := new_ArrayList_0()
	for _, e := range entries {
		list.add(any(e))
	}
	return outcome_(list, failure)
}

// openZip_ is a java.util.zip.ZipFile: an archive on disk, read through its central directory.
type openZip_ struct {
	file     *os.File
	entries  map[string]*zip.File
	modified time.Time
}

var openZips_ = map[string]*openZip_{}

// Io_readZipEntryText keeps the archive open for the next call; it is opened again when the
// file's modification time has changed.
func Io_readZipEntryText(zipPath string, entryName string, charset string) *Outcome_base {
	path := nioPath_(zipPath)
	st, err := os.Stat(sysPath_(path))
	if err != nil {
		return Outcome_base_err(nioError_(path, err))
	}
	z := openZips_[zipPath]
	if z == nil || !z.modified.Equal(st.ModTime()) {
		if z != nil {
			z.file.Close()
		}
		f, failure := openFile_(path, os.O_RDONLY)
		if failure != nil {
			return Outcome_base_err(failure)
		}
		r, err := zip.NewReader(f, st.Size())
		if err != nil {
			f.Close()
			if st.Size() == 0 {
				return Outcome_base_err(javaError_(&ZipException{}, "zip file is empty"))
			}
			return Outcome_base_err(javaError_(&ZipException{}, "zip END header not found"))
		}
		z = &openZip_{f, map[string]*zip.File{}, st.ModTime()}
		for _, e := range r.File {
			z.entries[e.Name] = e
		}
		openZips_[zipPath] = z
	}
	entry := z.entries[entryName]
	if entry == nil {
		entry = z.entries[entryName+"/"] // a directory can be asked for without its slash
	}
	if entry == nil {
		panic(new_NullPointerException_String(path + "!/" + entryName).self())
	}
	in, err := entry.Open()
	if err != nil {
		return Outcome_base_err(javaError_(&ZipException{}, err.Error()))
	}
	defer in.Close()
	b, err := io.ReadAll(in)
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return Outcome_base_err(javaError_(&EOFException{}, "Unexpected end of ZLIB input stream"))
	} else if err != nil && err != zip.ErrChecksum { // ZipFile does not check the checksum
		return Outcome_base_err(javaError_(&ZipException{}, err.Error()))
	}
	s, _ := charset_of_(charset).decode(b)
	return outcome_(s, nil)
}

// Io_gzip compresses at the fastest level. The result is not the bytes zlib produces for Java
// (only its header is the same); it decompresses to the same data.
func Io_gzip(data []int8) []int8 {
	var out bytes.Buffer
	w, _ := gzip.NewWriterLevel(&out, gzip.BestSpeed)
	w.Write(u8_(data))
	w.Close()
	b := out.Bytes()
	b[8] = 0 // Java leaves the "extra flags" byte zero
	return i8_(b)
}

// Io_gunzip reads every member of the data and, like GZIPInputStream, ignores what follows the
// last one if that is not another member.
func Io_gunzip(data []int8) *Outcome_base {
	in := bytes.NewReader(u8_(data))
	var out []byte
	for member := 0; ; member++ {
		if failure := skipGzipHeader_(in); failure != nil {
			if member > 0 {
				failure = nil // what follows the last member is ignored
			}
			return outcome_(i8_(out), failure)
		}
		body, err := io.ReadAll(flate.NewReader(in))
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return Outcome_base_err(javaError_(&EOFException{}, "Unexpected end of ZLIB input stream"))
		} else if err != nil {
			return Outcome_base_err(javaError_(&ZipException{}, err.Error()))
		}
		var trailer [8]byte // checksum and length of the data
		if _, err := io.ReadFull(in, trailer[:]); err != nil {
			return Outcome_base_err(javaError_(&EOFException{}, nullstr_))
		}
		le := binary.LittleEndian
		if le.Uint32(trailer[:]) != crc32.ChecksumIEEE(body) || le.Uint32(trailer[4:]) != uint32(len(body)) {
			return Outcome_base_err(javaError_(&ZipException{}, "Corrupt GZIP trailer"))
		}
		out = append(out, body...)
	}
}

// skipGzipHeader_ reads a member's header: magic, method, flags, six bytes of time and system,
// then the optional fields the flags announce. (The optional header checksum is not verified.)
func skipGzipHeader_(in *bytes.Reader) *Throwable {
	eof := func() *Throwable { return javaError_(&EOFException{}, nullstr_) }
	var h [10]byte
	n, _ := io.ReadFull(in, h[:])
	switch {
	case n < 2:
		return eof()
	case h[0] != 0x1f || h[1] != 0x8b:
		return javaError_(&ZipException{}, "Not in GZIP format")
	case n < 3:
		return eof()
	case h[2] != 8:
		return javaError_(&ZipException{}, "Unsupported compression method")
	}
	flags := h[3]
	if flags&4 != 0 { // an extra field, after its length
		var size [2]byte
		io.ReadFull(in, size[:])
		in.Seek(int64(binary.LittleEndian.Uint16(size[:])), io.SeekCurrent)
	}
	for _, text := range []byte{8, 16} { // file name, comment: each ends with a zero byte
		for c := byte(1); flags&text != 0 && c != 0; {
			var err error
			if c, err = in.ReadByte(); err != nil {
				return eof()
			}
		}
	}
	if flags&2 != 0 { // header checksum
		in.Seek(2, io.SeekCurrent)
	}
	if at, _ := in.Seek(0, io.SeekCurrent); n < len(h) || at > in.Size() {
		return eof()
	}
	return nil
}

// ---- Classpath resources: the files embedded from go/closure/resources (rt_io_resources.go)

func Io_resourceExists(relativeTo *Class, name string) bool {
	_, ok := resourceBytes_(relativeTo.resourceName_(name))
	return ok
}

func Io_readResource(relativeTo *Class, name string) *Outcome_base {
	if b, ok := resourceBytes_(relativeTo.resourceName_(name)); ok {
		return outcome_(i8_(b), nil)
	}
	return outcome_(nil, nil)
}

// ---- Path arithmetic on strings: java.nio.file.Path on Unix

func Io_separator() string { return "/" }

// nioPath_ is Path.of(path).toString(): repeated and trailing slashes are dropped.
func nioPath_(path string) string {
	if strings.IndexByte(path, 0) >= 0 {
		panic(javaError_(&InvalidPathException{}, "Nul character not allowed: "+path).self_)
	}
	return filePath_(path)
}

// sysPath_ is the path as the JDK hands it to the operating system: the empty path is ".".
func sysPath_(path string) string {
	if path == "" {
		return "."
	}
	return path
}

// pathNames_ are the names of a path: none for the root, one empty name for the empty path.
func pathNames_(path string) []string {
	if path == "/" {
		return nil
	}
	return strings.Split(strings.TrimPrefix(path, "/"), "/")
}

func hasDots_(path string) bool {
	for _, n := range pathNames_(path) {
		if n == "." || n == ".." {
			return true
		}
	}
	return false
}

func Io_pathString(first string, more []string) string {
	path := first
	for _, m := range more {
		if m != "" && path != "" {
			path += "/" + m
		} else {
			path += m
		}
	}
	return nioPath_(path)
}

func Io_pathFileName(path string) string {
	path = nioPath_(path)
	return path[strings.LastIndexByte(path, '/')+1:]
}

// Io_pathParent is empty if the path has no parent.
func Io_pathParent(path string) *Optional {
	path = nioPath_(path)
	switch i := strings.LastIndexByte(path, '/'); {
	case i < 0, path == "/":
		return Optional_empty()
	case i == 0:
		return Optional_of("/")
	default:
		return Optional_of(path[:i])
	}
}

func Io_pathNormalize(path string) string {
	path = nioPath_(path)
	absolute := strings.HasPrefix(path, "/")
	var out []string
	for _, n := range pathNames_(path) {
		switch {
		case n == ".":
		case n != "..":
			out = append(out, n)
		case len(out) > 0 && out[len(out)-1] != "..":
			out = out[:len(out)-1]
		case !absolute:
			out = append(out, n)
		}
	}
	if absolute {
		return "/" + strings.Join(out, "/")
	}
	return strings.Join(out, "/")
}

func Io_pathToAbsolute(path string) string {
	cwd, _ := syscall.Getwd() // not os.Getwd, which may answer with a symbolic link from $PWD
	return Io_pathResolve(cwd, path)
}

func Io_pathResolve(path string, other string) string {
	path, other = nioPath_(path), nioPath_(other)
	switch {
	case strings.HasPrefix(other, "/") || path == "":
		return other
	case other == "":
		return path
	}
	return strings.TrimSuffix(path, "/") + "/" + other
}

func Io_pathRelativize(path string, other string) string {
	base, target := nioPath_(path), nioPath_(other)
	if base == target {
		return ""
	}
	if strings.HasPrefix(base, "/") != strings.HasPrefix(target, "/") {
		panic(new_IllegalArgumentException_String("'other' is different type of Path").self())
	}
	if base == "" {
		return target
	}
	// Paths with "." or ".." are compared in normal form; a base that still starts with ".."
	// then has no answer.
	if hasDots_(base) || hasDots_(target) {
		base, target = Io_pathNormalize(base), Io_pathNormalize(target)
		if base == target {
			return ""
		} else if base == "" {
			return target
		}
	}
	a, b := pathNames_(base), pathNames_(target)
	if target == "" {
		b = nil
	}
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	var out []string
	for _, n := range a[i:] {
		if n == ".." {
			panic(new_IllegalArgumentException_String("Unable to compute relative  path from " + nioPath_(path) + " to " + nioPath_(other)).self())
		}
		out = append(out, "..")
	}
	return strings.Join(append(out, b[i:]...), "/")
}

// ---- URI arithmetic on strings: java.net.URI

// uri_ is a parsed URI: the parts the compiler asks for, still escaped.
type uri_ struct {
	scheme string
	path   string
	opaque bool // no path
}

func isAlnum_(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func isHex_(c byte) bool { return strings.IndexByte("0123456789abcdefABCDEF", c) >= 0 }

// parseURI_ is new URI(s): the syntax of RFC 2396 as the JDK reads it. The index in an error
// counts UTF-16 chars.
func parseURI_(s string) (u uri_, failure *Throwable) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			if failure, ok = r.(*Throwable); !ok {
				panic(r)
			}
		}
	}()
	fail := func(reason string, index int) {
		panic(javaError_(&URISyntaxException{}, reason+" at index "+istr_(int64(unitsBefore_(s, index)))+": "+s))
	}
	// check verifies that s[from:to] holds only letters, digits, the given punctuation, escapes
	// and "other" characters (not ASCII, not a control or a space).
	check := func(from int, to int, punctuation string, what string) {
		for i := from; i < to; {
			c, size := utf8.DecodeRuneInString(s[i:])
			switch {
			case c == '%':
				if i+2 >= to || !isHex_(s[i+1]) || !isHex_(s[i+2]) {
					fail("Malformed escape pair", i)
				}
			case c < 0x80 && (isAlnum_(byte(c)) || strings.ContainsRune(punctuation, c)):
			case c < 0x80 || unicode.IsControl(c) || unicode.Is(unicode.Zs, c) || c == 0x2028 || c == 0x2029:
				fail("Illegal character in "+what, i)
			}
			i += size
		}
	}
	const marks, anywhere = "_-!.~'()*,;:$&+=", "_-!.~'()*,;:$&+=?/[]@"
	p, n := 0, len(s)
	if end := strings.IndexAny(s, "/?#:"); end >= 0 && s[end] == ':' {
		if end == 0 {
			fail("Expected scheme name", 0)
		}
		for i := 0; i < end; i++ {
			if c := s[i]; !isAlnum_(c) && !strings.ContainsRune("+-.", rune(c)) || i == 0 && c < 'A' {
				fail("Illegal character in scheme name", i)
			}
		}
		u.scheme, p = s[:end], end+1
	}
	if i := strings.IndexByte(s[p:], '#'); i >= 0 {
		n = p + i
	}
	if u.opaque = p > 0 && (p >= n || s[p] != '/'); u.opaque {
		if p >= n {
			fail("Expected scheme-specific part", p)
		}
		check(p, n, anywhere, "opaque part")
	} else { // hierarchical: [//authority][path][?query]
		if strings.HasPrefix(s[p:n], "//") {
			p += 2
			q := p
			for q < n && s[q] != '/' && s[q] != '?' {
				q++
			}
			if q == p && q >= len(s) {
				fail("Expected authority", p)
			}
			if strings.HasPrefix(s[p:q], "[") { // an IPv6 address; its syntax is checked loosely
				if end := strings.IndexByte(s[p:q], ']'); end < 0 {
					fail("Expected closing bracket for IPv6 address", q)
				} else if strings.Trim(s[p+1:p+end], "0123456789abcdefABCDEF:.%") != "" {
					fail("Malformed IPv6 address", p+1)
				}
			}
			check(p, q, marks+"@[]", "authority")
			p = q
		}
		q := p
		for q < n && s[q] != '?' {
			q++
		}
		check(p, q, marks+"/@", "path")
		u.path = s[p:q]
		if q < n {
			check(q+1, n, anywhere, "query")
		}
	}
	if n < len(s) {
		check(n+1, len(s), anywhere, "fragment")
	}
	return u, nil
}

// createURI_ is URI.create: a syntax error is an IllegalArgumentException.
func createURI_(s string) uri_ {
	u, failure := parseURI_(s)
	if failure != nil {
		panic(new_IllegalArgumentException_String_Throwable(failure.message_, failure).self())
	}
	return u
}

func Io_uriParse(uri string) *Outcome_base {
	_, failure := parseURI_(uri)
	return outcome_(uri, failure)
}

// Io_uriScheme is empty if the URI has no scheme.
func Io_uriScheme(uri string) *Optional {
	if u := createURI_(uri); u.scheme != "" {
		return Optional_of(u.scheme)
	}
	return Optional_empty()
}

// Io_uriPath decodes the escapes of the path, each run of them as UTF-8; empty for an opaque URI.
func Io_uriPath(uri string) *Optional {
	u := createURI_(uri)
	if u.opaque {
		return Optional_empty()
	}
	var out, run []byte
	for i := 0; i <= len(u.path); i++ {
		if i < len(u.path) && u.path[i] == '%' {
			b, _ := strconv.ParseUint(u.path[i+1:i+3], 16, 8)
			run = append(run, byte(b))
			i += 2
			continue
		}
		out = append(out, decodeUTF8_(run, func(int, int) {})...)
		run = run[:0]
		if i < len(u.path) {
			out = append(out, u.path[i])
		}
	}
	return Optional_of(string(out))
}

// ---- In-memory helpers

// Io_splitLines splits as BufferedReader.readLine does: at \n, \r and \r\n.
func Io_splitLines(text string) List {
	lines := new_ArrayList_0()
	for text != "" {
		i := strings.IndexAny(text, "\r\n")
		if i < 0 {
			lines.add(any(text))
			break
		}
		lines.add(any(text[:i]))
		if strings.HasPrefix(text[i:], "\r\n") {
			i++
		}
		text = text[i+1:]
	}
	return lines
}

func Io_base64Encode(b []int8) string { return base64.StdEncoding.EncodeToString(u8_(b)) }

// Io_base64Decode is Guava's BaseEncoding.base64().decode, with its error messages.
func Io_base64Decode(text string) []int8 {
	text = strings.TrimRight(text, "=")
	problem := ""
	if String_length(text)%4 == 1 {
		problem = "Invalid input length " + strconv.Itoa(int(String_length(text)))
	} else if i := strings.IndexFunc(text, func(c rune) bool { return c >= 0x80 || !isAlnum_(byte(c)) && c != '+' && c != '/' }); i >= 0 {
		if c := String_charAt(text[i:], 0); c <= ' ' || c >= 0x7F {
			problem = "Unrecognized character: 0x" + strconv.FormatUint(uint64(c), 16)
		} else {
			problem = "Unrecognized character: " + string(rune(c))
		}
	}
	if problem != "" {
		panic(new_IllegalArgumentException_String("com.google.common.io.BaseEncoding$DecodingException: " + problem).self())
	}
	b, _ := base64.RawStdEncoding.DecodeString(text)
	return i8_(b)
}
