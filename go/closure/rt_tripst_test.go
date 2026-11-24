package closure

import "testing"

// TestTypedRoundTripsT: tsetAddAllOfT_, tsetAddSetT_, tsetAddListT_ are s.asSetT().addAll(..) and
// s.asCollT().addAll(..): the same elements in the same order, the same result, the same exception.
func TestTypedRoundTripsT(t *testing.T) {
	mk := func(kind string, xs ...string) *TSet[string] {
		var s *TSet[string]
		switch kind {
		case "imm":
			return tsetMake_(timmStr_, xs)
		case "sorted":
			s = newTSortedSet_(timmStr_)
		default:
			s = newTSet_(timmStr_)
		}
		for _, x := range xs {
			s.Add(x)
		}
		return s
	}
	keys := func(s *TSet[string]) []string {
		out := []string{}
		for k := range s.All() {
			out = append(out, k)
		}
		return out
	}
	same := func(what string, got, want []string) {
		t.Helper()
		if len(got) != len(want) {
			t.Errorf("%s: %v, want %v", what, got, want)
			return
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%s: %v, want %v", what, got, want)
			}
		}
	}
	try := func(f func() bool) (r bool, threw string) {
		threw = guavaThrows_(func() { r = f() })
		return
	}
	for _, kind := range []string{"", "sorted", "imm"} {
		for _, from := range [][]string{{}, {"a"}, {"z", "a", "q"}} {
			for _, into := range [][]string{{}, {"a"}, {"c", "q"}} {
				for _, view := range []string{"asSetT", "asCollT"} {
					call := func(s *TSet[string], c Collection) bool {
						if view == "asSetT" {
							return s.asSetT().addAll(c)
						}
						return s.asCollT().addAll(c)
					}
					src := mk("", from...)
					l := tlistFrom_(timmStr_, Collection(src.asSet()))
					w0, w1, w2 := mk(kind, into...), mk(kind, into...), mk(kind, into...)
					r0, e0 := try(func() bool { return call(w0, Collection(src.asSet())) })
					r1, e1 := try(func() bool { return call(w1, Collection(src.asSetT())) })
					r2, e2 := try(func() bool { return call(w2, Collection(l.asList())) })
					s0, s1, s2 := mk(kind, into...), mk(kind, into...), mk(kind, into...)
					q0, f0 := try(func() bool { return tsetAddAllOfT_(s0, Collection(src.asSet())) })
					q1, f1 := try(func() bool { return tsetAddSetT_(s1, src) })
					q2, f2 := try(func() bool { return tsetAddListT_(s2, l) })
					what := kind + " " + view
					if q0 != r0 || q1 != r1 || q2 != r2 || f0 != e0 || f1 != e1 || f2 != e2 {
						t.Errorf("%s %v into %v: results %v %v %v (%q %q %q), want %v %v %v (%q %q %q)", what, from, into, q0, q1, q2, f0, f1, f2, r0, r1, r2, e0, e1, e2)
					}
					same(what+" tsetAddAllOfT_", keys(s0), keys(w0))
					same(what+" tsetAddSetT_", keys(s1), keys(w1))
					same(what+" tsetAddListT_", keys(s2), keys(w2))
				}
			}
		}
	}
	// an element of another type: the same exception, after the same elements
	for _, kind := range []string{"", "sorted", "imm"} {
		w, s := mk(kind, "b"), mk(kind, "b")
		bad := Collection(immList_([]any{"a", int32(1), "c"}))
		r0, e0 := try(func() bool { return w.asSetT().addAll(bad) })
		r1, e1 := try(func() bool { return tsetAddAllOfT_(s, bad) })
		if r0 != r1 || e0 != e1 {
			t.Errorf("%s, a bad element: %v %q, want %v %q", kind, r1, e1, r0, e0)
		}
		same(kind+", a bad element", keys(s), keys(w))
	}
	// null sets and sources
	for _, kind := range []string{"", "imm"} {
		_, e0 := try(func() bool { return mk(kind, "a").asSetT().addAll(Collection((*TSet[string])(nil).asSet())) })
		_, e1 := try(func() bool { return tsetAddAllOfT_(mk(kind, "a"), Collection((*TSet[string])(nil).asSet())) })
		_, e2 := try(func() bool { return tsetAddSetT_(mk(kind, "a"), nil) })
		_, e3 := try(func() bool { return tsetAddListT_(mk(kind, "a"), nil) })
		if e1 != e0 || e2 != e0 || e3 != e0 {
			t.Errorf("%s, a null source: %q %q %q, want %q", kind, e1, e2, e3, e0)
		}
	}
	_, e0 := try(func() bool { return (*TSet[string])(nil).asSetT().addAll(Collection(mk("").asSet())) })
	_, e1 := try(func() bool { return tsetAddAllOfT_[string](nil, Collection(mk("").asSet())) })
	if e0 != e1 {
		t.Errorf("a null set: %q, want %q", e1, e0)
	}
}
