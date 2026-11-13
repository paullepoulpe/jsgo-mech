package closure

import (
	"bytes"
	"os"
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

func TestIONioFiles(t *testing.T) {
	dir := t.TempDir()
	p := Path_of(dir, []string{"x", "y", "f.txt"})
	Files_createDirectories(p.getParent(), nil)
	Files_createDirectories(p.getParent(), nil) // existing is fine
	w := Files_newBufferedWriter(p, StandardCharsets_UTF_8, nil)
	w.write("héllo\n")
	w.close()
	eq_(t, "readString", Files_readString(p, StandardCharsets_UTF_8), "héllo\n")
	out := Files_newOutputStream(p, []OpenOption{StandardOpenOption_CREATE, StandardOpenOption_APPEND})
	out.write_byteArr(i8_([]byte("more\xff")))
	out.close()
	br := Files_newBufferedReader(p, StandardCharsets_ISO_8859_1)
	eq_(t, "line 1", br.readLine(), "hÃ©llo")
	eq_(t, "line 2", br.readLine(), "moreÿ")
	eq_(t, "end", br.readLine(), nullstr_)
	br.close()
	if ex := throws_(func() { Files_readString(p, StandardCharsets_UTF_8) }); !is_CharacterCodingException(ex) ||
		msgOf_(ex) != "java.nio.charset.MalformedInputException: Input length = 1" {
		t.Errorf("readString of malformed input threw %v", msgOf_(ex))
	}
	if ex := throws_(func() { Files_newBufferedReader(p, StandardCharsets_UTF_8).readLine() }); !is_CharacterCodingException(ex) {
		t.Errorf("newBufferedReader of malformed input threw %v", msgOf_(ex))
	}
	missing := Path_of(dir, []string{"nope.txt"})
	for _, f := range []func(){
		func() { Files_readString(missing, StandardCharsets_UTF_8) },
		func() { Files_newBufferedReader(missing, StandardCharsets_UTF_8) },
		func() { Files_getLastModifiedTime(missing, nil) },
		func() { Files_newOutputStream(Path_of(dir, []string{"no", "f"}), nil) },
	} {
		if ex := throws_(f); !is_IOException(ex) || !strings.HasPrefix(msgOf_(ex), "java.nio.file.NoSuchFileException: "+dir) {
			t.Errorf("missing file threw %v", msgOf_(ex))
		}
	}
	t1 := Files_getLastModifiedTime(p, []*LinkOption{})
	t2 := Files_getLastModifiedTime(p, nil)
	if !objequals_(t1, t2) || t1.compareTo(t2) != 0 || objhash_(t1) != objhash_(t2) {
		t.Error("FileTime equality")
	}
	eq_(t, "FileTime.toString", (&FileTime{1500000000 * 1e9}).toString(), "2017-07-14T02:40:00Z")
	eq_(t, "FileTime.toString with fraction", (&FileTime{1500000000*1e9 + 120e6}).toString(), "2017-07-14T02:40:00.12Z")

	// Guava's Files.
	f := Path_toFile(p)
	Files_io_asCharSink(f, StandardCharsets_UTF_8, []*FileWriteMode{}).write("one\ntwo\n")
	Files_io_asCharSink(f, StandardCharsets_UTF_8, []*FileWriteMode{FileWriteMode_APPEND}).write(new_StringBuilder_String("three"))
	src := Files_io_asCharSource(f, StandardCharsets_UTF_8)
	eq_(t, "CharSource.read", src.read(), "one\ntwo\nthree")
	eq_(t, "CharSource.openStream", readAll_(src.openStream()), "one\ntwo\nthree")
	eq_(t, "Files.newReader", Files_io_newReader(f, StandardCharsets_UTF_8).readLine(), "one")
	eq_(t, "CharSource.wrap", CharSource_wrap("abc").read(), "abc")
	if ex := throws_(func() { Files_io_asCharSource(new_File(dir+"/zz"), StandardCharsets_UTF_8).read() }); !is_FileNotFoundException(ex) {
		t.Errorf("CharSource of a missing file threw %v", msgOf_(ex))
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

func new_ObjectInputStream_() *ObjectInputStream {
	in := &ObjectInputStream{}
	in.self_ = in
	return in
}

func (this *testSaxHandler_) startElement(uri string, localName string, qName string, atts Attributes) {
	s := "<" + qName
	for _, name := range []string{"lang", "id", "x:y", "name", "a"} {
		if v := atts.getValue(name); v != nullstr_ {
			s += " " + name + "=" + v
		}
	}
	if uri != "" || localName != "" || atts.getValue("no-such") != nullstr_ {
		s += " ?!"
	}
	this.events = append(this.events, s+">")
}
