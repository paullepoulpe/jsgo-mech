package closure

import "testing"

// Piece `nbox` (rt_nbox.go): null is the zero value; unboxing null throws
// NullPointerException; box_ gives the box erased code knows, nboxOf_ takes it back.
func TestNbox(t *testing.T) {
	var z Nbox[bool]
	if !z.null_() || z.box_() != nil {
		t.Fatalf("zero value: null_ %v box_ %v", z.null_(), z.box_())
	}
	if !npe_(func() { z.get_() }) {
		t.Fatalf("get_ of null: no NullPointerException")
	}
	f := nbox_(false)
	if f.null_() || f.get_() != false || f.box_() != any(false) {
		t.Fatalf("false: null_ %v get_ %v box_ %v", f.null_(), f.get_(), f.box_())
	}
	n := nbox_(int32(0))
	if n.null_() || n.box_() != any(int32(0)) {
		t.Fatalf("0: null_ %v box_ %v", n.null_(), n.box_())
	}
	if b := nboxOf_[int32](any(int32(2020))); b.null_() || b.get_() != 2020 {
		t.Fatalf("nboxOf_ of a box: %v", b)
	}
	if b := nboxOf_[int32](nil); !b.null_() {
		t.Fatalf("nboxOf_ of nil: %v", b)
	}
	if b := nboxOf_[bool](nbox_(true).box_()); b.get_() != true {
		t.Fatalf("round trip: %v", b)
	}
}
