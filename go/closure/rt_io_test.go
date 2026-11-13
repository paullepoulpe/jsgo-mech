package closure

// Tests of rt_io.go against what the Java Io class returns.
//
// testdata/io/*.tsv is written by tools/iotable (run tools/iotable/run.sh to regenerate): one
// line per call of a real Io method, as "operation <tab> arguments <tab> result". A string is
// escaped (\uXXXX for everything outside printable ASCII and for the backslash, \N for null), a
// byte array is "x" + hex, a list is "[size]" and its items, and a failure, thrown or returned
// in an Outcome, is "!" + the exception's class + ": " + its message.

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func ioEsc_(s string) string {
	if s == nullstr_ {
		return `\N`
	}
	var b strings.Builder
	for _, c := range String_toCharArray(s) {
		if c < 0x20 || c > 0x7e || c == '\\' {
			fmt.Fprintf(&b, `\u%04x`, c)
		} else {
			b.WriteByte(byte(c))
		}
	}
	return b.String()
}

func ioUnesc_(s string) string {
	if s == `\N` {
		return nullstr_
	}
	var units []uint16
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			u, _ := strconv.ParseUint(s[i+2:i+6], 16, 16)
			units = append(units, uint16(u))
			i += 5
		} else {
			units = append(units, uint16(s[i]))
		}
	}
	return fromUnits_(units)
}

func ioBytes_(field string) []int8 {
	b, err := hex.DecodeString(strings.TrimPrefix(field, "x"))
	if err != nil {
		panic(err)
	}
	return i8_(b)
}

func ioFailure_(e any) string {
	s := "!" + Object_getClass(e).getName()
	if m := e.(interface{ getMessage() string }).getMessage(); m != nullstr_ {
		s += ": " + ioEsc_(m)
	}
	return s
}

func ioShow_(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case *Outcome_base:
		if x.isErr() {
			return ioFailure_(x.error().self_)
		}
		return ioShow_(x.get())
	case []int8:
		return "x" + hex.EncodeToString(u8_(x))
	case string:
		if x == nullstr_ {
			return "null"
		}
		return ioEsc_(x)
	case bool:
		return strconv.FormatBool(x)
	case *ArrayList:
		s := "[" + strconv.Itoa(int(x.size())) + "]"
		for it := x.iterator(); it.hasNext(); {
			s += "\t" + ioShow_(it.next())
		}
		return s
	}
	panic(fmt.Sprintf("cannot show %T", v))
}

// ioCall_ is the result of f in the table's notation.
func ioCall_(f func() any) (result string) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(interface{ getMessage() string }); !ok {
				panic(r)
			}
			result = ioFailure_(r)
		}
	}()
	return ioShow_(f())
}

// ioZlib_ matches the description of corrupt deflate data: zlib's words in Java, Go's here.
var ioZlib_ = regexp.MustCompile(`flate: corrupt input before offset \d+|invalid (block type|stored block lengths|code lengths set|distance too far back|distance code|literal/length code|bit length repeat)$`)

// ioTable_ runs every line of a table whose operation is in ops: the function gets the
// unescaped arguments, and its result must be the rest of the line.
func ioTable_(t *testing.T, file string, ops map[string]func(a []string) any) {
	data, err := os.ReadFile(filepath.Join("testdata", "io", file))
	if err != nil {
		t.Fatal(err)
	}
	ran, failed := 0, 0
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		fields := strings.Split(line, "\t")
		op := ops[fields[0]]
		if op == nil {
			continue
		}
		// The arguments end where the result starts: a list's "[n]", or the last field.
		n := len(fields) - 1
		for i, f := range fields[1:] {
			if strings.HasPrefix(f, "[") && strings.HasSuffix(f, "]") && fields[0] != "glob" || fields[0] == "glob" && i == 2 {
				n = i + 1
				break
			}
		}
		args := make([]string, n-1)
		for i, f := range fields[1:n] {
			args[i] = ioUnesc_(f)
			if strings.HasSuffix(fields[0], "#") && strings.HasPrefix(f, "x") {
				args[i] = f // hex: the operation decodes it
			}
		}
		want := strings.Join(fields[n:], "\t")
		ran++
		if got := ioCall_(func() any { return op(args) }); ioZlib_.ReplaceAllString(got, "(corrupt)") != ioZlib_.ReplaceAllString(want, "(corrupt)") {
			if failed++; failed <= 20 {
				t.Errorf("%s:\n got %s\nwant %s", strings.Join(fields[:n], " | "), got, want)
			}
		}
	}
	if ran == 0 || failed > 0 {
		t.Errorf("%s: %d lines run, %d failed", file, ran, failed)
	}
}

