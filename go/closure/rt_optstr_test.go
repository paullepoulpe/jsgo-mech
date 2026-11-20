package closure

import (
	"fmt"
	"strconv"
	"testing"
)

// TestOptstr checks the string forms of Optional<String> (rt_optstr.go, tools/postflip/optstr)
// against the methods of Optional[string] they replace, on empty and present Optionals (the
// step proves no Optional<String> is null), with the exception and message thrown on failure.
func TestOptstr(t *testing.T) {
	try := func(f func() string) (out string) {
		defer func() {
			if r := recover(); r != nil {
				ex := baseCatch_(r)
				out = fmt.Sprintf("EX:%T", ex)
				if s, ok := ex.(interface{ toString() string }); ok {
					out += ":" + s.toString()
				}
			}
		}()
		return f()
	}
	type pair struct {
		o Optional[string]
		s string
	}
	var vals []pair
	for _, x := range []string{nullstr_, "", "a", "b"} {
		vals = append(vals, pair{Optional_ofNullable(x), x})
	}
	// the constructors
	for _, x := range []string{nullstr_, "", "a"} {
		if g, w := try(func() string { return optstrOf_(x) }), try(func() string { return optstr_(Optional_of(x)) }); g != w {
			t.Errorf("optstrOf_(%q) = %s, Optional.of %s", x, g, w)
		}
		if g, w := x, optstr_(Optional_ofNullable(x)); g != w {
			t.Errorf("ofNullable(%q): %q, want %q", x, g, w)
		}
	}
	if optstr_(Optional_empty[string]()) != nullstr_ {
		t.Errorf("empty is not nullstr_")
	}
	if g := try(func() string { return optstr_(Optional[string]{}) }); g == "" || g[:3] != "EX:" {
		t.Errorf("optstr_ of a null Optional = %s, want NullPointerException", g)
	}
	notEmpty := func(s string) bool { return s != "" }
	for _, a := range vals {
		o, s := a.o, a.s
		if g, w := s != nullstr_, o.isPresent(); g != w {
			t.Errorf("isPresent(%q) = %v, want %v", s, g, w)
		}
		if g, w := s == nullstr_, o.isEmpty(); g != w {
			t.Errorf("isEmpty(%q) = %v, want %v", s, g, w)
		}
		if g, w := try(func() string { return optstrGet_(s) }), try(func() string { return o.get() }); g != w {
			t.Errorf("optstrGet_(%q) = %s, get %s", s, g, w)
		}
		if g, w := try(func() string { return optstrGet_(s) }), try(func() string { return o.orElseThrow() }); g != w {
			t.Errorf("optstrGet_(%q) = %s, orElseThrow %s", s, g, w)
		}
		for _, y := range []string{nullstr_, "", "z"} {
			if g, w := optstrOr_(s, y), o.orElse(y); g != w {
				t.Errorf("optstrOr_(%q, %q) = %q, orElse %q", s, y, g, w)
			}
		}
		if g, w := optstrFilter_(s, notEmpty), optstr_(o.filterFn_(notEmpty)); g != w {
			t.Errorf("optstrFilter_(%q) = %q, filterFn_ %q", s, g, w)
		}
		if g, w := optstrHash_(s), optHash_(o); g != w {
			t.Errorf("optstrHash_(%q) = %d, optHash_ %d", s, g, w)
		}
		if g, w := objstr_(optstrBox_(s)), objstr_(o.box_()); g != w {
			t.Errorf("optstrBox_(%q) prints %q, box_ %q", s, g, w)
		}
		if g, w := Objects_hashCode(optstrBox_(s)), Objects_hashCode(o.box_()); g != w {
			t.Errorf("optstrBox_(%q) hashes %d, box_ %d", s, g, w)
		}
		if g := optstrUnbox_(o.box_()); g != s {
			t.Errorf("optstrUnbox_(box(%q)) = %q", s, g)
		}
		for _, b := range vals {
			if g, w := strconv.FormatBool(s == b.s), try(func() string { return strconv.FormatBool(optEquals_(o, b.o)) }); g != w {
				t.Errorf("%q == %q is %s, optEquals_ %s", s, b.s, g, w)
			}
			if g, w := s == b.s, optEqualsNull_(o, b.o); g != w {
				t.Errorf("%q == %q is %v, optEqualsNull_ %v", s, b.s, g, w)
			}
			if g, w := Object_equals(optstrBox_(s), optstrBox_(b.s)), Object_equals(o.box_(), b.o.box_()); g != w {
				t.Errorf("boxes of %q, %q equal %v, Optionals %v", s, b.s, g, w)
			}
		}
	}
}
