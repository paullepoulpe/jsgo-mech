package closure

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"
)

// The expectations in rt_base_data_test.go were produced by running the real
// JDK 21 and Guava on the same inputs.

func baseTry_(f func() string) (out string) {
	defer func() {
		if r := recover(); r != nil {
			ex := baseCatch_(r)
			name := fmt.Sprintf("%T", ex)
			name = name[strings.LastIndexAny(name, ".*")+1:]
			out = "EX:" + name
		}
	}()
	return f()
}

func baseArr_(a []string) string {
	return strconv.Itoa(len(a)) + strings.Repeat("\x01", min(len(a), 1)) + strings.Join(a, "\x01")
}

func baseStrs_(l []any) []string {
	out := make([]string, len(l))
	for i, v := range l {
		out[i] = v.(string)
	}
	return out
}

func baseArg_(a string) any {
	v := a[2:]
	switch a[0] {
	case 'i':
		n, _ := strconv.ParseInt(v, 10, 32)
		return int32(n)
	case 'l':
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	case 's':
		return v
	case 'd':
		return Double_parseDouble(v)
	case 'f':
		f, _ := strconv.ParseFloat(v, 32)
		return float32(f)
	case 'c':
		return uint16(v[0])
	case 'b':
		return v == "true"
	case 'n':
		return nil
	case 'B':
		return new_BigInteger_String(v)
	}
	panic(a)
}

func baseArgs_(as []string) []any {
	out := make([]any, len(as))
	for i, a := range as {
		out[i] = baseArg_(a)
	}
	return out
}

func baseBytes_(h string) []int8 {
	raw, _ := hex.DecodeString(h)
	out := make([]int8, len(raw))
	for i, b := range raw {
		out[i] = int8(b)
	}
	return out
}

func baseHex_(b []int8) string {
	raw := make([]byte, len(b))
	for i, x := range b {
		raw[i] = byte(x)
	}
	return hex.EncodeToString(raw)
}

func baseNull_(s string) string {
	if s == nullstr_ {
		return "<null>"
	}
	return s
}

func baseBool_(b bool) string { return strconv.FormatBool(b) }

func baseInt_(i int64) string { return strconv.FormatInt(i, 10) }

func baseCaseFormat_(n string) CaseFormat {
	for _, c := range []CaseFormat{CaseFormat_LOWER_HYPHEN, CaseFormat_LOWER_UNDERSCORE, CaseFormat_LOWER_CAMEL, CaseFormat_UPPER_CAMEL, CaseFormat_UPPER_UNDERSCORE} {
		if c.name() == n {
			return c
		}
	}
	panic(n)
}

// Java regex features the RE2 translation deliberately rejects, and one case
// where RE2 is more lenient than Java (`a{2` is a literal).
var baseRegexUnsupported_ = map[string]bool{
	"[a-z&&[^b]]": true, "(a)\\1": true, "a(?=b)": true, "a*+": true, "a{2": true,
	// Java advances one UTF-16 unit after an empty match, so it also reports a
	// match between the two halves of a surrogate pair; here the step is a code point.
	"x*": true,
	// The last iteration of a (...)* over an empty alternative: Java leaves the
	// group empty, RE2 keeps the previous iteration. The match itself is the same.
	"^(?:((\\w+\\.)+prototype\\.||window\\.)*(setUpPage|setUp|shouldRunTests|tearDown|tearDownPage|test[\\w\\$]+))$": true,
}

func TestBaseMatcher(t *testing.T) {
	m := Pattern_compile("(?P<func>a+)(?P<args>b*)").matcher("xaab")
	if !m.find() || m.group_String("func") != "aa" || m.group_String("args") != "b" {
		t.Errorf("named groups")
	}
	// UTF-16 indexes.
	m = Pattern_compile("b+").matcher("\U0001F600ébb\U0001F600b")
	if !m.find() || m.start() != 3 || m.end() != 5 || !m.find() || m.start() != 7 || m.find() {
		t.Errorf("utf-16 offsets")
	}
	if ex := baseTry_(func() string { Pattern_compile("(a"); return "" }); ex != "EX:PatternSyntaxException" {
		t.Errorf("got %q", ex)
	}
	if got := baseArr_(Pattern_compile(",").split_String_int("a,b,,", 0)); got != baseArr_([]string{"a", "b"}) {
		t.Errorf("split: %s", got)
	}
	if got := Pattern_compile("(a)(b)?").matcher("xab-a").replaceAll("[$1$2]"); got != "x[ab]-[a]" {
		t.Errorf("replaceAll: %s", got)
	}
}