func TestIoPaths(t *testing.T) {
	ioTable_(t, "paths.tsv", map[string]func(a []string) any{
		"pathString":     func(a []string) any { return Io_pathString(a[0], a[1:]) },
		"pathFileName":   func(a []string) any { return Io_pathFileName(a[0]) },
		"pathParent":     func(a []string) any { return Io_pathParent(a[0]) },
		"pathNormalize":  func(a []string) any { return Io_pathNormalize(a[0]) },
		"pathResolve":    func(a []string) any { return Io_pathResolve(a[0], a[1]) },
		"pathRelativize": func(a []string) any { return Io_pathRelativize(a[0], a[1]) },
	})
	cwd, _ := os.Getwd()
	for path, want := range map[string]string{"": cwd, "a//b/": cwd + "/a/b", "../x": cwd + "/../x", "/a/": "/a", ".": cwd + "/."} {
		if got := Io_pathToAbsolute(path); got != want {
			t.Errorf("pathToAbsolute(%q) = %q, want %q", path, got, want)
		}
	}
	if Io_separator() != "/" {
		t.Error("separator")
	}
}

func TestIoURIs(t *testing.T) {
	ioTable_(t, "uris.tsv", map[string]func(a []string) any{
		"uriParse":  func(a []string) any { return Io_uriParse(a[0]) },
		"uriScheme": func(a []string) any { return Io_uriScheme(a[0]) },
		"uriPath":   func(a []string) any { return Io_uriPath(a[0]) },
	})
}

func TestIoText(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "f")
	write := func(hexBytes string) {
		if r := Io_writeBytes(tmp, ioBytes_(hexBytes)); r.isErr() {
			t.Fatal(r.errorMessage())
		}
	}
	ioTable_(t, "text.tsv", map[string]func(a []string) any{
		"splitLines":         func(a []string) any { return Io_splitLines(a[0]) },
		"base64Decode":       func(a []string) any { return Io_base64Decode(a[0]) },
		"base64Encode#":      func(a []string) any { return Io_base64Encode(ioBytes_(a[0])) },
		"utf8String#":        func(a []string) any { return Io_utf8String(ioBytes_(a[0])) },
		"utf8Bytes":          func(a []string) any { return Io_utf8Bytes(a[0]) },
		"isCharsetSupported": func(a []string) any { return Io_isCharsetSupported(a[0]) },
		"charsetName":        func(a []string) any { return Io_charsetName(a[0]) },
		"readText#": func(a []string) any {
			write(a[1])
			return Io_readText(tmp, a[0])
		},
		"readTextStrict#": func(a []string) any {
			write(a[1])
			return Io_readTextStrict(tmp, a[0])
		},
		"writeText": func(a []string) any {
			if r := Io_writeText(tmp, a[0], a[1]); r.isErr() {
				return r
			}
			return Io_readBytes(tmp)
		},
		// The chars at which the answer changes, each with the answer from there on.
		"canEncode": func(a []string) any {
			e, s := Io_newEncoder(a[0]), ""
			for c := 0; c < 0x10000; c++ {
				if can := e.canEncode(uint16(c)); c == 0 || can != e.canEncode(uint16(c-1)) {
					s += fmt.Sprintf("%c%04x", "-+"[map[bool]int{true: 1}[can]], c)
				}
			}
			return s
		},
	})
	// The JDK has these charsets; this runtime does not, and says so consistently.
	for _, name := range []string{"Shift_JIS", "ISO-8859-15", "UTF-32", "windows-1251", "EUC-JP"} {
		if Io_isCharsetSupported(name) {
			t.Errorf("%s is supported", name)
		}
		if got := ioCall_(func() any { return Io_charsetName(name) }); got != "!java.nio.charset.UnsupportedCharsetException: "+name {
			t.Errorf("charsetName(%s): %s", name, got)
		}
	}
}

