package closure

import (
	"encoding/hex"
	"fmt"
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

func baseCaseFormat_(n string) *CaseFormat {
	for _, c := range []*CaseFormat{CaseFormat_LOWER_HYPHEN, CaseFormat_LOWER_UNDERSCORE, CaseFormat_LOWER_CAMEL, CaseFormat_UPPER_CAMEL, CaseFormat_UPPER_UNDERSCORE} {
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

func TestBasePreconditions(t *testing.T) {
	msg := func(f func()) (out string) {
		defer func() {
			if r := recover(); r != nil {
				out = Object_toString(baseCatch_(r))
			}
		}()
		f()
		return "no exception"
	}
	checks := []struct {
		f    func()
		want string
	}{
		{func() { Preconditions_checkState(false) }, "java.lang.IllegalStateException"},
		{func() { Preconditions_checkState_boolean_Object(false, int32(5)) }, "java.lang.IllegalStateException: 5"},
		{func() { Preconditions_checkState_boolean_String_Object_int(false, "a %s b %s", nil, 7) }, "java.lang.IllegalStateException: a null b 7"},
		{func() { Preconditions_checkArgument_boolean_String_int_int(false, "%s > %s", 1, 2) }, "java.lang.IllegalArgumentException: 1 > 2"},
		{func() { Preconditions_checkArgument_boolean_String_Object(false, "x", "extra") }, "java.lang.IllegalArgumentException: x [extra]"},
		{func() { Preconditions_checkNotNull(nil) }, "java.lang.NullPointerException"},
		{func() { Preconditions_checkNotNull_Object_String_Object(nil, "no %s", "y") }, "java.lang.NullPointerException: no y"},
		{func() { Optional_base_absent().get() }, "java.lang.IllegalStateException: Optional.get() cannot be called on an absent value"},
		{func() { Optional_empty().get() }, "java.util.NoSuchElementException: No value present"},
		{func() { Joiner_on(",").join_ObjectArr([]any{"a", nil}) }, "java.lang.NullPointerException"},
		{func() {
			Throwables_throwIfUnchecked(new_IllegalStateException_String("boom").up_RuntimeException().up_Exception().up_Throwable())
		}, "java.lang.IllegalStateException: boom"},
		{func() { Throwables_throwIfUnchecked(new_IOException_String("io").up_Exception().up_Throwable()) }, "no exception"},
	}
	for i, c := range checks {
		if got := msg(c.f); got != c.want {
			t.Errorf("%d: got %q want %q", i, got, c.want)
		}
	}
	if Preconditions_checkNotNull("x") != "x" {
		t.Errorf("checkNotNull")
	}
}

func TestBaseMisc(t *testing.T) {
	j := Joiner_on(", ")
	if got := j.join(listOf_([]any{"a", int32(1), true, uint16('c')})); got != "a, 1, true, c" {
		t.Errorf("join: %q", got)
	}
	if got := j.join_Object_Object_ObjectArr("a", "b", []any{"c"}); got != "a, b, c" {
		t.Errorf("join: %q", got)
	}
	if got := Joiner_on_char('-').appendTo(new_StringBuilder_String("x:"), listOf_([]any{"a", "b"})).toString(); got != "x:a-b" {
		t.Errorf("appendTo: %q", got)
	}
	if got := objstr_(Splitter_on('.').split("a.b.c")); got != "[a, b, c]" {
		t.Errorf("split: %q", got)
	}
	o := Optional_base_of("x")
	if !o.equals(Optional_base_fromNullable("x")) || o.equals(Optional_base_absent()) || !Optional_base_absent().equals(Optional_base_fromNullable(nil)) {
		t.Errorf("Optional.equals")
	}
	if o.or("y") != "x" || Optional_base_absent().or("y") != "y" || Optional_base_absent().orNull() != nil || Optional_base_absent().or_Optional(o) != o {
		t.Errorf("Optional_base.or")
	}
	if got := o.transform(&Function_base_fn{func(v any) any { return v.(string) + "!" }}).get(); got != "x!" {
		t.Errorf("Optional_base.transform: %v", got)
	}
	u := Optional_of("x")
	if !u.equals(Optional_of("x")) || u.equals(Optional_empty()) || u.orElse("y") != "x" || Optional_empty().orElse("y") != "y" || !Optional_empty().isEmpty() {
		t.Errorf("java.util.Optional_base")
	}
	n := 0
	s := Suppliers_memoize(&Supplier_base_fn{func() any { n++; return "v" }})
	if s.get() != "v" || s.get() != "v" || n != 1 {
		t.Errorf("memoize")
	}
	if !Predicates_equalTo("a").apply("a") || Predicates_equalTo("a").test("b") || !Predicates_equalTo(nil).apply(nil) || !Predicates_alwaysTrue().apply(nil) {
		t.Errorf("Predicates")
	}
	if Strings_emptyToNull("") != nullstr_ || Strings_nullToEmpty(nullstr_) != "" || !Strings_isNullOrEmpty(nullstr_) || Strings_isNullOrEmpty("a") {
		t.Errorf("Strings")
	}
	if got := Chars_toArray(Chars_asList([]uint16{'a', 'b'})); len(got) != 2 || got[1] != 'b' {
		t.Errorf("Chars")
	}
	arr := new_AtomicIntegerArray(3)
	if arr.updateAndGet(1, &IntUnaryOperator_fn{func(v int32) int32 { return v | 4 }}) != 4 || arr.get(1) != 4 || arr.get(0) != 0 {
		t.Errorf("AtomicIntegerArray")
	}
	if CaseFormat_UPPER_UNDERSCORE.name() != "UPPER_UNDERSCORE" || CaseFormat_LOWER_CAMEL.ordinal() != 2 {
		t.Errorf("CaseFormat enum")
	}
	if StandardSystemProperty_USER_DIR.value() == nullstr_ || BigInteger_ZERO.signum() != 0 || BigInteger_ONE.toString() != "1" || Locale_ROOT == nil {
		t.Errorf("constants")
	}
	if got := String_toLowerCase("ÀB", Locale_ROOT); got != "àb" {
		t.Errorf("toLowerCase: %q", got)
	}
}
