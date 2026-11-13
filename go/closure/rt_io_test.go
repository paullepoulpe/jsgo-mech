package closure

// Tests of rt_io.go against what the Java Io class returns.
//
// testdata/io/*.tsv is written by tools/iotable (run tools/iotable/run.sh to regenerate): one
// line per call of a real Io method, as "operation <tab> argument <tab> result". A string is
// escaped (\uXXXX for everything outside printable ASCII and for the backslash, \N for null), a
// byte array is "x" + hex, a list is "[size]" and its items, and a failure, thrown or returned
// in an Outcome, is "!" + the exception's class + ": " + its message.

import (
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

var ioTables_, _ = filepath.Abs(filepath.Join("testdata", "io"))

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

func ioUnits_(s string) []uint16 {
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
	return units
}

func ioUnesc_(s string) string {
	if s == `\N` {
		return nullstr_
	}
	return fromUnits_(ioUnits_(s))
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

// ioRaw_ is a result that is in the table's notation already.
type ioRaw_ string

func ioShow_(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case ioRaw_:
		return string(x)
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
	case int64:
		return strconv.FormatInt(x, 10)
	case *Optional:
		return ioShow_(x.orElse(nil))
	case *SimpleJsonValue:
		return ioEsc_(x.toString())
	case List:
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

// ioTable_ runs every line of a table: the function of its operation gets the argument
// (unescaped, or as it is for an operation whose name ends in "#": hex), and its result must be
// the rest of the line.
func ioTable_(t *testing.T, file string, ops map[string]func(a string) any) {
	data, err := os.ReadFile(filepath.Join(ioTables_, file))
	if err != nil {
		t.Fatal(err)
	}
	ran, failed := 0, 0
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		fields := strings.SplitN(line, "\t", 3)
		op := ops[fields[0]]
		if op == nil {
			t.Fatalf("%s: no operation %s", file, fields[0])
		}
		arg := fields[1]
		if !strings.HasSuffix(fields[0], "#") {
			arg = ioUnesc_(arg)
		}
		ran++
		if got := ioCall_(func() any { return op(arg) }); got != fields[2] {
			if failed++; failed <= 20 {
				t.Errorf("%s %s:\n got %s\nwant %s", fields[0], fields[1], got, fields[2])
			}
		}
	}
	if ran == 0 || failed > 0 {
		t.Errorf("%s: %d lines run, %d failed", file, ran, failed)
	}
}

func TestIoUtf8(t *testing.T) {
	ioTable_(t, "utf8.tsv", map[string]func(a string) any{
		"utf8#": func(a string) any {
			return ioRaw_(ioShow_(Io_utf8String(ioBytes_(a))) + "\t" + ioShow_(Io_utf8StringStrict(ioBytes_(a))))
		},
		"utf8Bytes": func(a string) any { return Io_utf8Bytes(a) },
	})
	// The same texts put together one char at a time: a surrogate pair is then two halves next
	// to each other in the Go string (rt_string.go), and must still come out as one character.
	data, _ := os.ReadFile(filepath.Join(ioTables_, "utf8.tsv"))
	n := 0
	for _, line := range strings.Split(string(data), "\n") {
		if fields := strings.Split(line, "\t"); fields[0] == "utf8Bytes" {
			s := ""
			for _, u := range ioUnits_(fields[1]) {
				s += fromUnits_([]uint16{u})
			}
			if got := ioShow_(Io_utf8Bytes(s)); got != fields[2] {
				t.Errorf("utf8Bytes, char by char, %s: got %s, want %s", fields[1], got, fields[2])
			}
			n++
		}
	}
	if n < 500 {
		t.Errorf("%d utf8Bytes lines", n)
	}
}

// ioSummary_ is the length and CRC-32 of the data, or the failure.
func ioSummary_(data *Outcome_base) any {
	if data.isErr() {
		return data
	}
	b := u8_(data.get().([]int8))
	return ioRaw_(fmt.Sprintf("%d %x", len(b), crc32.ChecksumIEEE(b)))
}

func TestIoCodecs(t *testing.T) {
	ioTable_(t, "codecs.tsv", map[string]func(a string) any{
		"base64Encode#": func(a string) any { return Io_base64Encode(ioBytes_(a)) },
		"base64Decode": func(a string) any { // the contract names the class of the failure only
			r := ioCall_(func() any { return Io_base64Decode(a) })
			if strings.HasPrefix(r, "!") {
				r, _, _ = strings.Cut(r, ":")
			}
			return ioRaw_(r)
		},
		"inflate#": func(a string) any { return ioSummary_(Io_inflate(ioBytes_(a))) },
		"gunzip#":  func(a string) any { return ioSummary_(Io_gunzip(ioBytes_(a))) },
	})
	// gzip is not zlib's bytes; gunzip gives the data back.
	for _, size := range []int{0, 1, 100, 100000} {
		data := make([]int8, size)
		for i := range data {
			data[i] = int8(i * i / 7)
		}
		gz := Io_gzip(data)
		if back := Io_gunzip(gz); back.isErr() || string(u8_(back.get().([]int8))) != string(u8_(data)) {
			t.Errorf("gunzip(gzip) of %d bytes", size)
		}
		if u8_(gz)[0] != 0x1f || u8_(gz)[1] != 0x8b || size == 100000 && len(gz) > size/2 {
			t.Errorf("gzip of %d bytes: %d bytes", size, len(gz))
		}
	}
}

// ioSinkScript_ opens the file, writes to it, closes it twice and reads it back.
func ioSinkScript_(path string, appendTo bool) any {
	opened := Io_openFile(path, appendTo)
	if opened.isErr() {
		return opened
	}
	sink := opened.get().(*Io_Sink)
	sink.write(Io_utf8Bytes("one "))
	sink.write([]int8{})
	sink.write(Io_utf8Bytes("two"))
	first := ioShow_(sink.close())
	sink.write(Io_utf8Bytes(" never"))
	return ioRaw_(first + " " + ioShow_(sink.close()) + " " + ioShow_(Io_readBytes(path)))
}

// TestIoFiles replays the table in an empty directory: its first lines make the tree that the
// generator made, the others are calls, in the generator's order (some write).
func TestIoFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	check := func(err error) any {
		if err != nil {
			t.Fatal(err)
		}
		return nil
	}
	ioTable_(t, "files.tsv", map[string]func(a string) any{
		"dir":  func(a string) any { return check(os.MkdirAll(a, 0o777)) },
		"file": func(a string) any { return check(os.WriteFile(a, []byte(a), 0o666)) },
		"link": func(a string) any {
			name, target, _ := strings.Cut(a, " -> ")
			return check(os.Symlink(target, name))
		},
		"mtime": func(a string) any {
			return check(os.Chtimes(a, time.Time{}, time.Unix(1000000000, 123456789)))
		},
		"exists":       func(a string) any { return Io_exists(a) },
		"isDirectory":  func(a string) any { return Io_isDirectory(a) },
		"readBytes":    func(a string) any { return Io_readBytes(a) },
		"listFiles":    func(a string) any { return Io_listFiles(a) },
		"lastModified": func(a string) any { return Io_lastModified(a) },
		"writeBytes": func(a string) any {
			return ioRaw_(ioShow_(Io_writeBytes(a, Io_utf8Bytes("written"))) + " " + ioShow_(Io_readBytes(a)))
		},
		"truncate": func(a string) any { return ioSinkScript_(a, false) },
		"append":   func(a string) any { return ioSinkScript_(a, true) },
		"mkdirs": func(a string) any {
			Io_mkdirs(a)
			return Io_isDirectory(a)
		},
	})
	if cwd, _ := filepath.EvalSymlinks(t.TempDir()); !strings.HasPrefix(Io_workingDirectory(), filepath.Dir(cwd)) || !filepath.IsAbs(Io_workingDirectory()) {
		t.Errorf("workingDirectory: %s", Io_workingDirectory())
	}
	// What the table cannot hold when it is generated by root, who may read and write anything.
	if os.Geteuid() != 0 {
		os.Chmod("T/a.js", 0)
		os.Chmod("T/other", 0)
		for call, want := range map[string]string{
			ioShow_(Io_readBytes("T/a.js")):               "!java.io.IOException: T/a.js: Cannot read file",
			ioShow_(Io_writeBytes("T/a.js", []int8{})):    "!java.io.IOException: T/a.js: Cannot write file",
			ioShow_(Io_openFile("T/a.js", true)):          "!java.io.IOException: T/a.js: Cannot write file",
			ioShow_(Io_writeBytes("T/other/n", []int8{})): "!java.io.IOException: T/other/n: Cannot write file",
			ioShow_(Io_listFiles("T/other")):              "[0]",
		} {
			if call != want {
				t.Errorf("got %s, want %s", call, want)
			}
		}
		os.Chmod("T/other", 0o777)
	}
}

func ioThrowable_(kind string) *Throwable {
	switch kind {
	case "plain":
		return new_RuntimeException_String("something\nin two lines").as_Throwable()
	case "bare":
		return new_IllegalStateException_0().as_Throwable()
	case "io":
		return new_IOException_String("x: No such file").as_Throwable()
	case "caused":
		return new_RuntimeException_String_Throwable("outer", new_IllegalArgumentException_String("inner").as_Throwable()).as_Throwable()
	case "wrapped":
		return new_RuntimeException_Throwable(new_IOException_String("inner").as_Throwable()).as_Throwable()
	case "chain":
		inner := new_RuntimeException_String_Throwable("b", new_NullPointerException_0().as_Throwable())
		return new_IllegalStateException_String_Throwable("a", inner.as_Throwable()).as_Throwable()
	}
	panic(kind)
}

func TestIoMisc(t *testing.T) {
	ioTable_(t, "misc.tsv", map[string]func(a string) any{
		// The table holds Java's text without the lines that say where ("at ...").
		"stackTraceText": func(a string) any { return Io_stackTraceText(ioThrowable_(a)) + "\n" },
		"readResource": func(a string) any {
			data := Io_readResource(a)
			if !data.isPresent() {
				return nil
			}
			return ioSummary_(Outcome_base_ok_Object(data.get()))
		},
	})
}

// TestIoStandardStreams: a sink of a standard stream writes at once, ignores a failure, and
// once closed stays closed for every sink of that stream; standard input is read whole.
func TestIoStandardStreams(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out")
	file, _ := os.Create(path)
	saved := *stdout_
	defer func() { *stdout_ = saved }()
	stdout_.file = file
	Io_stdout().write(Io_utf8Bytes("a"))
	if b, _ := os.ReadFile(path); string(b) != "a" {
		t.Errorf("not written at once: %q", b)
	}
	if r := Io_stdout().close(); r.isErr() {
		t.Error("close failed")
	}
	Io_stdout().write(Io_utf8Bytes("b"))
	if r := Io_stdout().close(); r.isErr() {
		t.Error("second close failed")
	}
	if b, _ := os.ReadFile(path); string(b) != "a" {
		t.Errorf("written after close: %q", b)
	}
	// A stream that cannot be written (it is closed underneath): no failure is reported.
	*stdout_ = Io_Sink{file: file}
	Io_stdout().write(Io_utf8Bytes("c"))
	if r := Io_stdout().close(); r.isErr() {
		t.Error("failure of a standard stream reported")
	}
	// The same for a file is reported by close.
	broken, _ := os.Create(path)
	broken.Close()
	sink := &Io_Sink{file: broken, path: "p"}
	sink.write(Io_utf8Bytes("d"))
	if got := ioShow_(sink.close()); got != "!java.io.IOException: p: Cannot write file" {
		t.Errorf("close after a failed write: %s", got)
	}

	os.WriteFile(path, []byte("in\x00\xff"), 0o666)
	in, _ := os.Open(path)
	if got := ioShow_((&Io_Input{in}).readBytes()); got != "x696e00ff" {
		t.Errorf("readBytes: %s", got)
	}
	in.Close()
	if got := ioShow_((&Io_Input{in}).readBytes()); got != "!java.io.IOException: stdin: Cannot read file" {
		t.Errorf("readBytes of a closed input: %s", got)
	}
}