// TestIoGlob builds the tree the table was made from and compares what each glob finds. The
// table's lists are sorted: the walk's order is the directory's own, which TestIoWalkOrder checks.
func TestIoGlob(t *testing.T) {
	dir := t.TempDir()
	data, _ := os.ReadFile(filepath.Join("testdata", "io", "glob.tsv"))
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Split(line, "\t")
		switch f[0] {
		case "file":
			path := filepath.Join(dir, "T", ioUnesc_(f[1]))
			os.MkdirAll(filepath.Dir(path), 0o777)
			if err := os.WriteFile(path, nil, 0o666); err != nil {
				t.Fatal(err)
			}
		case "link":
			if err := os.Symlink(ioUnesc_(f[2]), filepath.Join(dir, "T", ioUnesc_(f[1]))); err != nil {
				t.Fatal(err)
			}
		}
	}
	// The table's paths are relative to the directory that holds T.
	old, _ := os.Getwd()
	ioTable_(t, "glob.tsv", map[string]func(a []string) any{"glob": func(a []string) any {
		os.Chdir(dir)
		defer os.Chdir(old)
		r := Io_listFilesMatchingGlob(a[0], a[1])
		if r.isErr() {
			return r
		}
		var names []string
		for it := r.get().(*ArrayList).iterator(); it.hasNext(); {
			names = append(names, it.next().(string))
		}
		sort.Strings(names)
		return listOfStrings_(names)
	}})
}

// TestIoWalkOrder: files come in the order the operating system lists each directory, as in
// Java, not sorted.
func TestIoWalkOrder(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 300; i++ {
		sub := filepath.Join(dir, strconv.Itoa(i%7))
		os.MkdirAll(sub, 0o777)
		os.WriteFile(filepath.Join(sub, strconv.Itoa((i*7919)%1000)+".js"), nil, 0o666)
	}
	var want []string
	top, _ := dirNames_(dir)
	for _, d := range top {
		names, _ := dirNames_(filepath.Join(dir, d))
		for _, n := range names {
			want = append(want, filepath.Join(dir, d, n))
		}
	}
	got := ioShow_(Io_listFilesMatchingGlob(dir, dir+"/**.js"))
	if got != ioShow_(listOfStrings_(want)) || len(want) != 300 {
		t.Errorf("walk order differs from directory order (%d files)", len(want))
	}
	if sort.StringsAreSorted(want) {
		t.Log("this file system lists directories sorted: the order check proves nothing here")
	}
	if got := ioShow_(Io_listDirectory(dir)); got != ioShow_(listOfStrings_(func() (paths []string) {
		for _, d := range top {
			paths = append(paths, dir+"/"+d)
		}
		return
	}())) {
		t.Errorf("listDirectory: %s", got)
	}
}

