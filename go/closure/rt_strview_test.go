package closure

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// outcome_ is the result of f, or the Java exception it throws.
func outcome_(f func() any) (r any) {
	defer func() {
		if e := recover(); e != nil {
			r = "throws " + objstr_(catch_(e))
		}
	}()
	return f()
}

// A view gives what the String_ helpers give, for every position in and around the string.
func TestUtf16View(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	pieces := []string{"a", "b", "/", "*", "\\", "0", " ", "é", "ž", "中", "\U0001F600", "\U00010000",
		surrogateBytes_(0xD83D), surrogateBytes_(0xDE00), "\xff", "\x80", "\xe4\xb8"}
	subjects := []string{"", "a", "abc/def", nullstr_, strings.Repeat("ab/", 100), strings.Repeat("ab/", 100) + "é" + strings.Repeat("x", 70) + "\U0001F600y"}
	for n := 0; n < 400; n++ {
		var b strings.Builder
		ascii := rnd.Intn(3) == 0
		for k := rnd.Intn(12); k > 0; k-- {
			if ascii {
				b.WriteString(pieces[rnd.Intn(7)])
			} else {
				b.WriteString(pieces[rnd.Intn(len(pieces))])
			}
		}
		subjects = append(subjects, b.String())
	}
	needles := []string{"", "a", "/", "ab", "b/", "AB", "B/x", "é", "\U0001F600", surrogateBytes_(0xDE00), "x"}
	chars := []int32{'a', '/', 0, 'x', 0xE9, 0x4E2D, 0x1F600, 0xD83D, 0xDE00, -1, -191, 0x7F, 0x80}
	var kept utf16_
	for k, s := range subjects {
		v := utf16Of_(s)
		if k%3 == 1 {
			v = utf16Kept_(s)
			if int(v.n) != len(String_toCharArray(s)) || v.u != nil && len(v.u) != int(v.n) {
				t.Fatalf("%q: a kept view of %d units has %d", s, v.n, len(v.u))
			}
		}
		if k%3 == 0 {
			v = kept.of(s)
			if w := kept.of(s); w.s != s || len(w.u) != len(v.u) {
				t.Fatalf("%q: of gives the view of %q", s, w.s)
			}
		}
		n := String_length(s)
		check := func(what string, got, want func() any) {
			if g, w := outcome_(got), outcome_(want); g != w {
				t.Fatalf("%q: %s: view %v, String_ %v", s, what, g, w)
			}
		}
		check("length", func() any { return v.length() }, func() any { return String_length(s) })
		for i := int32(-2); i <= n+2; i++ {
			check(fmt.Sprint("charAt ", i), func() any { return v.charAt(i) }, func() any { return String_charAt(s, i) })
			check(fmt.Sprint("codePointAt ", i), func() any { return v.codePointAt(i) }, func() any { return String_codePointAt(s, i) })
			check(fmt.Sprint("substring_int ", i), func() any { return v.substring_int(i) }, func() any { return String_substring_int(s, i) })
			for j := int32(-1); j <= n+2; j++ {
				check(fmt.Sprint("substring ", i, j), func() any { return v.substring(i, j) }, func() any { return String_substring(s, i, j) })
			}
			for _, p := range needles {
				for _, n := range []int32{-1, 0, 1, 2, 3, 0x7fffffff} {
					for _, po := range []int32{-1, 0, 1} {
						check(fmt.Sprint("regionMatches ", p, i, po, n), func() any { return v.regionMatches(i%2 == 0, i, p, po, n) }, func() any { return String_regionMatches(s, i%2 == 0, i, p, po, n) })
					}
				}
				check(fmt.Sprint("indexOf_String_int ", p, i), func() any { return v.indexOf_String_int(p, i) }, func() any { return String_indexOf_String_int(s, p, i) })
				check(fmt.Sprint("startsWith_String_int ", p, i), func() any { return v.startsWith_String_int(p, i) }, func() any { return String_startsWith_String_int(s, p, i) })
			}
			for _, c := range chars {
				check(fmt.Sprint("lastIndexOf_int_int ", c, i), func() any { return v.lastIndexOf_int_int(c, i) }, func() any { return String_lastIndexOf_int_int(s, c, i) })
				check(fmt.Sprint("indexOf_int_int ", c, i), func() any { return v.indexOf_int_int(c, i) }, func() any { return String_indexOf_int_int(s, c, i) })
			}
		}
		for _, p := range needles {
			check("indexOf_String "+p, func() any { return v.indexOf_String(p) }, func() any { return String_indexOf_String(s, p) })
			check("lastIndexOf_String "+p, func() any { return v.lastIndexOf_String(p) }, func() any { return String_lastIndexOf_String(s, p) })
		}
		for _, c := range chars {
			check(fmt.Sprint("indexOf ", c), func() any { return v.indexOf(c) }, func() any { return String_indexOf(s, c) })
			check(fmt.Sprint("lastIndexOf ", c), func() any { return v.lastIndexOf(c) }, func() any { return String_lastIndexOf(s, c) })
		}
	}
}