// TestStringUnits checks stringUnits_ and stringUnitsFrom_ against String_length and String_charAt, on short and long
// strings with surrogate pairs, lone surrogates and bytes that are not UTF-8, and asciiCharAt_
// against String_charAt.
func TestStringUnits(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	pieces := []string{"a", "z", "\x00", "é", " ", "\U0001F600", "\xff", "\xe2\x82", fromUnits_([]uint16{0xD800}), fromUnits_([]uint16{0xDC00})}
	for round := 0; round < 500; round++ {
		var b strings.Builder
		n := r.Intn(8)
		if round%5 == 0 {
			n = 300 + r.Intn(100)
		}
		for i := 0; i < n; i++ {
			if r.Intn(4) == 0 {
				b.WriteString(pieces[r.Intn(len(pieces))])
			} else {
				b.WriteByte(byte('a' + r.Intn(26)))
			}
		}
		s := b.String()
		next := int32(0)
		for i, c := range stringUnits_(s) {
			if i != next || c != String_charAt(s, i) {
				t.Fatalf("%q: unit %d is %#x, charAt %#x", s, i, c, String_charAt(s, i))
			}
			next++
		}
		if next != String_length(s) {
			t.Fatalf("%q: %d units, length %d", s, next, String_length(s))
		}
		for i := range stringUnits_(s) { // stopping early
			if i == 2 {
				break
			}
		}
		// stringUnitsFrom_ from every start, a low surrogate half and the end included
		for from := int32(0); from <= next+1; from++ {
			k := from
			for i, c := range stringUnitsFrom_(s, from) {
				if i != k || c != String_charAt(s, i) {
					t.Fatalf("%q from %d: unit %d is %#x, charAt %#x", s, from, i, c, String_charAt(s, i))
				}
				k++
			}
			if k != max(from, next) {
				t.Fatalf("%q from %d: stopped at %d of %d units", s, from, k, next)
			}
			if from == 20 && next > 40 {
				from = next - 3 // a long string: its end
			}
		}
	}
	const hex = "0123456789abcdef"
	for i := int32(0); i < 16; i++ {
		if asciiCharAt_(hex, i) != String_charAt(hex, i) {
			t.Fatal(i)
		}
	}
	for _, i := range []int32{-1, 16} {
		func() {
			defer func() {
				if _, ok := catch_(recover()).(*StringIndexOutOfBoundsException); !ok {
					t.Fatalf("asciiCharAt_(%d) does not throw StringIndexOutOfBoundsException", i)
				}
			}()
			asciiCharAt_(hex, i)
		}()
	}
}

// A type assertion of a nil interface is the dereference of a null cast (castfold -assert):
// NullPointerException; of a value of another type, ClassCastException.
func TestCatchAssertion(t *testing.T) {
	caught := func(f func()) (ex any) {
		defer func() { ex = catch_(recover()) }()
		f()
		return nil
	}
	var v any
	if ex := caught(func() { _ = v.(*Scope).depth }); !is_NullPointerException(ex) {
		t.Errorf("assertion of nil: %T, want NullPointerException", ex)
	}
	var s StaticScope
	if ex := caught(func() { _ = s.(*Scope).depth }); !is_NullPointerException(ex) {
		t.Errorf("assertion of a nil interface: %T, want NullPointerException", ex)
	}
	v = "x"
	if ex := caught(func() { _ = v.(*Scope).depth }); !is_[*ClassCastException](ex) {
		t.Errorf("assertion of another type: %T, want ClassCastException", ex)
	}
}

// TestOptEquals checks optEquals_, optEqualsNull_ and optHash_ (tools/postflip/opteq) against
// the dynamic protocol they replace, on null, empty and present Optionals.
func TestOptEquals(t *testing.T) {
	strs := []Optional[string]{{}, Optional_empty[string](), Optional_of("a"), Optional_of("b"), Optional_of("")}
	ints := []Optional[int32]{{}, Optional_empty[int32](), Optional_of[int32](0), Optional_of[int32](7)}
	try := func(f func() bool) string {
		return baseTry_(func() string { return strconv.FormatBool(f()) })
	}
	for _, a := range strs {
		for _, b := range strs {
			if g, w := try(func() bool { return optEquals_(a, b) }), try(func() bool { return Object_equals(a.box_(), b.box_()) }); g != w {
				t.Errorf("optEquals_(%v, %v) = %s, Object_equals %s", a, b, g, w)
			}
			if g, w := optEqualsNull_(a, b), Objects_equals(a.box_(), b.box_()); g != w {
				t.Errorf("optEqualsNull_(%v, %v) = %v, Objects_equals %v", a, b, g, w)
			}
		}
		if g, w := optHash_(a), Objects_hashCode(a.box_()); g != w {
			t.Errorf("optHash_(%v) = %d, Objects_hashCode %d", a, g, w)
		}
	}
	for _, a := range ints {
		for _, b := range ints {
			if g, w := try(func() bool { return optEquals_(a, b) }), try(func() bool { return Object_equals(a.box_(), b.box_()) }); g != w {
				t.Errorf("optEquals_(%v, %v) = %s, Object_equals %s", a, b, g, w)
			}
			if g, w := optEqualsNull_(a, b), Objects_equals(a.box_(), b.box_()); g != w {
				t.Errorf("optEqualsNull_(%v, %v) = %v, Objects_equals %v", a, b, g, w)
			}
		}
		if g, w := optHash_(a), Objects_hashCode(a.box_()); g != w {
			t.Errorf("optHash_(%v) = %d, Objects_hashCode %d", a, g, w)
		}
	}
}
