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
