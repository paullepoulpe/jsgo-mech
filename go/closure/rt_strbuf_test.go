package closure

import "testing"

// TestStrBuilderLikeStringBuilder checks that strBuilder_ builds what StringBuilder builds for
// strings that are ASCII, valid UTF-8, lone surrogates (WTF-8), a surrogate pair split over
// two appends, and stray bytes.
func TestStrBuilderLikeStringBuilder(t *testing.T) {
	hi, lo := surrogateBytes_(0xD83D), surrogateBytes_(0xDE00)
	cases := [][]any{
		{"abc", "def"},
		{"a", "é", "€", "😀"},
		{"x", hi, lo, "y"},
		{hi, "z", lo},
		{"a", uint16(0xD83D), uint16(0xDE00), "b"},
		{"\xe2\x82", "\xac"},
		{"ok", "\xff", "é"},
		{nullstr_, int32(-12), int64(1) << 40, true, 1.5, uint16('q'), uint16(0x00e9)},
		{"é", hi},
	}
	for _, parts := range cases {
		var a strBuilder_
		b := new_StringBuilder()
		for _, p := range parts {
			switch v := p.(type) {
			case string:
				a.append(v)
				b.append(v)
			case uint16:
				a.append_char(v)
				b.append_char(v)
			case int32:
				a.append_int(v)
				b.append_int(v)
			case int64:
				a.append_long(v)
				b.append_long(v)
			case bool:
				a.append_boolean(v)
				b.append_boolean(v)
			case float64:
				a.append_double(v)
				b.append_double(v)
			}
			if a.length() != b.length() || a.isEmpty() != b.isEmpty() {
				t.Errorf("%q: length %d, StringBuilder %d", parts, a.length(), b.length())
			}
		}
		if a.toString() != b.toString() {
			t.Errorf("%q: %q, StringBuilder %q", parts, a.toString(), b.toString())
		}
	}
}

// TestStrBuilderReadsAndChanges checks length, charAt, substring, setLength, insert, delete
// and setCharAt of strBuilder_ against StringBuilder after appends of ASCII, multi-byte text,
// a supplementary character and a lone surrogate.
func TestStrBuilderReadsAndChanges(t *testing.T) {
	hi := surrogateBytes_(0xD83D)
	starts := [][]string{{"abc", "def"}, {"ab", "é", "c"}, {"x", "😀"}, {"a", hi, "b"}, {}}
	type op func(a *strBuilder_, b *StringBuilder) (string, string)
	ops := map[string]op{
		"last": func(a *strBuilder_, b *StringBuilder) (string, string) {
			if b.length() == 0 {
				return "", ""
			}
			return string(rune(a.charAt(a.length() - 1))), string(rune(b.charAt(b.length() - 1)))
		},
		"first": func(a *strBuilder_, b *StringBuilder) (string, string) {
			if b.length() == 0 {
				return "", ""
			}
			return string(rune(a.charAt(0))), string(rune(b.charAt(0)))
		},
		"sub": func(a *strBuilder_, b *StringBuilder) (string, string) {
			n := b.length()
			return a.substring(n/2, n), b.substring(n/2, n)
		},
		"set0": func(a *strBuilder_, b *StringBuilder) (string, string) {
			a.setLength(0)
			b.setLength(0)
			return "", ""
		},
		"grow": func(a *strBuilder_, b *StringBuilder) (string, string) {
			a.setLength(a.length() + 2)
			b.setLength(b.length() + 2)
			return "", ""
		},
		"shrink": func(a *strBuilder_, b *StringBuilder) (string, string) {
			if b.length() > 1 {
				a.setLength(a.length() - 1)
				b.setLength(b.length() - 1)
			}
			return "", ""
		},
		"insert": func(a *strBuilder_, b *StringBuilder) (string, string) {
			a.insert(a.length()/2, '\n')
			b.insert(b.length()/2, '\n')
			return "", ""
		},
		"delete": func(a *strBuilder_, b *StringBuilder) (string, string) {
			a.delete(0, 1)
			b.delete(0, 1)
			return "", ""
		},
		"index": func(a *strBuilder_, b *StringBuilder) (string, string) {
			return istr_(int64(a.indexOf("b"))), istr_(int64(b.indexOf("b")))
		},
	}
	names := []string{"last", "first", "sub", "set0", "grow", "shrink", "insert", "delete", "index"}
	for _, start := range starts {
		for _, n1 := range names {
			for _, n2 := range names {
				a, b := &strBuilder_{}, new_StringBuilder()
				for _, s := range start {
					a.append(s)
					b.append(s)
				}
				for _, n := range []string{n1, n2} {
					if x, y := ops[n](a, b); x != y {
						t.Errorf("%q %s %s: %s gives %q, StringBuilder %q", start, n1, n2, n, x, y)
					}
					a.append("é")
					b.append("é")
					if a.length() != b.length() || a.toString() != b.toString() {
						t.Errorf("%q %s %s: after %s %q (%d), StringBuilder %q (%d)", start, n1, n2, n, a.toString(), a.length(), b.toString(), b.length())
					}
				}
			}
		}
	}
}
