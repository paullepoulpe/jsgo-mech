package closure

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// throws_ runs f and returns the Java exception it threw (nil if none).
func throws_(f func()) (ex any) {
	defer func() {
		if r := recover(); r != nil {
			ex = caught_(r)
		}
	}()
	f()
	return nil
}

func msgOf_(ex any) string {
	if ex == nil {
		return "<no exception>"
	}
	return ex.(interface{ toString() string }).toString()
}

func eq_(t *testing.T, what string, got, want any) {
	t.Helper()
	if !objequals_(got, want) && !(got == nil && want == nil) {
		t.Errorf("%s: got %#v, want %#v", what, got, want)
	}
}

func TestIOCharsetEncode(t *testing.T) {
	hi, lo := surrogateBytes_(0xD83D), surrogateBytes_(0xDE00)
	cases := []struct {
		s    string
		cs   *Charset
		want []byte
	}{
		{"héllo", StandardCharsets_UTF_8, []byte("héllo")},
		{"a" + hi + lo + "b", StandardCharsets_UTF_8, []byte("a\U0001F600b")},
		{"a" + hi + "b", StandardCharsets_UTF_8, []byte("a?b")},
		{"a" + lo + hi, StandardCharsets_UTF_8, []byte("a??")},
		{"\U0001F600", StandardCharsets_UTF_8, []byte("\U0001F600")},
		{"héllo€", StandardCharsets_US_ASCII, []byte("h?llo?")},
		{"a\U0001F600b", StandardCharsets_US_ASCII, []byte("a?b")},
		{"héllo€", StandardCharsets_ISO_8859_1, []byte{'h', 0xE9, 'l', 'l', 'o', '?'}},
		{"é€", Charset_forName("windows-1252"), []byte{0xE9, 0x80}},
		{"aé", StandardCharsets_UTF_16BE, []byte{0, 'a', 0, 0xE9}},
		{"aé", StandardCharsets_UTF_16LE, []byte{'a', 0, 0xE9, 0}},
		{"a\U0001F600", StandardCharsets_UTF_16, []byte{0xFE, 0xFF, 0, 'a', 0xD8, 0x3D, 0xDE, 0x00}},
		{"", StandardCharsets_UTF_16, []byte{}},
	}
	for _, c := range cases {
		got := u8_(String_getBytes(c.s, c.cs))
		if !bytes.Equal(got, c.want) {
			t.Errorf("getBytes(%q, %s) = % x, want % x", c.s, c.cs.name(), got, c.want)
		}
	}
}

func TestIOCharsetDecode(t *testing.T) {
	const R = "\uFFFD"
	cases := []struct {
		b    []byte
		cs   *Charset
		want string
	}{
		{[]byte("héllo"), StandardCharsets_UTF_8, "héllo"},
		{[]byte{'a', 0xFF, 'b'}, StandardCharsets_UTF_8, "a" + R + "b"},
		{[]byte{0xED, 0xA0, 0x80}, StandardCharsets_UTF_8, R},              // an encoded surrogate
		{[]byte{0xE0, 0x80, 0x80}, StandardCharsets_UTF_8, R + R + R},      // overlong
		{[]byte{0xC0, 0x80}, StandardCharsets_UTF_8, R + R},                // overlong
		{[]byte{0xE2, 0x82}, StandardCharsets_UTF_8, R},                    // truncated
		{[]byte{0xE2, 0x82, 'a'}, StandardCharsets_UTF_8, R + "a"},         // truncated then ASCII
		{[]byte{0xF0, 0x9F, 0x98, 'x'}, StandardCharsets_UTF_8, R + "x"},   // truncated 4-byte
		{[]byte{0xF5, 0x80}, StandardCharsets_UTF_8, R + R},                // invalid lead
		{[]byte{0xEF, 0xBB, 0xBF, 'a'}, StandardCharsets_UTF_8, "\uFEFFa"}, // the BOM is kept
		{[]byte{'a', 0xE9}, StandardCharsets_US_ASCII, "a" + R},
		{[]byte{'a', 0xE9}, StandardCharsets_ISO_8859_1, "aé"},
		{[]byte{0x80, 0x81, 0xE9}, Charset_forName("cp1252"), "€" + R + "é"},
		{[]byte{0xFE, 0xFF, 0, 'a', 0xD8, 0x3D, 0xDE, 0x00}, StandardCharsets_UTF_16, "a\U0001F600"},
		{[]byte{0xFF, 0xFE, 'a', 0}, StandardCharsets_UTF_16, "a"},
		{[]byte{0, 'a', 0xD8, 0x3D}, StandardCharsets_UTF_16BE, "a" + R},
		{[]byte{'a', 0, 'b'}, StandardCharsets_UTF_16LE, "a" + R},
	}
	for _, c := range cases {
		got := new_String_byteArr_Charset(i8_(c.b), c.cs)
		if got != c.want {
			t.Errorf("new String(% x, %s) = %q, want %q", c.b, c.cs.name(), got, c.want)
		}
	}
}

func TestIOCharsetLookup(t *testing.T) {
	for name, want := range map[string]*Charset{"utf-8": StandardCharsets_UTF_8, "UTF8": StandardCharsets_UTF_8, "ascii": StandardCharsets_US_ASCII,
		"latin1": StandardCharsets_ISO_8859_1, "ISO-8859-1": StandardCharsets_ISO_8859_1, "UTF-16": StandardCharsets_UTF_16, "Cp1252": charsetCP1252_} {
		if !Charset_isSupported(name) || Charset_forName(name) != want {
			t.Errorf("Charset.forName(%q)", name)
		}
	}
	if Charset_isSupported("no-such-charset") {
		t.Error("isSupported(no-such-charset)")
	}
	if ex := throws_(func() { Charset_forName("nope") }); !is_IllegalArgumentException(ex) || msgOf_(ex) != "java.nio.charset.UnsupportedCharsetException: nope" {
		t.Errorf("forName(nope) threw %v", msgOf_(ex))
	}
	eq_(t, "name", StandardCharsets_UTF_8.name(), "UTF-8")
	eq_(t, "toString", objstr_(StandardCharsets_US_ASCII), "US-ASCII")
	enc := StandardCharsets_US_ASCII.newEncoder()
	if !enc.canEncode('a') || enc.canEncode(0xE9) || enc.canEncode(0xD800) {
		t.Error("ASCII canEncode")
	}
	enc = StandardCharsets_UTF_8.newEncoder()
	if !enc.canEncode(0x20AC) || enc.canEncode(0xDC00) {
		t.Error("UTF-8 canEncode")
	}
	if !StandardCharsets_ISO_8859_1.newEncoder().canEncode(0xFF) || StandardCharsets_ISO_8859_1.newEncoder().canEncode(0x100) {
		t.Error("Latin-1 canEncode")
	}
	if StandardCharsets_UTF_8.compareTo(StandardCharsets_US_ASCII) <= 0 {
		t.Error("compareTo")
	}
}

func TestIOByteStreams(t *testing.T) {
	out := new_ByteArrayOutputStream()
	out.write('a')
	out.write_byteArr([]int8{'b', 'c'})
	out.write_byteArr_int_int([]int8{'x', 'd', 'e', 'x'}, 1, 2)
	out.up_OutputStream().write(0x1FF) // only the low byte
	eq_(t, "size", out.size(), int32(6))
	if !bytes.Equal(u8_(out.toByteArray()), []byte("abcde\xff")) {
		t.Errorf("toByteArray = %q", u8_(out.toByteArray()))
	}
	eq_(t, "toString(charset)", out.toString_Charset(StandardCharsets_ISO_8859_1), "abcdeÿ")

	in := new_ByteArrayInputStream(i8_([]byte{1, 2, 0xFF, 4, 5})).up_InputStream()
	eq_(t, "available", in.available(), int32(5))
	eq_(t, "read", in.read(), int32(1))
	buf := make([]int8, 10)
	eq_(t, "read(buf)", in.read_byteArr_int_int(buf, 2, 2), int32(2))
	eq_(t, "buf[3]", buf[3], int8(-1))
	eq_(t, "read(empty)", in.read_byteArr_int_int(buf, 0, 0), int32(0))
	eq_(t, "skip", in.skip(1), int64(1))
	eq_(t, "read", in.read(), int32(5))
	eq_(t, "read at end", in.read(), int32(-1))
	eq_(t, "read(buf) at end", in.read_byteArr(buf), int32(-1))

	// Buffered streams, and a limit.
	data := bytes.Repeat([]byte("0123456789"), 5000)
	bin := new_BufferedInputStream_InputStream_int(new_ByteArrayInputStream(i8_(data)).up_InputStream(), 64)
	lim := ByteStreams_limit(bin.up_FilterInputStream().up_InputStream(), 12345)
	got := readAllBytes_(lim)
	if !bytes.Equal(got, data[:12345]) {
		t.Errorf("limit read %d bytes", len(got))
	}
	eq_(t, "after the limit", bin.read(), int32('5'))

	sink := new_ByteArrayOutputStream()
	bout := new_BufferedOutputStream_OutputStream_int(sink.up_OutputStream(), 16)
	o := bout.up_FilterOutputStream().up_OutputStream()
	o.write_byteArr(i8_([]byte("hello")))
	eq_(t, "buffered: nothing written yet", sink.size(), int32(0))
	o.flush()
	eq_(t, "buffered: after flush", sink.size(), int32(5))
	o.write_byteArr(i8_(data[:100]))
	o.close()
	eq_(t, "buffered: after close", sink.size(), int32(105))
	if ex := throws_(func() { o.write('x') }); !is_IOException(ex) {
		t.Errorf("write after close threw %v", msgOf_(ex))
	}
}

