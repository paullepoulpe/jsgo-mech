//go:build !j2go_nofntypes

package closure

import "testing"

// Piece `cmp`: the typed sorts' natural order (naturalT_), Comparator.naturalOrder and
// Comparator.comparing call compareTo_t.
func TestLgCmpFn(t *testing.T) {
	var typed, erased int
	mk := func(n int32) *cmpTest_ { return &cmpTest_{n, &typed, &erased} }
	a, b := mk(5), mk(2)
	if r := naturalT_(a, b); r != 3 || typed != 1 || erased != 0 {
		t.Fatalf("naturalT_: %d typed %d erased %d", r, typed, erased)
	}
	if r := Comparator_naturalOrder[*cmpTest_]()(b, a); r != -3 || typed != 2 || erased != 0 {
		t.Fatalf("Comparator_naturalOrder: %d typed %d erased %d", r, typed, erased)
	}
	type wrap struct{ c *cmpTest_ }
	key := Comparator_comparing[wrap, *cmpTest_](func(w wrap) *cmpTest_ { return w.c })
	if r := key(wrap{a}, wrap{b}); r != 3 || typed != 3 || erased != 0 {
		t.Fatalf("Comparator_comparing: %d typed %d erased %d", r, typed, erased)
	}
	// a String key keeps its own comparison
	if r := naturalT_("a", "b"); r != -1 {
		t.Fatalf("naturalT_ of Strings: %d", r)
	}
}
