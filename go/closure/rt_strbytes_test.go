package closure

import (
	"fmt"
	"testing"
)

// The byte-offset helpers do at a character boundary what the String_ helpers do at the unit
// index it stands for, and throw the same exception out of range.
func TestStrBytesHelpers(t *testing.T) {
	strs := []string{"", "a", "a/b.c", "é/x", "x\U0001F600/y", "\xed\xa0\xbdz/", "ab/cd/ef", "ÿ.a.ÿ"}
	try := func(f func() string) (r string) {
		defer func() {
			if e := recover(); e != nil {
				r = fmt.Sprint("throws ", objstr_(catch_(e)))
			}
		}()
		return f()
	}
	for _, s := range strs {
		// the byte offsets of boundaries, a negative and two past the end
		var offs []int32
		for b := 0; b <= len(s); b++ {
			if b == len(s) || s[b] < 0x80 || s[b] >= 0xC0 {
				offs = append(offs, int32(b))
			}
		}
		offs = append(offs, -1, int32(len(s))+1, int32(len(s))+3)
		for _, a := range offs {
			ua := unitIndex_(s, a)
			for _, b := range offs {
				ub := unitIndex_(s, b)
				if g, w := try(func() string { return cutBytes_(s, a, b) }), try(func() string { return String_substring(s, ua, ub) }); g != w {
					t.Errorf("cutBytes_(%q, %d, %d) = %s, want %s", s, a, b, g, w)
				}
			}
			if g, w := try(func() string { return cutBytesFrom_(s, a) }), try(func() string { return String_substring_int(s, ua) }); g != w {
				t.Errorf("cutBytesFrom_(%q, %d) = %s, want %s", s, a, g, w)
			}
			gc := try(func() string { return fmt.Sprint(byteAt_(s, a) < 0x80, byteAt_(s, a)&0x7F) })
			wc := try(func() string { c := String_charAt(s, ua); return fmt.Sprint(c < 0x80, c&0x7F) })
			if gc != wc && !(gc[:4] == "fals" && wc[:4] == "fals") {
				t.Errorf("byteAt_(%q, %d) = %s, want %s", s, a, gc, wc)
			}
			for _, c := range []byte{'/', '.', 'a', 'z'} {
				if g, w := unitIndex_(s, indexByteFrom_(s, c, a)), String_indexOf_int_int(s, int32(c), ua); g != w {
					t.Errorf("indexByteFrom_(%q, %q, %d) = %d, want %d", s, c, a, g, w)
				}
				if g, w := unitIndex_(s, lastIndexByteFrom_(s, c, a)), String_lastIndexOf_int_int(s, int32(c), ua); g != w {
					t.Errorf("lastIndexByteFrom_(%q, %q, %d) = %d, want %d", s, c, a, g, w)
				}
			}
			for _, p := range []string{"/", "b.", "", "/y"} {
				if p != "" {
					if g, w := unitIndex_(s, indexFrom_(s, p, a)), String_indexOf_String_int(s, p, ua); g != w {
						t.Errorf("indexFrom_(%q, %q, %d) = %d, want %d", s, p, a, g, w)
					}
					if g, w := unitIndex_(s, lastIndexFrom_(s, p, a)), String_lastIndexOf_String_int(s, p, ua); g != w {
						t.Errorf("lastIndexFrom_(%q, %q, %d) = %d, want %d", s, p, a, g, w)
					}
				}
				if g, w := hasPrefixAt_(s, p, a), String_startsWith_String_int(s, p, ua); g != w {
					t.Errorf("hasPrefixAt_(%q, %q, %d) = %v, want %v", s, p, a, g, w)
				}
			}
		}
	}
}
