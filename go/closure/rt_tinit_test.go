package closure

import (
	"reflect"
	"testing"
)

// TestInPlaceCollections: initTList_, initTListN_, initTMap_, initTMapN_, initTSet_ and
// initTSetN_ (post-flip inlinefld) make, in a value that held anything before, the collection
// the constructor of the same name makes: the same fields and the same capacity of the list's
// slice; and a collection made in place behaves as one made by the constructor.
func TestInPlaceCollections(t *testing.T) {
	type holder struct {
		l TList[string]
		m TMap[string, int32]
		s TSet[string]
	}
	tc_int32 := &tconv_[int32]{box: func(v int32) any { return v }, unbox: func(v any) int32 { return v.(int32) }}
	var h holder
	h.l.items = []string{"x"}
	h.l.imm = true
	h.m.live = 3
	h.s.imm = true
	for _, n := range []int32{0, 1, 7} {
		initTListN_(&h.l, tc_string, n)
		if want := newTListN_(tc_string, n); !reflect.DeepEqual(h.l, *want) || cap(h.l.items) != cap(want.items) {
			t.Errorf("initTListN_(%d) = %+v, want %+v", n, h.l, *want)
		}
		initTMapN_(&h.m, tc_string, tc_int32, n)
		if want := newTMapN_(tc_string, tc_int32, n); !reflect.DeepEqual(h.m, *want) {
			t.Errorf("initTMapN_(%d) = %+v, want %+v", n, h.m, *want)
		}
		initTSetN_(&h.s, tc_string, n)
		if want := newTSetN_(tc_string, n); !reflect.DeepEqual(h.s, *want) {
			t.Errorf("initTSetN_(%d) = %+v, want %+v", n, h.s, *want)
		}
	}
	initTList_(&h.l, tc_string)
	initTMap_(&h.m, tc_string, tc_int32)
	initTSet_(&h.s, tc_string)
	if !reflect.DeepEqual(h.l, *newTList_(tc_string)) || !reflect.DeepEqual(h.m, *newTMap_(tc_string, tc_int32)) || !reflect.DeepEqual(h.s, *newTSet_(tc_string)) {
		t.Errorf("in place: %+v, %+v, %+v", h.l, h.m, h.s)
	}
	for i := 0; i < 20; i++ {
		h.l.add(istr_(int64(i)))
		h.m.Put(istr_(int64(i)), int32(i))
		h.s.Add(istr_(int64(i % 7)))
	}
	l := &h.l
	if l.size() != 20 || h.m.Len() != 20 || h.s.Len() != 7 || l.get(3) != "3" || h.m.Get("19") != 19 || !h.s.Has("6") {
		t.Errorf("in place: %d %d %d", l.size(), h.m.Len(), h.s.Len())
	}
	if l.asList() != l.asList() || List(l.asList()) == List(newTList_(tc_string).asList()) {
		t.Errorf("in place: the list's view has not the list's identity")
	}
}