// testFilterOut_ is a subclass in the style of translated code: it overrides close().
type testFilterOut_ struct {
	FilterOutputStream
	closes int
}

func (this *testFilterOut_) close() {
	this.closes++
	this.flush()
}

// testUpperOut_ overrides write(int) only.
type testUpperOut_ struct {
	OutputStream
	sb strings.Builder
}

func (this *testUpperOut_) write(b int32) { this.sb.WriteByte(byte(b) - 32) }

func TestIOStreamSubclass(t *testing.T) {
	sink := new_ByteArrayOutputStream()
	buffered := new_BufferedOutputStream(sink.up_OutputStream())
	f := &testFilterOut_{}
	f.self_ = f
	f.init_FilterOutputStream(buffered.up_FilterOutputStream().up_OutputStream())
	base := f.up_OutputStream()
	base.write_byteArr(i8_([]byte("abc")))
	base.write('d')
	eq_(t, "not flushed", sink.size(), int32(0))
	base.close() // reaches the override, which only flushes
	eq_(t, "override called", f.closes, 1)
	eq_(t, "flushed", sink.toString(), "abcd")
	base.write('e') // the underlying stream is still open
	base.flush()
	eq_(t, "still open", sink.toString(), "abcde")

	u := &testUpperOut_{}
	u.self_ = u
	u.OutputStream.write_byteArr(i8_([]byte("abc")))
	ps := new_PrintStream(&u.OutputStream)
	ps.print("xyz")
	eq_(t, "write(int) override", u.sb.String(), "ABCXYZ")
}

func TestIOPrintStream(t *testing.T) {
	sink := new_ByteArrayOutputStream()
	ps := new_PrintStream(sink.up_OutputStream())
	ps.print("a")
	ps.print(nullstr_)
	ps.println("é")
	ps.println_0()
	ps.println_Object(int32(42))
	ps.println_Object(nil)
	ps.print_Object(3.0)
	ps.append_char('x').append_CharSequence("yz").append_CharSequence_int_int("0123", 1, 3).append_CharSequence(nil)
	ps.write('!')
	ps.up_FilterOutputStream().up_OutputStream().write_byteArr(i8_([]byte("#\n")))
	// A surrogate pair printed in two halves comes out as one character.
	ps.append_char(0xD83D)
	ps.append_char(0xDE00)
	ps.append_char(0xD83D) // a lone half is replaced, here at close
	ps.close()
	want := "anullé\n\n42\nnull\n3.0xyz12null!#\n\U0001F600?"
	eq_(t, "PrintStream output", sink.toString(), want)
	if ps.checkError() {
		t.Error("checkError after a clean close")
	}

	// Errors are swallowed and remembered.
	dir := t.TempDir()
	fo := new_FileOutputStream(filepath.Join(dir, "f.txt"))
	ps = new_PrintStream(fo.up_OutputStream())
	fo.f_.Close()
	ps.println("lost")
	if !ps.checkError() {
		t.Error("checkError after a failed write")
	}

	ps = new_PrintStream_String(filepath.Join(dir, "g.txt"))
	ps.println("line")
	ps.close()
	b, _ := os.ReadFile(filepath.Join(dir, "g.txt"))
	eq_(t, "PrintStream(String)", string(b), "line\n")
}

func TestIOReaders(t *testing.T) {
	r := new_StringReader("ab\U0001F600").up_Reader()
	var got []int32
	for c := r.read(); c >= 0; c = r.read() {
		got = append(got, c)
	}
	if len(got) != 4 || got[2] != 0xD83D || got[3] != 0xDE00 {
		t.Errorf("StringReader units: %x", got)
	}

	br := new_BufferedReader(new_StringReader("one\ntwo\r\nthree\rfour\n\nlast").up_Reader())
	var lines []string
	for l := br.readLine(); l != nullstr_; l = br.readLine() {
		lines = append(lines, l)
	}
	eq_(t, "readLine", strings.Join(lines, "|"), "one|two|three|four||last")
	eq_(t, "readLine of empty", new_BufferedReader(new_StringReader("").up_Reader()).readLine(), nullstr_)
	eq_(t, "readLine of newline", new_BufferedReader(new_StringReader("\n").up_Reader()).readLine(), "")

	in := new_ByteArrayInputStream(i8_([]byte("h\xc3\xa9llo\nw\xffrld"))).up_InputStream()
	isr := new_InputStreamReader(in, StandardCharsets_UTF_8)
	eq_(t, "CharStreams.toString", CharStreams_toString(isr.self()), "héllo\nw\uFFFDrld")

	in = new_ByteArrayInputStream(i8_([]byte("a\nb\n"))).up_InputStream()
	l := CharStreams_readLines(new_BufferedReader(new_InputStreamReader(in, StandardCharsets_US_ASCII).up_Reader()).self())
	eq_(t, "readLines", objstr_(l), "[a, b]")

	rd := new_StringReader("hello").up_Reader()
	buf := make([]uint16, 8)
	eq_(t, "read(char[])", rd.read_charArr_int_int(buf, 1, 3), int32(3))
	eq_(t, "buf", fromUnits_(buf[1:4]), "hel")
	rd.mark(10)
	eq_(t, "rest", readAll_(rd), "lo")
	rd.reset()
	eq_(t, "after reset", rd.read(), int32('l'))
	rd.close()
	if ex := throws_(func() { rd.read() }); !is_IOException(ex) {
		t.Errorf("read after close threw %v", msgOf_(ex))
	}
}

func TestIOWriters(t *testing.T) {
	sw := new_StringWriter()
	w := sw.up_Writer()
	w.write("ab")
	w.write_int('c')
	w.write_charArr([]uint16{'d', 'e'})
	w.write_String_int_int("xfgx", 1, 2)
	w.append_char('h').append_CharSequence(new_StringBuilder_String("ij")).append_CharSequence_int_int("xklx", 1, 3).append_CharSequence(nil)
	w.write_int(0xD83D)
	w.write_int(0xDE00)
	w.close()
	w.write("!") // a closed StringWriter still works
	eq_(t, "StringWriter", sw.toString(), "abcdefghijklnull\U0001F600!")

	sink := new_ByteArrayOutputStream()
	osw := new_OutputStreamWriter(sink.up_OutputStream(), StandardCharsets_ISO_8859_1)
	bw := new_BufferedWriter(osw.up_Writer())
	bw.write("héllo")
	bw.newLine()
	bw.append_char(0x20AC)
	eq_(t, "nothing written before flush", sink.size(), int32(0))
	bw.flush()
	if !bytes.Equal(u8_(sink.toByteArray()), []byte("h\xe9llo\n?")) {
		t.Errorf("OutputStreamWriter wrote %q", u8_(sink.toByteArray()))
	}
	bw.close()
	if ex := throws_(func() { bw.write("x") }); !is_IOException(ex) {
		t.Errorf("write after close threw %v", msgOf_(ex))
	}

	// UTF-8 with surrogates split over writes, and a large write.
	sink = new_ByteArrayOutputStream()
	osw = new_OutputStreamWriter(sink.up_OutputStream(), StandardCharsets_UTF_8)
	big := strings.Repeat("0123456789", 3000)
	osw.write("a")
	osw.write_int(0xD83D)
	osw.write_int(0xDE00)
	osw.write(big)
	osw.write_int(0xDE00) // a lone low surrogate
	osw.write_int(0xD83D) // a high surrogate at the end
	osw.close()
	eq_(t, "UTF-8 writer", sink.toString(), "a\U0001F600"+big+"??")

	// An encoder that reports instead of replacing.
	enc := StandardCharsets_US_ASCII.newEncoder()
	osw = new_OutputStreamWriter_OutputStream_CharsetEncoder(new_ByteArrayOutputStream().up_OutputStream(), enc)
	if ex := throws_(func() { osw.write("é") }); !is_CharacterCodingException(ex) || msgOf_(ex) != "java.nio.charset.UnmappableCharacterException: Input length = 1" {
		t.Errorf("unmappable character threw %v", msgOf_(ex))
	}
	sink = new_ByteArrayOutputStream()
	enc = StandardCharsets_US_ASCII.newEncoder().onMalformedInput(CodingErrorAction_REPLACE).onUnmappableCharacter(CodingErrorAction_REPLACE)
	osw = new_OutputStreamWriter_OutputStream_CharsetEncoder(sink.up_OutputStream(), enc)
	osw.write("é!")
	osw.flush()
	eq_(t, "replacing encoder", sink.toString(), "?!")
}