// The expected texts below were printed by the Java Io class for the same calls (a throwaway
// program; see the commit that added this test).
func TestIoFiles(t *testing.T) {
	old, _ := os.Getwd()
	defer os.Chdir(old)
	os.Chdir(t.TempDir())
	os.Mkdir("d", 0o777)
	os.WriteFile("f", nil, 0o666)
	os.WriteFile("junk", []byte("hello this is not a zip"), 0o666)
	for _, c := range []struct {
		name string
		got  any
		want string
	}{
		{"readBytes missing", Io_readBytes("nope"), "!java.io.FileNotFoundException: nope (No such file or directory)"},
		{"readBytes dir", Io_readBytes("d"), "!java.io.FileNotFoundException: d (Is a directory)"},
		{"readBytes under file", Io_readBytes("f//x/"), "!java.io.FileNotFoundException: f/x (Not a directory)"},
		{"readBytes empty", Io_readBytes(""), "!java.io.FileNotFoundException:  (No such file or directory)"},
		{"readText slash", Io_readText("f/", "UTF-8"), ""},
		{"readTextStrict missing", Io_readTextStrict("nope", "UTF-8"), "!java.nio.file.NoSuchFileException: nope"},
		{"readTextStrict dir", Io_readTextStrict("d", "UTF-8"), "!java.io.IOException: Is a directory"},
		{"readTextStrict under file", Io_readTextStrict("f/x", "UTF-8"), "!java.nio.file.FileSystemException: f/x: Not a directory"},
		{"readTextStrict empty", Io_readTextStrict("", "UTF-8"), "!java.io.IOException: Is a directory"},
		{"writeBytes dir", Io_writeBytes("d", nil), "!java.io.FileNotFoundException: d (Is a directory)"},
		{"writeBytes no dir", Io_writeBytes("nodir/x", nil), "!java.io.FileNotFoundException: nodir/x (No such file or directory)"},
		{"writeBytes under file", Io_writeBytes("f/x", nil), "!java.io.FileNotFoundException: f/x (Not a directory)"},
		{"writeBytes empty", Io_writeBytes("", nil), "!java.io.FileNotFoundException:  (No such file or directory)"},
		{"writeBytes full", Io_writeBytes("/dev/full", []int8{1}), "!java.io.IOException: No space left on device"},
		{"openOutput dir", Io_openOutput("d", "UTF-8", true), "!java.io.FileNotFoundException: d (Is a directory)"},
		{"createNewFile exists", Io_createNewFile("f"), "false"},
		{"createNewFile no dir", Io_createNewFile("nodir/x"), "!java.io.IOException: No such file or directory"},
		{"createNewFile under file", Io_createNewFile("f/x"), "!java.io.IOException: Not a directory"},
		{"createNewFile dir", Io_createNewFile("d"), "false"},
		{"createNewFile empty", Io_createNewFile(""), "!java.io.IOException: No such file or directory"},
		{"createNewFile new", Io_createNewFile("new"), "true"},
		{"mkdirs", []bool{Io_mkdirs("d"), Io_mkdirs("f/x"), Io_mkdirs("n1/n2"), Io_mkdirs("n1/n2"), Io_mkdirs(""), Io_mkdirs("n3/../n4/n5"), Io_exists("n3"), Io_exists("n4/n5")},
			"[false false true false false true false true]"},
		{"exists", []bool{Io_exists(""), Io_isDirectory(""), Io_exists("."), Io_exists("f/"), Io_exists("f/x"), Io_isDirectory("d/"), Io_isDirectory("f")},
			"[false false true true false true false]"},
		{"listDirectory file", Io_listDirectory("f") == nil && Io_listDirectory("nope") == nil && Io_listDirectory("") == nil, "true"},
		{"listDirectory", Io_listDirectory("n1/"), "[1]\tn1/n2"},
		{"listZipEntryNames missing", Io_listZipEntryNames("nope.zip", "UTF-8"), "!java.io.FileNotFoundException: nope.zip (No such file or directory)"},
		{"listZipEntryNames dir", Io_listZipEntryNames("d", "UTF-8"), "!java.io.FileNotFoundException: d (Is a directory)"},
		{"listZipEntryNames empty", Io_listZipEntryNames("f", "UTF-8"), "[0]"},
		{"listZipEntryNames junk", Io_listZipEntryNames("junk", "UTF-8"), "[0]"},
		{"readZipEntryText missing", Io_readZipEntryText("nope.zip", "x", "UTF-8"), "!java.nio.file.NoSuchFileException: nope.zip"},
		{"readZipEntryText empty", Io_readZipEntryText("f", "x", "UTF-8"), "!java.util.zip.ZipException: zip file is empty"},
		{"readZipEntryText dir", Io_readZipEntryText("d", "x", "UTF-8"), "!java.io.FileNotFoundException: d (Is a directory)"},
		{"readZipEntryText junk", Io_readZipEntryText("junk", "x", "UTF-8"), "!java.util.zip.ZipException: zip END header not found"},
		{"glob missing dir", Io_listFilesMatchingGlob("nope", "nope/**"), "[0]"},
		{"glob file", Io_listFilesMatchingGlob("f", "**"), "[1]\tf"},
	} {
		got, ok := c.got.(string)
		if !ok {
			if got = fmt.Sprint(c.got); fmt.Sprintf("%T", c.got) != "[]bool" && fmt.Sprintf("%T", c.got) != "bool" {
				got = ioShow_(c.got)
			}
		}
		if got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestIoOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.txt")
	read := func() string { return ioShow_(Io_readBytes(path)) }
	// Java: [63, 63, 63, 63, 110, 117, 108, 108, 98, 99, 110, 117, 108, 108, 110, 117, 108, 108, 10]
	o := Io_openOutput(path, "US-ASCII", false).get().(*Io_Output)
	o.print(ioUnesc_(`\u00e9\ud83d\ude00\ud800`))
	o.append_char(0xd83d)
	o.append_char(0xde00) // the two halves make one character
	o.append_CharSequence(nil)
	o.append_CharSequence_int_int("abcdef", 1, 3)
	o.print(nullstr_)
	o.println(nullstr_)
	if got := read(); got != "x" {
		t.Errorf("before close: %s", got)
	}
	if r := o.close(); r.isErr() || read() != "x3f3f3f3f6e756c6c62636e756c6c6e756c6c0a" {
		t.Errorf("after close: %s %s", ioShow_(r), read())
	}
	o.print("x")
	if got := ioShow_(o.close()); got != "!java.io.IOException: Stream closed" {
		t.Errorf("print after close: %s", got)
	}
	// Buffering: text waits until 8192 chars have gathered, their bytes until the next ones
	// would not fit into 8192. Each line is what Java printed for the file's size after printing
	// that many chars (tools/iotable has no table for this: sizes observed with a JDK 21).
	for _, c := range []struct {
		char  string
		steps string
	}{
		{"a", "+8191=0 +2=0 +8191=8192 +8192=16384 +1=16384 +8192=24576 +8192=32768 flush=40961 close=40971"},
		{"a", "+100=0 +100=0 +20000=8192 +1=8192 +30000=40960 +5000=40960 +3000=49152 flush=58201 close=58211"},
		{"a", "+4096=0 +4096=0 +4096=0 +4096=8192 +4096=8192 +4096=16384 flush=24576 close=24586"},
		{`\u00e9`, "+8191=0 +2=8192 +8191=24576 +8192=40960 +1=40960 +8192=57344 +8192=73728 flush=81922 close=81942"},
		{`\u00e9`, "+513=0 +511=0 +1024=0 +7000=8192 +200=8192 +8192=24576 +8192=40960 flush=51264 close=51284"},
		{`\u4e2d`, "+8191=0 +2=24570 +8191=49140 +8192=73710 +1=73710 +8192=98280 +8192=122850 flush=122883 close=122913"},
		{`\u4e2d`, "+8193=24570 +8191=49140 +16384=98280 +16385=147420 flush=147459 close=147489"},
		{`\ud83d\ude00`, "+4095=0 +1=8192 +4095=8192 +4096=24576 +0=24576 +4096=40960 +4096=57344 flush=81916 close=81956"},
		{`\ud83d\ude00`, "+256=0 +255=0 +512=0 +3500=8192 +100=8192 +4096=24576 +4096=40960 flush=51260 close=51300"},
	} {
		o = Io_openOutput(path, "UTF-8", false).get().(*Io_Output)
		char, got := ioUnesc_(c.char), ""
		size := func() string {
			st, _ := os.Stat(path)
			return "=" + strconv.Itoa(int(st.Size()))
		}
		for _, step := range strings.Fields(c.steps) {
			count, _, _ := strings.Cut(step, "=")
			if n, err := strconv.Atoi(count); err == nil {
				o.print(strings.Repeat(char, n))
			} else if count == "flush" {
				o.flush()
			} else {
				o.print(strings.Repeat(char, 10))
				o.close()
			}
			got += " " + count + size()
		}
		if got != " "+c.steps {
			t.Errorf("%s:\n got%s\nwant %s", c.char, got, c.steps)
		}
	}
	o = Io_openOutput(path, "UTF-8", false).get().(*Io_Output)
	o.print("0123456789")
	o.close()
	// Appending keeps the content.
	o = Io_openOutput(path, "UTF-16", true).get().(*Io_Output)
	o.print("z")
	o.close()
	if st, _ := os.Stat(path); st.Size() != 10+4 {
		t.Errorf("after append: %d bytes", st.Size())
	}
	// A write error is reported by close, once; the first one wins.
	full := Io_openOutput("/dev/full", "UTF-8", false).get().(*Io_Output)
	full.print("abc")
	full.flush()
	full.print("def")
	if got := ioShow_(full.close()); got != "!java.io.IOException: No space left on device" {
		t.Errorf("/dev/full: %s", got)
	}
	n := Io_nullOutput()
	n.print("x")
	n.close()
	n.print("y")
	n.flush()
	if got := ioShow_(n.close()); got != "null" {
		t.Errorf("null output: %s", got)
	}
}

// TestIoConsole replays a sequence of prints to System.out, directly and through encodedAs, for
// which Java (JDK 21) wrote the bytes below in the POSIX locale and in a UTF-8 one.
func TestIoConsole(t *testing.T) {
	for charset, want := range map[string]string{
		"US-ASCII": "a?c\nb\u00e9??x?null\ntail",
		"UTF-8":    "a\u00e9c\nb\u00e9\U0001F600?x?null\ntail",
	} {
		r, w, _ := os.Pipe()
		console := &console_{file: w, enc: encoder_{cs: charset_of_(charset)}}
		out := &Io_Output{console: console}
		out.print("a\u00e9")
		enc := out.encodedAs("UTF-8")
		enc.print("b\u00e9") // waits in the writer's buffer
		out.println("c")
		enc.flush()
		out.print(ioUnesc_(`\ud83d`)) // the two halves of a pair, from two sinks
		(&Io_Output{console: console}).print(ioUnesc_(`\ude00`))
		out.print(ioUnesc_(`\ud83d`))
		out.print("x")
		out.append_char(0xd83d)
		out.flush()
		out.println(nullstr_)
		enc.print("tail")
		if got := ioShow_(enc.close()); got != "null" { // closes the console too
			t.Errorf("%s: close: %s", charset, got)
		}
		out.print("after")
		enc.print("more")
		if got := ioShow_(enc.close()) + " " + ioShow_(out.close()); got != "!java.io.IOException: Stream closed null" {
			t.Errorf("%s: second close: %s", charset, got)
		}
		buf := make([]byte, 100)
		n, _ := r.Read(buf)
		if string(buf[:n]) != want {
			t.Errorf("%s: wrote %q, want %q", charset, buf[:n], want)
		}
		r.Close()
	}
	// The console's charset is the locale's, as the JDK sees it.
	for env, want := range map[string]string{
		"||": "US-ASCII", "|POSIX|C.UTF-8": "US-ASCII", "||C.UTF-8": "UTF-8", "C.utf8|POSIX|POSIX": "UTF-8",
		"C||": "US-ASCII", "||en_US.ISO-8859-1@euro": "ISO-8859-1", "||de_DE": "US-ASCII",
	} {
		v := strings.Split(env, "|")
		t.Setenv("LC_ALL", v[0])
		t.Setenv("LC_CTYPE", v[1])
		t.Setenv("LANG", v[2])
		if got := localeCharset_().name; got != want {
			t.Errorf("locale %s: %s, want %s", env, got, want)
		}
	}
}

// TestIoZip: archives written by java.util.zip, whole, cut short at every length, and with single
// header bytes damaged. An entry is shown as name=data.
func TestIoZip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.zip")
	ioTable_(t, "zip.tsv", map[string]func(a []string) any{"readZipEntries#": func(a []string) any {
		r := Io_readZipEntries(ioBytes_(a[1]), a[0])
		// The same archive as a file gives the same names, or the same failure.
		Io_writeBytes(path, ioBytes_(a[1]))
		names := Io_listZipEntryNames(path, a[0])
		if r.isErr() {
			if ioShow_(names) != ioShow_(r) {
				t.Errorf("listZipEntryNames: %s, readZipEntries: %s", ioShow_(names), ioShow_(r))
			}
			return r
		}
		var entries, entryNames []string
		for it := r.get().(*ArrayList).iterator(); it.hasNext(); {
			e := it.next().(*Io_ZipEntryData)
			entries = append(entries, e.name()+"="+ioShow_(e.data()))
			entryNames = append(entryNames, e.name())
		}
		if ioShow_(names) != ioShow_(listOfStrings_(entryNames)) {
			t.Errorf("listZipEntryNames: %s", ioShow_(names))
		}
		return listOfStrings_(entries)
	}})
}

