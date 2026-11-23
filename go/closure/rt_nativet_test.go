package closure

import "testing"

// TestNativeTypedViews: slicePtrViewT_ reads as asListT() of a typed list over the same items
// (also when the slice changes between reads); sliceOfT_ is tlistOfT_'s items.
func TestNativeTypedViews(t *testing.T) {
	for _, items := range [][]string{{}, {"a"}, {"b", "a", "c"}} {
		s := append([]string(nil), items...)
		v := slicePtrViewT_(timmStr_, &s)
		l := tlistFrom_(timmStr_, Collection(sliceView_(timmStr_, items)))
		w := l.asListT()
		if v.size() != w.size() || v.isEmpty() != w.isEmpty() || Object_toString(v) != Object_toString(w) || Object_hashCode(v) != Object_hashCode(w) || !Object_equals(v, w) {
			t.Errorf("%v: %v, want %v", items, Object_toString(v), Object_toString(w))
		}
		for i := int32(-1); i <= int32(len(items)); i++ {
			a := guavaThrows_(func() { v.get_t(i) })
			b := guavaThrows_(func() { w.get_t(i) })
			if a != b || a == "" && v.get_t(i) != w.get_t(i) {
				t.Errorf("%v get_t(%d): %q, want %q", items, i, a, b)
			}
		}
		var got, want []string
		for e := jeachOf_[string](v); e.next(); {
			got = append(got, e.v)
		}
		for e := jeachOf_[string](w); e.next(); {
			want = append(want, e.v)
		}
		for x := range elemsT_[string](v) {
			got = append(got, x)
		}
		for x := range elemsT_[string](w) {
			want = append(want, x)
		}
		if len(got) != len(want) {
			t.Errorf("%v: walked %v, want %v", items, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%v: walked %v, want %v", items, got, want)
			}
		}
		it := v.iterator_t()
		for range items {
			it.next_t()
		}
		wi := w.iterator_t()
		for range items {
			wi.next_t()
		}
		if it.hasNext() != wi.hasNext() || guavaThrows_(func() { it.next_t() }) != guavaThrows_(func() { wi.next_t() }) {
			t.Errorf("%v: the iterator does not end as the list's", items)
		}
		// the view reads the slice anew
		s = append(s, "z")
		if v.size() != int32(len(items)+1) || v.get_t(int32(len(items))) != "z" {
			t.Errorf("%v: the view does not see the slice's change", items)
		}
		if guavaThrows_(func() { any(v.iterator_t()).(interface{ remove() }).remove() }) == "" {
			t.Errorf("%v: remove before next did not throw", items)
		}
	}
	if slicePtrViewT_[string](timmStr_, nil) != nil {
		t.Error("slicePtrViewT_ of nil is not nil")
	}
	// sliceOfT_ / slicePtrOfT_ are tlistOfT_
	for _, l := range []*TList[string]{nil, tlistEmptyImm_(timmStr_), immutableListOf_(timmStr_, "a", "b")} {
		var x Collection
		if l != nil {
			x = Collection(l.asListT())
		}
		got, want := sliceOfT_(timmStr_, x), tlistOfT_(timmStr_, x)
		if (got == nil) != (want == nil) || want != nil && len(got) != len(want.items) {
			t.Errorf("sliceOfT_ %v, want %v", got, want)
		}
		if p := slicePtrOfT_(timmStr_, x); (p == nil) != (want == nil) {
			t.Errorf("slicePtrOfT_ %v", p)
		}
	}
	if guavaThrows_(func() { sliceOfT_(timmStr_, Collection(newTList_(timmStr_).asListT())) }) != guavaThrows_(func() { tlistOfT_(timmStr_, Collection(newTList_(timmStr_).asListT())) }) {
		t.Error("sliceOfT_ of a list that is no ImmutableList does not throw as tlistOfT_")
	}
}