func TestIOFile(t *testing.T) {
	cases := []struct{ path, name, parent string }{
		{"a/b/c.txt", "c.txt", "a/b"},
		{"a//b///c/", "c", "a/b"},
		{"/a", "a", "/"},
		{"/", "", nullstr_},
		{"a", "a", nullstr_},
		{"", "", nullstr_},
		{"/a/b/", "b", "/a"},
	}
	for _, c := range cases {
		f := new_File(c.path)
		if f.getName() != c.name || f.getParent() != c.parent {
			t.Errorf("File(%q): name %q parent %q", c.path, f.getName(), f.getParent())
		}
	}
	eq_(t, "getPath", new_File("a//b/").getPath(), "a/b")
	eq_(t, "File(parent, child)", new_File_File_String(new_File("a"), "/b").getPath(), "a/b")
	eq_(t, "File(null, child)", new_File_File_String(nil, "b").getPath(), "b")
	if new_File("x").getParentFile() != nil {
		t.Error("getParentFile")
	}
	eq_(t, "equals", any(new_File("a/b")), any(new_File("a//b")))
	eq_(t, "hashCode", new_File("a").hashCode(), String_hashCode("a")^1234321)
	if new_File("a").compareTo(new_File("b")) >= 0 {
		t.Error("compareTo")
	}
	cwd, _ := os.Getwd()
	eq_(t, "getAbsolutePath", new_File("x/../y").getAbsolutePath(), cwd+"/x/../y")
	eq_(t, "getAbsolutePath of absolute", new_File("/x").getAbsolutePath(), "/x")
	eq_(t, "separator", File_separator, "/")
	eq_(t, "separatorChar", File_separatorChar, uint16('/'))

	dir := t.TempDir()
	d := new_File(dir + "/p/q")
	if d.isDirectory() || !d.mkdirs() || d.mkdirs() || !d.isDirectory() {
		t.Error("mkdirs")
	}
	f := new_File_File_String(d, "f.txt")
	if !f.createNewFile() || f.createNewFile() || !f.exists() || !f.isFile() || f.isDirectory() {
		t.Error("createNewFile")
	}
	if ex := throws_(func() { new_File(dir + "/nope/f").createNewFile() }); !is_IOException(ex) || !strings.Contains(msgOf_(ex), "No such file or directory") {
		t.Errorf("createNewFile in a missing directory threw %v", msgOf_(ex))
	}
	Files_write(i8_([]byte("data")), f)
	eq_(t, "length", f.length(), int64(4))
	eq_(t, "toByteArray", string(u8_(Files_toByteArray(f))), "data")
	list := d.listFiles()
	if len(list) != 1 || list[0].getPath() != dir+"/p/q/f.txt" {
		t.Errorf("listFiles: %v", list)
	}
	if f.listFiles() != nil {
		t.Error("listFiles of a file")
	}
	if d.delete() || !f.delete() || !d.delete() || d.exists() {
		t.Error("delete")
	}

	ex := throws_(func() { new_FileInputStream(dir + "/missing.js") })
	if !is_FileNotFoundException(ex) || msgOf_(ex) != "java.io.FileNotFoundException: "+dir+"/missing.js (No such file or directory)" {
		t.Errorf("FileInputStream of a missing file threw %v", msgOf_(ex))
	}
	ex = throws_(func() { new_FileInputStream(dir) })
	if !is_FileNotFoundException(ex) || !strings.HasSuffix(msgOf_(ex), "(Is a directory)") {
		t.Errorf("FileInputStream of a directory threw %v", msgOf_(ex))
	}
	ex = throws_(func() { new_FileOutputStream(dir + "/no/dir/f") })
	if !is_FileNotFoundException(ex) {
		t.Errorf("FileOutputStream in a missing directory threw %v", msgOf_(ex))
	}
}

func TestIOPath(t *testing.T) {
	p := func(s string) Path { return Path_of(s, nil) }
	str := func(x Path) string {
		if x == nil {
			return "<null>"
		}
		return objstr_(x)
	}
	eq_(t, "of", str(Path_of("a", []string{"b", "", "c/"})), "a/b/c")
	eq_(t, "of slashes", str(p("//a///b//")), "/a/b")
	eq_(t, "of empty", str(Path_of("", []string{"a"})), "a")

	for _, c := range [][2]string{
		{"a/./b/../c", "a/c"}, {"../a/..", ".."}, {"/../a", "/a"}, {"a/..", ""}, {".", ""}, {"./a.js", "a.js"},
		{"a/../../b", "../b"}, {"/a/b/../../..", "/"}, {"", ""}, {"../..", "../.."},
	} {
		eq_(t, "normalize "+c[0], str(p(c[0]).normalize()), c[1])
	}
	for _, c := range [][3]string{
		{"a/b", "c", "a/b/c"}, {"a/b", "/c", "/c"}, {"a", "", "a"}, {"", "c", "c"}, {"/", "c", "/c"}, {"a", "../b", "a/../b"},
	} {
		eq_(t, "resolve "+c[0]+" "+c[1], str(p(c[0]).resolve_Path(p(c[1]))), c[2])
		eq_(t, "Path.resolve(String)", str(Path_resolve(p(c[0]), c[1])), c[2])
	}
	for _, c := range [][3]string{
		{"a/b", "a/b/c/d", "c/d"}, {"a/b", "a/c", "../c"}, {"/a/b", "/a/b", ""}, {"a", "b", "../b"}, {"", "a/b", "a/b"},
		{"a/b", "", "../.."}, {"/a", "/", ".."}, {"/", "/a/b", "a/b"}, {"a/../b", "c", "../c"}, {"a", "../b", "../../b"},
	} {
		eq_(t, "relativize "+c[0]+" "+c[1], str(p(c[0]).relativize(p(c[1]))), c[2])
	}
	if ex := throws_(func() { p("..").relativize(p("a")) }); msgOf_(ex) != "java.lang.IllegalArgumentException: Unable to compute relative  path from .. to a" {
		t.Errorf("relativize from .. threw %v", msgOf_(ex))
	}
	eq_(t, "compareTo", compareNatural_(p("a/b"), p("a")), int32(2))
	if ex := throws_(func() { p("/a").relativize(p("b")) }); !is_IllegalArgumentException(ex) {
		t.Errorf("relativize of different types threw %v", msgOf_(ex))
	}
	for _, c := range [][2]string{{"a/b", "a"}, {"/a", "/"}, {"/", "<null>"}, {"a", "<null>"}, {"", "<null>"}, {"/a/b", "/a"}} {
		eq_(t, "getParent "+c[0], str(p(c[0]).getParent()), c[1])
	}
	for _, c := range [][2]string{{"a/b", "b"}, {"/a", "a"}, {"/", "<null>"}, {"", ""}} {
		eq_(t, "getFileName "+c[0], str(p(c[0]).(*unixPath_).getFileName()), c[1])
	}
	eq_(t, "resolveSibling", str(Path_resolveSibling(p("a/b"), "c")), "a/c")
	eq_(t, "resolveSibling without parent", str(Path_resolveSibling(p("b"), "c")), "c")
	cwd, _ := os.Getwd()
	eq_(t, "toAbsolutePath", str(p("x/../y").toAbsolutePath()), cwd+"/x/../y")
	eq_(t, "toAbsolutePath of empty", str(p("").toAbsolutePath()), cwd)
	eq_(t, "toFile", Path_toFile(p("a/b")).getPath(), "a/b")

	up := p("/a/b/c").(*unixPath_)
	if !up.startsWith(p("/a/b")) || up.startsWith(p("a/b")) || up.startsWith(p("/a/bc")) || !up.startsWith(p("/")) ||
		!up.endsWith(p("b/c")) || up.endsWith(p("/b/c")) || !up.endsWith(p("/a/b/c")) || up.endsWith(p("")) {
		t.Error("startsWith / endsWith")
	}
	eq_(t, "getNameCount", up.getNameCount(), int32(3))
	eq_(t, "getNameCount of root", p("/").(*unixPath_).getNameCount(), int32(0))
	eq_(t, "getNameCount of empty", p("").(*unixPath_).getNameCount(), int32(1))
	eq_(t, "getName", str(up.getName(1)), "b")
	eq_(t, "subpath", str(up.subpath(1, 3)), "b/c")
	var names []string
	for it := up.iterator(); it.hasNext(); {
		names = append(names, objstr_(it.next()))
	}
	eq_(t, "iterator", strings.Join(names, ","), "a,b,c")
	eq_(t, "equals", any(p("a//b")), any(p("a/b")))
	eq_(t, "hashCode", objhash_(p("ab")), int32(31*'a'+'b'))
	if compareNatural_(p("a"), p("b")) >= 0 || compareNatural_(p("a/b"), p("a")) <= 0 {
		t.Error("compareTo")
	}
}