func TestIoGzip(t *testing.T) {
	plain := "hello hello hello"
	// What Java's Io.gzip returns for it.
	java := []int8{31, -117, 8, 0, 0, 0, 0, 0, 0, -1, -53, 72, -51, -55, -55, 87, -56, 64, -112, 0, -128, -120, -7, -27, 17, 0, 0, 0}
	twice := append(append([]int8{}, java...), java...)
	bad := func(i int, d int8) []int8 {
		b := append([]int8{}, java...)
		b[i] += d
		return b
	}
	want := "x" + hex.EncodeToString([]byte(plain))
	for _, c := range []struct {
		name string
		in   []int8
		want string
	}{
		{"java's bytes", java, want},
		{"round trip", Io_gzip(i8_([]byte(plain))), want},
		{"two members", twice, want + hex.EncodeToString([]byte(plain))},
		{"trailing zeros", append(append([]int8{}, java...), 0, 0, 0, 0, 0), want},
		{"empty", nil, "!java.io.EOFException"},
		{"not gzip", []int8{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, "!java.util.zip.ZipException: Not in GZIP format"},
		{"other method", []int8{0x1f, -117, 7, 0, 0, 0, 0, 0, 0, 0, 11, 12}, "!java.util.zip.ZipException: Unsupported compression method"},
		{"cut in trailer", java[:len(java)-3], "!java.io.EOFException"},
		{"cut in data", java[:len(java)-9], "!java.io.EOFException: Unexpected end of ZLIB input stream"},
		{"wrong length", bad(len(java)-1, 1), "!java.util.zip.ZipException: Corrupt GZIP trailer"},
	} {
		if got := ioShow_(Io_gunzip(c.in)); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
	if g := Io_gzip(i8_([]byte(plain))); hex.EncodeToString(u8_(g[:10])) != hex.EncodeToString(u8_(java[:10])) {
		t.Errorf("gzip header: %v", g[:10])
	}
}

// TestIoOrderDump is a tool, not a test: with IO_ORDER_DIR set it prints what the walk and the
// directory listing return for that directory, to compare with Java's on the same directory.
func TestIoOrderDump(t *testing.T) {
	dir := os.Getenv("IO_ORDER_DIR")
	if dir == "" {
		t.Skip("IO_ORDER_DIR is not set")
	}
	var b strings.Builder
	for it := Io_listFilesMatchingGlob(dir, dir+"/**").get().(*ArrayList).iterator(); it.hasNext(); {
		b.WriteString(it.next().(string) + "\n")
	}
	for it := Io_listDirectory(dir).(*ArrayList).iterator(); it.hasNext(); {
		b.WriteString(it.next().(string) + "\n")
	}
	os.WriteFile(os.Getenv("IO_ORDER_OUT"), []byte(b.String()), 0o666)
}
