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