func TestIOGlob(t *testing.T) {
	fs := FileSystems_getDefault()
	cases := []struct {
		glob, path string
		want       bool
	}{
		{"*.js", "a.js", true}, {"*.js", "dir/a.js", false}, {"**.js", "dir/sub/a.js", true}, {"**/*.js", "a.js", false},
		{"**/*.js", "x/a.js", true}, {"./**.js", "./x/y.js", true}, {"./**.js", "x/y.js", false}, {"a?c", "abc", true},
		{"a?c", "a/c", false}, {"*.{js,ts}", "a.ts", true}, {"*.{js,ts}", "a.jsx", false}, {"[a-c]x", "bx", true},
		{"[a-c]x", "dx", false}, {"[!a-c]x", "dx", true}, {"[!a-c]x", "/x", false}, {"a\\*b", "a*b", true},
		{"a\\*b", "axb", false}, {"a.b", "axb", false}, {"a+(b)", "a+(b)", true}, {"*", ".hidden", true},
		{"src/*/x", "src/a/x", true}, {"src/*/x", "src/a/b/x", false}, {"[*?]", "*", true}, {"a,b", "a,b", true},
		{"{a,b}/**", "b/c/d", true}, {"é*", "éa", true}, {"}", "}", true}, {"[^a]", "^", true}, {"[^a]", "b", false},
		{"[a-]", "-", true}, {"**", "a/b", true}, {"*", "a/b", false}, {"a**b", "a/x/b", true}, {"{a,}b", "b", true},
		{"[\\]]", "\\", false}, {"[a&&b]", "&", true}, {"", "", true},
	}
	for _, c := range cases {
		m := fs.getPathMatcher("glob:" + c.glob)
		if got := m.matches(Path_of(c.path, nil)); got != c.want {
			t.Errorf("glob %q matches %q = %v, want %v", c.glob, c.path, got, c.want)
		}
	}
	if !fs.getPathMatcher("regex:a.c").matches(Path_of("abc", nil)) {
		t.Error("regex matcher")
	}
	if ex := throws_(func() { fs.getPathMatcher("glob:[z-a]") }); msgOf_(ex) != "java.lang.IllegalArgumentException: Invalid range near index 1\n[z-a]\n ^" {
		t.Errorf("glob [z-a] threw %v", msgOf_(ex))
	}
	for _, bad := range []string{"glob:[a", "glob:{a,{b}}", "glob:{a", "glob:a\\", "glob:[a/b]", "noglob", "x:y", "glob:[]a]", "glob:[!]a]"} {
		ex := throws_(func() { fs.getPathMatcher(bad) })
		if !is_IllegalArgumentException(ex) && !is_UnsupportedOperationException(ex) {
			t.Errorf("getPathMatcher(%q) threw %v", bad, msgOf_(ex))
		}
	}
}

type testVisitor_ struct {
	SimpleFileVisitor
	log  []string
	skip string
}

func (this *testVisitor_) preVisitDirectory(dir any, attrs BasicFileAttributes) *FileVisitResult {
	name := objstr_(dir)
	this.log = append(this.log, "pre "+name)
	if this.skip != "" && strings.HasSuffix(name, this.skip) {
		return FileVisitResult_SKIP_SUBTREE
	}
	return FileVisitResult_CONTINUE
}

func (this *testVisitor_) visitFile(file any, attrs BasicFileAttributes) *FileVisitResult {
	a := attrs.(*fileAttributes_)
	this.log = append(this.log, "file "+objstr_(file)+" "+istr_(a.size()))
	return FileVisitResult_CONTINUE
}

func TestIOWalkFileTree(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(dir+"/a/b", 0o777)
	os.MkdirAll(dir+"/c", 0o777)
	os.WriteFile(dir+"/a/1.js", []byte("1"), 0o666)
	os.WriteFile(dir+"/a/b/2.js", []byte("22"), 0o666)
	os.WriteFile(dir+"/c/3.txt", []byte("333"), 0o666)
	os.Symlink(dir+"/c", dir+"/a/link")

	run := func(v *testVisitor_, follow bool, depth int32) []string {
		v.self_ = v
		v.init_SimpleFileVisitor()
		var opts Set
		if follow {
			opts = EnumSet_of(FileVisitOption_FOLLOW_LINKS.up_Enum())
		} else {
			opts = EnumSet_noneOf(nil)
		}
		Files_file_walkFileTree(Path_of(dir, nil), opts, depth, v)
		for i := range v.log {
			v.log[i] = strings.ReplaceAll(v.log[i], dir, "D")
		}
		sort.Strings(v.log)
		return v.log
	}
	eq_(t, "walk following links", strings.Join(run(&testVisitor_{}, true, 0x7fffffff), "; "),
		"file D/a/1.js 1; file D/a/b/2.js 2; file D/a/link/3.txt 3; file D/c/3.txt 3; pre D; pre D/a; pre D/a/b; pre D/a/link; pre D/c")
	got := run(&testVisitor_{}, false, 0x7fffffff)
	if strings.Contains(strings.Join(got, ";"), "pre D/a/link") || !strings.Contains(strings.Join(got, ";"), "file D/a/link ") {
		t.Errorf("walk without following links: %v", got)
	}
	eq_(t, "walk with a depth", strings.Join(run(&testVisitor_{}, true, 1), "; "), "file D/a 4096; file D/c 4096; pre D")
	eq_(t, "walk skipping a subtree", strings.Join(run(&testVisitor_{skip: "/a"}, true, 99), "; "), "file D/c/3.txt 3; pre D; pre D/a; pre D/c")

	// A missing start file goes to visitFileFailed, which rethrows by default.
	v := new_SimpleFileVisitor()
	ex := throws_(func() { Files_file_walkFileTree(Path_of(dir+"/missing", nil), nil, 10, v) })
	if !is_IOException(ex) || msgOf_(ex) != "java.nio.file.NoSuchFileException: "+dir+"/missing" {
		t.Errorf("walk of a missing file threw %v", msgOf_(ex))
	}
}

