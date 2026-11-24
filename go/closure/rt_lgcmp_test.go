package closure

import "testing"

// cmpTest_ is a class as piece `cmp` writes one: compareTo_t at its type, and the bridge.
type cmpTest_ struct {
	n      int32
	typed  *int
	erased *int
}

func (this *cmpTest_) compareTo_t(o *cmpTest_) int32 {
	*this.typed++
	return this.n - o.n
}

func (this *cmpTest_) compareTo(p0 any) int32 {
	*this.erased++
	return this.compareTo_t(cast_[*cmpTest_](p0))
}

// Piece `cmp` (rt_lgcmp.go): the typed natural orders call compareTo_t, not the bridge, and give
// what the erased natural order gives.
func TestLgCmp(t *testing.T) {
	var typed, erased int
	mk := func(n int32) *cmpTest_ { return &cmpTest_{n, &typed, &erased} }
	a, b := mk(1), mk(2)
	if r, ok := cmpTypedT_(a, b); !ok || r != -1 || typed != 1 || erased != 0 {
		t.Fatalf("cmpTypedT_: %d %v typed %d erased %d", r, ok, typed, erased)
	}
	if _, ok := cmpTypedT_[*cmpTest_](nil, b); ok {
		t.Fatalf("cmpTypedT_ of a null receiver must leave it to compareNatural_")
	}
	// at another type (an interface, any) the class's compareTo_t is not ComparableT[T]
	if _, ok := cmpTypedT_[any](a, b); ok {
		t.Fatalf("cmpTypedT_[any] must not take the typed method")
	}
	if r := compareNatural_(a, b); r != -1 || erased != 1 {
		t.Fatalf("compareNatural_: %d erased %d", r, erased)
	}
	typed, erased = 0, 0
	tc := &jtconv_[*cmpTest_]{box: func(v *cmpTest_) any { return v }, unbox: func(v any) *cmpTest_ { return v.(*cmpTest_) }}
	s := newTSortedSet_(tc)
	for _, n := range []int32{3, 1, 2, 1} {
		s.addSorted(mk(n))
	}
	var got []int32
	for i, k := range s.keys {
		if !s.dead(i) {
			got = append(got, k.n)
		}
	}
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 || typed == 0 || erased != 0 {
		t.Fatalf("typed sorted set: %v typed %d erased %d", got, typed, erased)
	}
}