func TestIONioFiles(t *testing.T) {
	dir := t.TempDir()
	p := Path_of(dir, []string{"x", "y", "f.txt"})
	Files_file_createDirectories(p.getParent(), nil)
	Files_file_createDirectories(p.getParent(), nil) // existing is fine
	w := Files_file_newBufferedWriter(p, StandardCharsets_UTF_8, nil)
	w.write("héllo\n")
	w.close()
	eq_(t, "readString", Files_file_readString(p, StandardCharsets_UTF_8), "héllo\n")
	out := Files_file_newOutputStream(p, []OpenOption{StandardOpenOption_CREATE, StandardOpenOption_APPEND})
	out.write_byteArr(i8_([]byte("more\xff")))
	out.close()
	br := Files_file_newBufferedReader(p, StandardCharsets_ISO_8859_1)
	eq_(t, "line 1", br.readLine(), "hÃ©llo")
	eq_(t, "line 2", br.readLine(), "moreÿ")
	eq_(t, "end", br.readLine(), nullstr_)
	br.close()
	if ex := throws_(func() { Files_file_readString(p, StandardCharsets_UTF_8) }); !is_CharacterCodingException(ex) ||
		msgOf_(ex) != "java.nio.charset.MalformedInputException: Input length = 1" {
		t.Errorf("readString of malformed input threw %v", msgOf_(ex))
	}
	if ex := throws_(func() { Files_file_newBufferedReader(p, StandardCharsets_UTF_8).readLine() }); !is_CharacterCodingException(ex) {
		t.Errorf("newBufferedReader of malformed input threw %v", msgOf_(ex))
	}
	missing := Path_of(dir, []string{"nope.txt"})
	for _, f := range []func(){
		func() { Files_file_readString(missing, StandardCharsets_UTF_8) },
		func() { Files_file_newBufferedReader(missing, StandardCharsets_UTF_8) },
		func() { Files_file_getLastModifiedTime(missing, nil) },
		func() { Files_file_newOutputStream(Path_of(dir, []string{"no", "f"}), nil) },
	} {
		if ex := throws_(f); !is_IOException(ex) || !strings.HasPrefix(msgOf_(ex), "java.nio.file.NoSuchFileException: "+dir) {
			t.Errorf("missing file threw %v", msgOf_(ex))
		}
	}
	t1 := Files_file_getLastModifiedTime(p, []*LinkOption{})
	t2 := Files_file_getLastModifiedTime(p, nil)
	if !objequals_(t1, t2) || t1.compareTo(t2) != 0 || objhash_(t1) != objhash_(t2) {
		t.Error("FileTime equality")
	}
	eq_(t, "FileTime.toString", (&FileTime{1500000000 * 1e9}).toString(), "2017-07-14T02:40:00Z")
	eq_(t, "FileTime.toString with fraction", (&FileTime{1500000000*1e9 + 120e6}).toString(), "2017-07-14T02:40:00.12Z")

	// Guava's Files.
	f := Path_toFile(p)
	Files_asCharSink(f, StandardCharsets_UTF_8, []*FileWriteMode{}).write("one\ntwo\n")
	Files_asCharSink(f, StandardCharsets_UTF_8, []*FileWriteMode{FileWriteMode_APPEND}).write(new_StringBuilder_String("three"))
	src := Files_asCharSource(f, StandardCharsets_UTF_8)
	eq_(t, "CharSource.read", src.read(), "one\ntwo\nthree")
	eq_(t, "CharSource.openStream", readAll_(src.openStream()), "one\ntwo\nthree")
	eq_(t, "Files.newReader", Files_newReader(f, StandardCharsets_UTF_8).readLine(), "one")
	eq_(t, "CharSource.wrap", CharSource_wrap("abc").read(), "abc")
	if ex := throws_(func() { Files_asCharSource(new_File(dir+"/zz"), StandardCharsets_UTF_8).read() }); !is_FileNotFoundException(ex) {
		t.Errorf("CharSource of a missing file threw %v", msgOf_(ex))
	}
}

func TestIOZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	big := strings.Repeat("var x = 1;\n", 5000)
	for _, e := range [][2]string{{"a.js", "alpha"}, {"dir/", ""}, {"dir/b.js", big}, {"é.js", "x"}} {
		w, _ := zw.Create(e[0])
		w.Write([]byte(e[1]))
	}
	st, _ := zw.CreateHeader(&zip.FileHeader{Name: "stored.txt", Method: zip.Store})
	st.Write([]byte("stored"))
	zw.Close()
	path := filepath.Join(t.TempDir(), "t.zip")
	os.WriteFile(path, buf.Bytes(), 0o666)

	zf := new_ZipFile(new_File(path))
	e := zf.getEntry("dir/b.js")
	eq_(t, "getName", e.getName(), "dir/b.js")
	eq_(t, "getSize", e.getSize(), int64(len(big)))
	eq_(t, "contents", string(readAllBytes_(zf.getInputStream(e))), big)
	eq_(t, "stored contents", string(readAllBytes_(zf.getInputStream(zf.getEntry("stored.txt")))), "stored")
	eq_(t, "directory entry", zf.getEntry("dir").getName(), "dir/")
	if zf.getEntry("nope") != nil {
		t.Error("getEntry of a missing name")
	}
	zf.close()
	zf.close()
	if ex := throws_(func() { zf.getEntry("a.js") }); !is_IllegalStateException(ex) {
		t.Errorf("getEntry on a closed file threw %v", msgOf_(ex))
	}
	if ex := throws_(func() { new_ZipFile(new_File(path + ".missing")) }); !is_IOException(ex) {
		t.Errorf("ZipFile of a missing file threw %v", msgOf_(ex))
	}
	os.WriteFile(path+".bad", []byte("not a zip file at all"), 0o666)
	if ex := throws_(func() { new_ZipFile(new_File(path + ".bad")) }); !is_IOException(ex) {
		t.Errorf("ZipFile of a non-zip threw %v", msgOf_(ex))
	}

	// The same archive as a stream, read the way the externs are.
	zin := new_ZipInputStream(new_ByteArrayInputStream(i8_(buf.Bytes())).up_InputStream())
	var seen []string
	for entry := zin.getNextEntry(); entry != nil; entry = zin.getNextEntry() {
		in := zin.up_InflaterInputStream().up_FilterInputStream().up_InputStream()
		data := readAllBytes_(new_BufferedInputStream(ByteStreams_limit(in, entry.getSize())).up_FilterInputStream().up_InputStream())
		seen = append(seen, entry.getName()+":"+istr_(int64(len(data))))
		eq_(t, "read at the end of an entry", in.read(), int32(-1))
	}
	eq_(t, "ZipInputStream entries", strings.Join(seen, " "), "a.js:5 dir/:0 dir/b.js:55000 é.js:1 stored.txt:6")
	zin.close()
	zin = new_ZipInputStream(new_ByteArrayInputStream(i8_([]byte("garbage"))).up_InputStream())
	if zin.getNextEntry() != nil {
		t.Error("ZipInputStream over garbage")
	}
}

func TestIOGzip(t *testing.T) {
	data := []byte(strings.Repeat("the quick brown fox ", 4000))
	for _, level := range []int32{-1, 1, 9, 0} {
		sink := new_ByteArrayOutputStream()
		gz := new_GZIPOutputStream_OutputStream(sink.up_OutputStream())
		if !bytes.Equal(u8_(sink.toByteArray()), []byte{0x1f, 0x8b, 8, 0, 0, 0, 0, 0, 0, 0xff}) {
			t.Errorf("GZIP header: % x", u8_(sink.toByteArray()))
		}
		gz.def.setLevel(level)
		o := gz.up_DeflaterOutputStream().up_FilterOutputStream().up_OutputStream()
		o.write_byteArr(i8_(data[:10]))
		o.write(int32(data[10]))
		o.write_byteArr_int_int(i8_(data), 11, int32(len(data)-11))
		o.close()
		o.close()
		zr, err := gzip.NewReader(bytes.NewReader(u8_(sink.toByteArray())))
		if err != nil {
			t.Fatal(err)
		}
		back, err := io.ReadAll(zr)
		if err != nil || !bytes.Equal(back, data) {
			t.Errorf("level %d: Go cannot read the stream back: %v", level, err)
		}
		in := new_GZIPInputStream(new_ByteArrayInputStream(sink.toByteArray()).up_InputStream(), 65536)
		if !bytes.Equal(readAllBytes_(in.up_InflaterInputStream().up_FilterInputStream().up_InputStream()), data) {
			t.Errorf("level %d: GZIPInputStream round trip", level)
		}
	}
	// finish() ends the GZIP member and leaves the stream below open.
	sink := new_ByteArrayOutputStream()
	gz := new_GZIPOutputStream_OutputStream(sink.up_OutputStream())
	gz.write_byteArr(i8_([]byte("abc")))
	gz.finish()
	n := sink.size()
	gz.finish()
	eq_(t, "finish twice", sink.size(), n)
	if ex := throws_(func() { gz.write('x') }); !is_IOException(ex) {
		t.Errorf("write after finish threw %v", msgOf_(ex))
	}
	if ex := throws_(func() { gz.def.setLevel(10) }); !is_IllegalArgumentException(ex) {
		t.Errorf("setLevel(10) threw %v", msgOf_(ex))
	}
	for _, bad := range [][]byte{[]byte("plain text, not gzip"), {}} {
		ex := throws_(func() { new_GZIPInputStream(new_ByteArrayInputStream(i8_(bad)).up_InputStream(), 512) })
		if !is_IOException(ex) {
			t.Errorf("GZIPInputStream(%q) threw %v", bad, msgOf_(ex))
		}
	}
	trunc := u8_(sink.toByteArray())[:14]
	in := new_GZIPInputStream(new_ByteArrayInputStream(i8_(trunc)).up_InputStream(), 512)
	if ex := throws_(func() { readAllBytes_(&in.InputStream) }); !is_IOException(ex) {
		t.Errorf("truncated gzip threw %v", msgOf_(ex))
	}
}

func TestIOResources(t *testing.T) {
	c := classOf_("com.google.javascript.jscomp.AbstractCommandLineRunner")
	in := c.getResourceAsStream("/externs.zip")
	if in == nil {
		t.Fatal("no /externs.zip")
	}
	zin := new_ZipInputStream(in)
	n := 0
	var es3 string
	for e := zin.getNextEntry(); e != nil; e = zin.getNextEntry() {
		n++
		data := readAllBytes_(ByteStreams_limit(&zin.InputStream, e.getSize()))
		if int64(len(data)) != e.getSize() {
			t.Errorf("%s: read %d of %d bytes", e.getName(), len(data), e.getSize())
		}
		if e.getName() == "es3.js" {
			es3 = string(data)
		}
	}
	if n < 20 || !strings.Contains(es3, "@externs") {
		t.Errorf("externs.zip: %d entries, es3.js has %d bytes", n, len(es3))
	}
	if c.getResourceAsStream("externs.zip") != nil {
		t.Error("externs.zip resolved relative to the package")
	}
	if c.getResource("js/base.js") == nil || c.getResource("/com/google/javascript/jscomp/js/base.js") == nil {
		t.Error("js/base.js not found")
	}
	if c.getResource("js/nope.js") != nil || c.getResource("js") != nil || c.getResource("/") != nil {
		t.Error("getResource of a missing name")
	}
	inner := classOf_("com.google.javascript.jscomp.Compiler$Inner")
	s := CharStreams_toString(new_InputStreamReader(inner.getResourceAsStream("js/es6/util/arrayfromiterable.js"), StandardCharsets_UTF_8).self())
	if !strings.Contains(s, "$jscomp") {
		t.Errorf("runtime library: %q", s)
	}
	u := c.getResource("/externs.zip")
	eq_(t, "URL.toString", objstr_(u), "jar:file:/closure-compiler.jar!/externs.zip")
	if len(readAllBytes_(u.openStream())) < 1000 {
		t.Error("URL.openStream")
	}

	b := ResourceBundle_getBundle("com.google.javascript.jscomp.parsing.ParserConfig")
	ann := b.getString("jsdoc.annotations")
	if !strings.HasPrefix(ann, "addon,alias,alternateMessageId,") || strings.ContainsAny(ann, "\n\\") || !strings.HasSuffix(ann, ",wizcallback") {
		t.Errorf("jsdoc.annotations = %q", ann)
	}
	eq_(t, "compiler.reserved.vars", b.getString("compiler.reserved.vars"), "i,j,s,$,$j,o,ga,_gaq,sun,XR,java,gc")
	if !b.containsKey("jsdoc.suppressions") || !strings.HasSuffix(b.getString("jsdoc.primitives"), "asserts.truthy,") || b.keySet().size() != 4 {
		t.Error("ParserConfig keys")
	}
	if ex := throws_(func() { b.getString("no.such.key") }); !is_RuntimeException(ex) {
		t.Errorf("getString of a missing key threw %v", msgOf_(ex))
	}
	if ex := throws_(func() { ResourceBundle_getBundle("no.such.Bundle") }); !is_RuntimeException(ex) {
		t.Errorf("getBundle of a missing bundle threw %v", msgOf_(ex))
	}
}

func TestIOProperties(t *testing.T) {
	text := "# comment\n! comment\n\na = 1\nb:2\nc   3  \n  d=\\u00e9\\t\\x\\\\\ne = one, \\\n    two\\\n three\nf\\ g\\=h = v\nempty\n\\ud83d\\ude00 x\r\nlast=end"
	got := map[string]string{}
	var order []string
	parseProperties_(text, func(k, v string) {
		got[k] = v
		order = append(order, k)
	})
	want := map[string]string{"a": "1", "b": "2", "c": "3  ", "d": "é\tx\\", "e": "one, twothree", "f g=h": "v", "empty": "", "\U0001F600": "x", "last": "end"}
	if len(got) != len(want) {
		t.Errorf("keys: %q", order)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("property %q = %q, want %q", k, got[k], v)
		}
	}
}

func TestIOURI(t *testing.T) {
	type parts struct{ scheme, path string }
	n := nullstr_
	cases := map[string]parts{
		"http://example.com/a/b.js?q=1#f": {"http", "/a/b.js"},
		"a/b/c.js":                        {n, "a/b/c.js"},
		"/abs/path":                       {n, "/abs/path"},
		"file:/x/y%20z":                   {"file", "/x/y z"},
		"mailto:a@b.c":                    {"mailto", n},
		"//host/p":                        {n, "/p"},
		"":                                {n, ""},
		"a%C3%A9":                         {n, "aé"},
		"./x:y":                           {n, "./x:y"},
		"goog:foo.bar":                    {"goog", n},
		"é.js":                            {n, "é.js"},
	}
	for s, want := range cases {
		u := new_URI(s)
		if u.getScheme() != want.scheme || u.getPath() != want.path || u.isAbsolute() != (want.scheme != n) || u.toString() != s {
			t.Errorf("URI(%q): scheme %q path %q", s, u.getScheme(), u.getPath())
		}
	}
	bad := map[string]string{
		"a b":           "java.net.URISyntaxException: Illegal character in path at index 1: a b",
		"C:\\x\\y":      "java.net.URISyntaxException: Illegal character in opaque part at index 2: C:\\x\\y",
		":x":            "java.net.URISyntaxException: Expected scheme name at index 0: :x",
		"x:":            "java.net.URISyntaxException: Expected scheme-specific part at index 2: x:",
		"a%zz":          "java.net.URISyntaxException: Malformed escape pair at index 1: a%zz",
		"1a:b":          "java.net.URISyntaxException: Illegal character in scheme name at index 0: 1a:b",
		"http://h/a?b{": "java.net.URISyntaxException: Illegal character in query at index 12: http://h/a?b{",
		"é é":           "java.net.URISyntaxException: Illegal character in path at index 1: é é",
	}
	for s, want := range bad {
		ex := throws_(func() { new_URI(s) })
		if !is_URISyntaxException(ex) || msgOf_(ex) != want {
			t.Errorf("URI(%q) threw %v", s, msgOf_(ex))
		}
	}
	u := new_URI("http://user@Example.com:8080/a")
	eq_(t, "host", u.getHost(), "Example.com")
	eq_(t, "port", u.getPort(), int32(8080))
	eq_(t, "equals ignores the case of scheme and host", any(new_URI("HTTP://example.COM/a")), any(new_URI("http://example.com/a")))
	eq_(t, "hashCode", objhash_(new_URI("HTTP://example.COM/a")), objhash_(new_URI("http://example.com/a")))
	if new_URI("a/b").compareTo(new_URI("a/c")) >= 0 || new_URI("a").compareTo(new_URI("x:a")) >= 0 || new_URI("x:/a").compareTo(new_URI("x:a")) >= 0 {
		t.Error("compareTo")
	}
	eq_(t, "normalize", new_URI("a/./b/../c").normalize().toString(), "a/c")
	eq_(t, "resolve", new_URI("http://h/a/b/c.js").resolve("../d.js").toString(), "http://h/a/d.js")
	eq_(t, "resolve absolute", new_URI("http://h/a/b").resolve("/x").toString(), "http://h/x")
	eq_(t, "File.toURI", new_File("/tmp/a b").toURI().toString(), "file:/tmp/a%20b")
}

func TestIOThreadsAndExecutors(t *testing.T) {
	main := Thread_currentThread()
	var inside *Thread
	th := new_Thread_ThreadGroup_Runnable_String_long(nil, &Runnable_fn{func() { inside = Thread_currentThread() }}, "worker", 1<<20)
	th.setDaemon(true)
	th.start()
	th.join()
	if inside != th || Thread_currentThread() != main || th.isAlive() || th.getName() != "worker" {
		t.Error("Thread.start")
	}
	if Thread_interrupted() {
		t.Error("interrupted")
	}

	var made []*Thread
	factory := &ThreadFactory_fn{func(r Runnable) *Thread {
		th := new_Thread_ThreadGroup_Runnable_String_long(nil, r, "jscompiler", 0)
		made = append(made, th)
		return th
	}}
	ex := Executors_newSingleThreadExecutor(factory)
	var ran *Thread
	f := ex.submit(&Callable_fn{func() any { ran = Thread_currentThread(); return "result" }})
	eq_(t, "Future.get", f.get(), "result")
	eq_(t, "Future.get with a timeout", f.get_long_TimeUnit(1, TimeUnit_SECONDS), "result")
	if len(made) != 1 || ran != made[0] {
		t.Error("the task did not run on a thread of the factory")
	}
	f = ex.submit(&Callable_fn{func() any { panic(new_IllegalStateException_String("boom").self()) }})
	err := throws_(func() { f.get() })
	if !is_ExecutionException(err) || msgOf_(err) != "java.util.concurrent.ExecutionException: java.lang.IllegalStateException: boom" ||
		!is_IllegalStateException(to_ExecutionException(err).getCause().self()) {
		t.Errorf("get of a failed task threw %v", msgOf_(err))
	}
	ex.shutdown()

	pool := new_ThreadPoolExecutor_int_int_long_TimeUnit_BlockingQueue_ThreadFactory(4, 4, 0x7fffffff, TimeUnit_SECONDS, new_LinkedBlockingQueue(), factory)
	les := MoreExecutors_listeningDecorator(cast_[ExecutorService](pool.self()))
	futures := new_ArrayList_0()
	for i := int32(0); i < 3; i++ {
		i := i
		futures.add(any(cast_[ListenableFuture](les.submit(&Callable_fn{func() any { return i * 2 }}))))
	}
	eq_(t, "allAsList", objstr_(Futures_allAsList(futures).get()), "[0, 2, 4]")
	futures.add(any(les.submit(&Callable_fn{func() any { panic(new_RuntimeException_String("bad").self()) }})))
	err = throws_(func() { Futures_allAsList(futures).get() })
	if !is_ExecutionException(err) || msgOf_(err) != "java.util.concurrent.ExecutionException: java.lang.RuntimeException: bad" {
		t.Errorf("allAsList with a failure threw %v", msgOf_(err))
	}
	les.shutdown()

	eq_(t, "TimeUnit.toMillis", TimeUnit_SECONDS.toMillis(3), int64(3000))
	eq_(t, "TimeUnit.toSeconds", TimeUnit_MILLISECONDS.toSeconds(3999), int64(3))
	eq_(t, "TimeUnit saturates", TimeUnit_DAYS.toNanos(1<<40), int64(0x7fffffffffffffff))
	eq_(t, "TimeUnit name", objstr_(TimeUnit_SECONDS), "SECONDS")

	tl := new_ThreadLocal()
	eq_(t, "ThreadLocal initial", tl.get(), nil)
	tl.set("v")
	eq_(t, "ThreadLocal set", tl.get(), "v")
	tl.remove()
	eq_(t, "ThreadLocal removed", tl.get(), nil)
}

func TestIOThrowable(t *testing.T) {
	cause := new_IOException_String("disk")
	e := new_RuntimeException_String_Throwable("outer", cause.up_Exception().up_Throwable())
	sink := new_ByteArrayOutputStream()
	ps := new_PrintStream(sink.up_OutputStream())
	e.up_Exception().up_Throwable().printStackTrace(ps)
	eq_(t, "printStackTrace", sink.toString(), "java.lang.RuntimeException: outer\nCaused by: java.io.IOException: disk\n")
	th := new_Throwable_String("plain")
	eq_(t, "Throwable", th.toString(), "java.lang.Throwable: plain")
	eq_(t, "Throwable()", new_Throwable_0().getMessage(), nullstr_)
	th.initCause(&cause.Throwable)
	if th.getCause() != &cause.Throwable {
		t.Error("initCause")
	}
	if ex := throws_(func() { th.initCause(&cause.Throwable) }); !is_IllegalStateException(ex) {
		t.Errorf("second initCause threw %v", msgOf_(ex))
	}
	th.addSuppressed(&e.Throwable)
	eq_(t, "getSuppressed", len(th.getSuppressed()), 1)
	eq_(t, "FileNotFoundException is an IOException", is_IOException(new_FileNotFoundException_String("x").self()), true)
	eq_(t, "CharacterCodingException is an IOException", is_IOException(new_CharacterCodingException().self()), true)
}

func TestIOManagement(t *testing.T) {
	rt := Runtime_getRuntime()
	if rt.totalMemory() <= 0 || rt.freeMemory() < 0 || rt.freeMemory() > rt.totalMemory() {
		t.Error("Runtime memory")
	}
	heap := ManagementFactory_getMemoryMXBean().getHeapMemoryUsage()
	if heap.getUsed() <= 0 || heap.getMax() != -1 {
		t.Errorf("heap usage: %s", heap.toString())
	}
	if ManagementFactory_getMemoryMXBean().getNonHeapMemoryUsage().getUsed() <= 0 {
		t.Error("non-heap usage")
	}
	pools := ManagementFactory_getMemoryPoolMXBeans()
	for it := pools.iterator(); it.hasNext(); {
		p := it.next().(MemoryPoolMXBean)
		if p.getPeakUsage().getUsed() < 0 || p.getUsage().getUsed() <= 0 || p.getName() == "" {
			t.Error("memory pool")
		}
	}
	for it := ManagementFactory_getGarbageCollectorMXBeans().iterator(); it.hasNext(); {
		g := it.next().(GarbageCollectorMXBean)
		if g.getCollectionCount() < 0 || g.getCollectionTime() < 0 || g.getName() == "" {
			t.Error("gc bean")
		}
	}
	if ManagementFactory_getCompilationMXBean().getTotalCompilationTime() != 0 {
		t.Error("compilation bean")
	}
	eq_(t, "MemoryUsage", new_MemoryUsage(1, 2, 3, 4).getUsed(), int64(2))
}

func TestIOLogging(t *testing.T) {
	sink := new_ByteArrayOutputStream()
	saved := System_err
	System_err = new_PrintStream(sink.up_OutputStream())
	defer func() { System_err = saved }()

	l := Logger_getLogger("test.io.pkg.Class")
	if l != Logger_getLogger("test.io.pkg.Class") {
		t.Error("getLogger is not stable")
	}
	if l.isLoggable(Level_INFO) || !l.isLoggable(Level_WARNING) {
		t.Error("default level")
	}
	l.info("hidden")
	l.fine("hidden")
	l.warning("shown")
	l.log(Level_SEVERE, "bad", new_Throwable_String("why"))
	Logger_getLogger("test.io.pkg").setLevel(Level_FINE) // children inherit
	l.fine("fine now")
	l.finest("still hidden")
	l.log_Level_String_Object(Level_INFO, "value {0}", int32(7))
	l.setLevel(Level_OFF)
	l.severe("off")
	l.log_Level_String(Level_SEVERE, "off")
	eq_(t, "log output", sink.toString(), "WARNING: shown\nSEVERE: bad\njava.lang.Throwable: why\nFINE: fine now\nINFO: value 7\n")

	eq_(t, "Level.parse", Level_parse("WARNING"), any(Level_WARNING))
	eq_(t, "Level.parse of a number", Level_parse("800"), any(Level_INFO))
	eq_(t, "Level.getName", Level_FINE.getName(), "FINE")
	if ex := throws_(func() { Level_parse("LOUD") }); !is_IllegalArgumentException(ex) || msgOf_(ex) != "java.lang.IllegalArgumentException: Bad level \"LOUD\"" {
		t.Errorf("Level.parse(LOUD) threw %v", msgOf_(ex))
	}
}

type testLoader_ struct {
	CacheLoader
	loads []string
}

func (this *testLoader_) load(key any) any {
	k := str_(key)
	this.loads = append(this.loads, k)
	switch k {
	case "null":
		return nil
	case "checked":
		panic(new_IOException_String("io").self())
	case "unchecked":
		panic(new_IllegalStateException_String("state").self())
	}
	return "v:" + k
}

func TestIOCache(t *testing.T) {
	loader := &testLoader_{}
	loader.self_ = loader
	loader.init_CacheLoader()
	var removed []string
	cache := CacheBuilder_newBuilder().maximumSize(2).removalListener(&RemovalListener_fn{func(n *RemovalNotification) {
		removed = append(removed, str_(n.getKey())+"="+str_(n.getValue()))
	}}).build(&loader.CacheLoader)
	eq_(t, "load", cache.getUnchecked("a"), "v:a")
	eq_(t, "cached", cache.getUnchecked("a"), "v:a")
	eq_(t, "apply", cache.apply("b"), "v:b")
	cache.getUnchecked("a") // a is now the most recently used
	cache.getUnchecked("c") // evicts b
	eq_(t, "loads", strings.Join(loader.loads, ","), "a,b,c")
	eq_(t, "evictions", strings.Join(removed, ","), "b=v:b")
	cache.getUnchecked("b") // evicts a
	eq_(t, "evictions", strings.Join(removed, ","), "b=v:b,a=v:a")

	ex := throws_(func() { cache.getUnchecked("checked") })
	if !is_UncheckedExecutionException(ex) || msgOf_(ex) != "com.google.common.util.concurrent.UncheckedExecutionException: java.io.IOException: io" {
		t.Errorf("getUnchecked with a checked exception threw %v", msgOf_(ex))
	}
	ex = throws_(func() { cache.getUnchecked("unchecked") })
	if !is_UncheckedExecutionException(ex) || !is_IllegalStateException(to_UncheckedExecutionException(ex).getCause().self()) {
		t.Errorf("getUnchecked with an unchecked exception threw %v", msgOf_(ex))
	}
	lc := cache.(*localCache_)
	if ex = throws_(func() { lc.get("checked") }); !is_ExecutionException(ex) {
		t.Errorf("get with a checked exception threw %v", msgOf_(ex))
	}
	if ex = throws_(func() { lc.get("unchecked") }); !is_UncheckedExecutionException(ex) {
		t.Errorf("get with an unchecked exception threw %v", msgOf_(ex))
	}
	if ex = throws_(func() { cache.getUnchecked("null") }); ex == nil || !strings.Contains(msgOf_(ex), "CacheLoader returned null for key null.") {
		t.Errorf("a loader returning null threw %v", msgOf_(ex))
	}

	fn := CacheBuilder_from("maximumSize=10,concurrencyLevel=1,expireAfterAccess=5m").build(CacheLoader_from(&Function_base_fn{func(k any) any { return str_(k) + "!" }}))
	eq_(t, "CacheLoader.from", fn.getUnchecked("x"), "x!")
	for _, bad := range []string{"maximumSize", "maximumSize=x", "bogus=1", "weakKeys=1"} {
		if ex := throws_(func() { CacheBuilder_from(bad) }); !is_IllegalArgumentException(ex) {
			t.Errorf("CacheBuilder.from(%q) threw %v", bad, msgOf_(ex))
		}
	}
}

type testSaxHandler_ struct {
	events []string
}

func (this *testSaxHandler_) startDocument() { this.events = append(this.events, "startDocument") }
func (this *testSaxHandler_) endDocument()   { this.events = append(this.events, "endDocument") }
func (this *testSaxHandler_) startElement(uri string, localName string, qName string, atts Attributes) {
	a := atts.(*saxAttributes_)
	s := "<" + qName
	for i := int32(0); i < a.getLength(); i++ {
		s += " " + a.getQName(i) + "=" + a.getValue_int(i)
	}
	if uri != "" || localName != "" || atts.getValue("no-such") != nullstr_ {
		s += " ?!"
	}
	this.events = append(this.events, s+">")
}
func (this *testSaxHandler_) endElement(uri string, localName string, qName string) {
	this.events = append(this.events, "</"+qName+">")
}
func (this *testSaxHandler_) characters(ch []uint16, start int32, length int32) {
	this.events = append(this.events, "'"+strings.ReplaceAll(String_valueOf_charArr_int_int(ch, start, length), "\n", "\\n")+"'")
}

type testResolver_ struct{ calls []string }

func (this *testResolver_) resolveEntity(publicId string, systemId string) *InputSource {
	this.calls = append(this.calls, sstr_(publicId)+"|"+sstr_(systemId))
	return new_InputSource_Reader(new_StringReader("").up_Reader())
}

func TestIOSax(t *testing.T) {
	doc := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<!DOCTYPE translationbundle SYSTEM \"translationbundle.dtd\">\n" +
		"<translationbundle lang=\"fr\">\n<!-- c -->" +
		"<translation id=\"12\" x:y='a&amp;b'>H\u00e9llo &lt;<ph name=\"X\"/>w&#233;&#x1F600;</translation>\r\n" +
		"<t><![CDATA[<raw> & ]]></t></translationbundle>\n"
	parse := func(src *InputSource) (*testSaxHandler_, *testResolver_) {
		factory := SAXParserFactory_newInstance()
		factory.setValidating(false)
		factory.setXIncludeAware(false)
		factory.setFeature("http://xml.org/sax/features/external-general-entities", false)
		parser := factory.newSAXParser()
		res := &testResolver_{}
		parser.getXMLReader().setEntityResolver(res)
		h := &testSaxHandler_{}
		reader := parser.getXMLReader()
		reader.setContentHandler(h)
		reader.parse(src)
		return h, res
	}
	want := "startDocument <translationbundle lang=fr> '\\n' <translation id=12 x:y=a&b> 'H\u00e9llo ' '<' <ph name=X> </ph> 'w' '\u00e9' '\U0001F600' </translation> " +
		"'\\n' <t> '<raw> & ' </t> </translationbundle> endDocument"
	h, res := parse(new_InputSource(new_ByteArrayInputStream(i8_([]byte(doc))).up_InputStream()))
	eq_(t, "events from bytes", strings.Join(h.events, " "), want)
	cwd, _ := os.Getwd()
	eq_(t, "resolver", strings.Join(res.calls, " "), "null|file://"+cwd+"/translationbundle.dtd")
	h, _ = parse(new_InputSource_Reader(new_StringReader(doc).up_Reader()))
	eq_(t, "events from chars", strings.Join(h.events, " "), want)

	// How the JDK's parser cuts text into characters() calls (observed on JDK 21).
	chunks := map[string]string{
		"c\n\nd\ne\nf\ng":       "'c\\n\\nd' '\\ne\\nf' '\\ng'",
		"\n\na\nb\nc":           "'\\n\\na\\nb' '\\nc'",
		"a\n":                   "'a\\n'",
		"x&amp;y\nz\nw":         "'x' '&' 'y\\nz' '\\nw'",
		"a\n\n\nb\n\nc\nd":      "'a\\n\\n\\nb' '\\n\\nc\\nd'",
		"a]b\nc\nd":             "'a]b' '\\nc\\nd'",
		"a]]b]c":                "'a]]b]' 'c'",
		"\n \n a \n b\n":        "'\\n \\n a ' '\\n b\\n'",
		"a\r\nb\rc\nd":          "'a\\nb' '\\nc\\nd'",
		"a&#10;b\nc":            "'a' '\\n' 'b\\nc'",
		"]a\nb\nc":              "']a' '\\nb\\nc'",
		"a\nb]c\nd\ne":          "'a\\nb]' 'c\\nd' '\\ne'",
		"]]":                    "']]'",
		"a&lt;\nb\nc\nd":        "'a' '<' '\\nb\\nc' '\\nd'",
		"p<!-- c -->q<?pi d?>r": "'p' 'q' 'r'",
	}
	for text, want := range chunks {
		h, _ = parse(new_InputSource_Reader(new_StringReader("<r a='x\ny'>" + text + "</r>").up_Reader()))
		eq_(t, "chunks of "+text, strings.Join(h.events, " "), "startDocument <r a=x y> "+want+" </r> endDocument")
	}

	latin := "<?xml version='1.0' encoding='ISO-8859-1'?><a>\xe9</a>"
	h, _ = parse(new_InputSource(new_ByteArrayInputStream(i8_([]byte(latin))).up_InputStream()))
	eq_(t, "ISO-8859-1 document", strings.Join(h.events, " "), "startDocument <a> '\u00e9' </a> endDocument")

	for _, bad := range []string{"<a><b></a>", "<a>", "", "text", "<a>&nope;</a>", "<a></a><b></b>", "<a x=1></a>", "<a x='1' x='2'/>"} {
		ex := throws_(func() { parse(new_InputSource_Reader(new_StringReader(bad).up_Reader())) })
		if !is_SAXException(ex) {
			t.Errorf("parse(%q) threw %v", bad, msgOf_(ex))
		}
	}
}

func TestIOSystem(t *testing.T) {
	if System_out == nil || System_err == nil || System_in == nil {
		t.Fatal("standard streams")
	}
	eq_(t, "lineSeparator", System_lineSeparator(), "\n")
	os.Setenv("RT_IO_TEST_VAR", "yes")
	eq_(t, "getenv", System_getenv("RT_IO_TEST_VAR"), "yes")
	eq_(t, "getenv of an unset variable", System_getenv("RT_IO_TEST_UNSET"), nullstr_)
	flushStd_()
	in := new_ObjectInputStream_()
	if ex := throws_(func() { in.defaultReadObject() }); !is_UnsupportedOperationException(ex) {
		t.Errorf("defaultReadObject threw %v", msgOf_(ex))
	}
}

func new_ObjectInputStream_() *ObjectInputStream {
	in := &ObjectInputStream{}
	in.self_ = in
	return in
}
