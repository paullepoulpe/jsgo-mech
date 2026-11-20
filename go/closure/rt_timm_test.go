package closure

import "testing"

var timmStr_ = &tconv_[string]{box: func(v string) any { return strobj_(v) }, unbox: func(v any) string { return str_(v) }, null: nullstr_}

// The typed immutable collections behave as the erased ones (rt_guava.go), and crossing to
// erased code and back keeps their identity.
func TestTypedImmutables(t *testing.T) {
	l := immutableListOf_(timmStr_, "a", "b")
	guavaStr_(t, l.asList(), "[a, b]")
	if l.asImm() != l.asImm() || tlistImm_(timmStr_, Collection(l.asImm())) != l {
		t.Error("list twin identity")
	}
	if immutableListCopyOf_(timmStr_, l.asList()) != l {
		t.Error("copyOf of an immutable list is the list")
	}
	guavaEq_(t, l.asImm(), ImmutableList_of_Object_Object("a", "b"))
	guavaThrew_(t, "UnsupportedOperationException", func() { l.asImm().add("c") })
	guavaThrew_(t, "UnsupportedOperationException", func() { l.add("c") })
	guavaThrew_(t, "NullPointerException", func() { immutableListOf_(timmStr_, "a", nullstr_) })
	if immutableListOf_(timmStr_) != tlistEmptyImm_(timmStr_) || tlistEmptyImm_(timmStr_).asImm() != ImmutableList_of() {
		t.Error("empty list identity")
	}
	if tlistImm_(timmStr_, Collection(ImmutableList_of())) != tlistEmptyImm_(timmStr_) {
		t.Error("erased empty list")
	}
	e := ImmutableList_of_Object("x")
	if tlistImm_(timmStr_, Collection(e)).asImm() != e {
		t.Error("an erased list is its typed copy's twin")
	}

	b := immutableListBuilder_(timmStr_)
	b.add("x").addAll(Iterable(l.asList()))
	built := b.build()
	b.add("y")
	guavaStr_(t, built.asList(), "[x, a, b]")
	guavaThrew_(t, "NullPointerException", func() { b.add(nullstr_) })

	s := immutableSetOf_(timmStr_, "b", "a", "b")
	guavaStr_(t, s.asSet(), "[b, a]")
	if !s.Has("a") || s.Has("c") || s.Len() != 2 {
		t.Error("set contents")
	}
	if immutableSetCopyOf_(timmStr_, Iterable(s.asImm())) != s || tsetFrom_(timmStr_, Collection(s.asImm()), true) != s {
		t.Error("set twin identity")
	}
	guavaEq_(t, s.asImm(), ImmutableSet_of_Object_Object("a", "b"))
	if guavaThrows_(func() { ImmutableSet_copyOf(Collection(s.asImm())) }) != "" || ImmutableSet_copyOf(Collection(s.asImm())) != s.asImm() {
		t.Error("ImmutableSet.copyOf of the twin")
	}
	if immutableSetOf_(timmStr_).asImm() != ImmutableSet_of() {
		t.Error("empty set")
	}
	sb := immutableSetBuilder_(timmStr_)
	sb.add("q").add("q").add("r")
	guavaStr_(t, sb.build().asSet(), "[q, r]")

	m := immutableMapOf_(timmStr_, timmStr_, []string{"k", "j"}, []string{"v", "w"})
	guavaStr_(t, m.asMap(), "{k=v, j=w}")
	if m.Get("j") != "w" || m.Get("z") != nullstr_ {
		t.Error("map get")
	}
	if immutableMapCopyOf_(timmStr_, timmStr_, m.asMap()) != m {
		t.Error("map twin identity")
	}
	guavaThrew_(t, "IllegalArgumentException", func() {
		immutableMapOf_(timmStr_, timmStr_, []string{"k", "k"}, []string{"v", "w"})
	})
	guavaThrew_(t, "NullPointerException", func() {
		immutableMapOf_(timmStr_, timmStr_, []string{"k"}, []string{nullstr_})
	})
	mb := immutableMapBuilder_(timmStr_, timmStr_)
	mb.put("a", "1").put("b", "2").put("a", "3")
	guavaThrew_(t, "IllegalArgumentException", func() { mb.build() })
	guavaStr_(t, mb.buildKeepingLast().asMap(), "{b=2, a=3}")
	guavaStr_(t, ImmutableMap_copyOf(m.asMap()), "{k=v, j=w}")
	if ImmutableMap_copyOf(m.asMap()) != m.asImm() {
		t.Error("ImmutableMap.copyOf of the twin")
	}
}

func TestTypedImmutableEqHash(t *testing.T) {
	a := immutableListOf_(timmStr_, "x", "y")
	b := immutableListOf_(timmStr_, "x", "y")
	if !tlistEq_(a, b, true) || tlistHash_(a, true) != a.asImm().hashCode() || tlistEq_(a, nil, true) || !tlistEq_[string](nil, nil, true) {
		t.Error("list equals/hashCode")
	}
	guavaThrew_(t, "NullPointerException", func() { tlistEq_(nil, b, false) })
	s := immutableSetOf_(timmStr_, "p", "q")
	s2 := immutableSetOf_(timmStr_, "q", "p")
	if !tsetEq_(s, s2, false) || tsetHash_(s, false) != s.asImm().hashCode() {
		t.Error("set equals/hashCode")
	}
	m := immutableMapOf_(timmStr_, timmStr_, []string{"k"}, []string{"v"})
	m2 := immutableMapOf_(timmStr_, timmStr_, []string{"k"}, []string{"w"})
	if tmapEq_(m, m2, true) || tmapHash_(m, true) != m.asImm().hashCode() {
		t.Error("map equals/hashCode")
	}
}

// The helpers of the post-flip step trips give what the pair of conversions they replace gives.
func TestTypedRoundTrips(t *testing.T) {
	same := func(what string, got, want []string) {
		t.Helper()
		if (got == nil) != (want == nil) || len(got) != len(want) {
			t.Errorf("%s: %v, want %v", what, got, want)
			return
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%s: %v, want %v", what, got, want)
			}
		}
	}
	// tlistSlice_(l) is sliceImm_(c, l.asImm())
	for _, l := range []*TList[string]{nil, immutableListOf_(timmStr_), immutableListOf_(timmStr_, "a"), immutableListOf_(timmStr_, "b", "a", "b")} {
		got := tlistSlice_(timmStr_, l)
		same("tlistSlice_", got, sliceImm_(timmStr_, Collection(l.asImm())))
		same("tlistSlice_, again", tlistSlice_(timmStr_, l), sliceImm_(timmStr_, Collection(l.asImm())))
		if l != nil && len(l.items) > 0 && &got[0] != &l.items[0] {
			t.Error("tlistSlice_ copies")
		}
	}
	// (a list of another conversion object, or whose twin has another typed copy: the conversions themselves)
	other := &tconv_[string]{box: timmStr_.box, unbox: timmStr_.unbox, null: nullstr_}
	lo, lw := immutableListOf_(other, "p", "q"), immutableListOf_(other, "p", "q")
	for range 2 { // the helpers on lo, the conversions on lw: the same results at every step
		same("tlistSlice_, other conversion", tlistSlice_(timmStr_, lo), sliceImm_(timmStr_, Collection(lw.asImm())))
		a, b := tlistImmCopy_(timmStr_, lo), immutableListCopyOf_(timmStr_, lw.asList())
		same("tlistImmCopy_, other conversion", a.items, b.items)
		c, d := tlistImmCopy_(other, lo), immutableListCopyOf_(other, lw.asList())
		same("tlistImmCopy_, own conversion", c.items, d.items)
		if (a == lo) != (b == lw) || (c == lo) != (d == lw) {
			t.Errorf("tlistImmCopy_ is the list: %v %v, the conversions: %v %v", a == lo, c == lo, b == lw, d == lw)
		}
	}
	// tlistImmCopy_(c, l) is immutableListCopyOf_(c, l.asList())
	for _, l := range []*TList[string]{immutableListOf_(timmStr_), immutableListOf_(timmStr_, "a"), immutableListOf_(timmStr_, "b", "a", "b")} {
		if got, want := tlistImmCopy_(timmStr_, l), immutableListCopyOf_(timmStr_, l.asList()); got != want || got != l {
			t.Error("tlistImmCopy_ of an immutable list is not the list")
		}
	}
	for _, xs := range [][]string{{}, {"a"}, {"b", "a", "b"}} {
		l := newTList_(timmStr_)
		for _, x := range xs {
			l.add(x)
		}
		got, want := tlistImmCopy_(timmStr_, l), immutableListCopyOf_(timmStr_, l.asList())
		same("tlistImmCopy_", got.items, want.items)
		if !got.imm || got == l || (len(xs) == 0) != (got == tlistEmptyImm_(timmStr_)) {
			t.Error("tlistImmCopy_ of a mutable list")
		}
		if lv := newTList_(timmStr_); len(xs) == 0 {
			if tlistImmCopy_(timmStr_, lv); lv.view != nil {
				t.Error("tlistImmCopy_ made the list's view")
			}
		}
	}
	mutNull := newTList_(timmStr_)
	mutNull.add(nullstr_)
	guavaThrew_(t, "NullPointerException", func() { tlistImmCopy_(timmStr_, mutNull) })
	guavaThrew_(t, "errorString", func() { tlistImmCopy_(timmStr_, nil) })
	// sliceAddSlice_, sliceAddList_ are sliceAddAll_ of a slice's view and of a list's
	for _, xs := range [][]string{{}, {"a"}, {"b", "a", "b"}} {
		a, b, c := []string{"z"}, []string{"z"}, []string{"z"}
		l := immutableListCopyOfArr_(timmStr_, xs)
		ra, rb, rc := sliceAddAll_(timmStr_, &a, sliceView_(timmStr_, xs)), sliceAddSlice_(&b, xs), sliceAddList_(&c, l)
		rl := sliceAddAll_(timmStr_, &a, iup_Collection(l.asImm()))
		sliceAddSlice_(&b, xs)
		same("sliceAddSlice_", b, a)
		same("sliceAddList_", append(c, xs...), a)
		if ra != rb || ra != rc || ra != rl {
			t.Errorf("sliceAdd results %v %v %v %v", ra, rb, rc, rl)
		}
	}
	self := []string{"a", "b"}
	sliceAddSlice_(&self, self)
	same("sliceAddSlice_ of itself", self, []string{"a", "b", "a", "b"})
	var none []string
	guavaThrew_(t, "errorString", func() { sliceAddAll_(timmStr_, &none, sliceView_(timmStr_, []string(nil))) })
	guavaThrew_(t, "NullPointerException", func() { sliceAddSlice_(&none, nil) })
	guavaThrew_(t, "errorString", func() { sliceAddList_(&none, nil) })
	// sliceSkip_(pieces, n) is immCopyOf_(c, Iterables.skip(list of pieces, n)); splitStrs_ the typed pieces
	for _, sp := range []*Splitter{Splitter_on('.'), Splitter_on('.').omitEmptyStrings(), Splitter_on('.').limit(2), Splitter_on_String(", ").trimResults()} {
		for _, in := range []string{"", "a", "a.b.c", "..a..b.", " a , b.c, ,d "} {
			want := sliceImm_(timmStr_, Collection(sp.splitToList_s(in)))
			same("splitStrs_ "+in, sp.splitStrs_(in), want)
			for n := int32(0); n < 4; n++ {
				same("sliceSkip_ "+in, sliceSkip_(sp.splitStrs_(in), n), immCopyOf_(timmStr_, Iterables_skip(sp.split_s(in), n)))
			}
		}
	}
	guavaThrew_(t, "NullPointerException", func() { Splitter_on('.').splitStrs_(nullstr_) })
	guavaThrew_(t, "NullPointerException", func() { Splitter_on('.').splitToList_s(nullstr_) })
	guavaThrew_(t, "IllegalArgumentException", func() { sliceSkip_([]string{"a"}, -1) })
	guavaThrew_(t, "IllegalArgumentException", func() { Iterables_skip(Splitter_on('.').split_s("a"), -1) })
	// (the splitter itself, and the pieces' slice made once at its final size)
	if n := testing.AllocsPerRun(100, func() { Splitter_on('.').splitStrs_("a.b.c.d") }); n > 2 {
		t.Errorf("splitStrs_ allocates %v times for four pieces", n)
	}
	// immCopyArr_(c, a) is sliceImm_(c, ImmutableList.copyOf(a))
	box := func(s string) any { return timmStr_.box(s) }
	for _, a := range [][]string{nil, {}, {"a"}, {"b", "a", "b"}} {
		got := immCopyArr_(timmStr_, a)
		same("immCopyArr_", got, sliceImm_(timmStr_, Collection(ImmutableList_copyOf_ObjectArr(arrmap_(a, box)))))
		if len(a) > 0 && &got[0] == &a[0] {
			t.Error("immCopyArr_ does not copy")
		}
	}
	guavaThrew_(t, "NullPointerException", func() { immCopyArr_(timmStr_, []string{"a", nullstr_}) })
	guavaThrew_(t, "NullPointerException", func() {
		sliceImm_(timmStr_, Collection(ImmutableList_copyOf_ObjectArr(arrmap_([]string{"a", nullstr_}, box))))
	})
	// tsetCopy_(c, s) is tsetFrom_(c, s.asSet(), false)
	mk := func(imm bool, xs ...string) *TSet[string] {
		if imm {
			return tsetMake_(timmStr_, xs)
		}
		s := newTSet_(timmStr_)
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
	holed := mk(false, "x", "c", "a", "y", "b")
	holed.Delete("x")
	holed.Delete("y")
	for _, s := range []*TSet[string]{mk(false), mk(false, "b", "a"), holed, mk(true), mk(true, "c", "a", "b")} {
		a, b := tsetCopy_(timmStr_, s), tsetFrom_(timmStr_, Collection(s.asSet()), false)
		same("tsetCopy_", keys(a), keys(b))
		if a.imm || b.imm || a == s {
			t.Error("tsetCopy_: not a new mutable set")
		}
	}
	// (a nil set is a Go nil dereference in both, which catch_ makes a NullPointerException)
	guavaThrew_(t, "errorString", func() { tsetCopy_(timmStr_, nil) })
	guavaThrew_(t, "errorString", func() { tsetFrom_(timmStr_, Collection((*TSet[string])(nil).asSet()), false) })
	// tsetAddAllOf_, tsetAddSet_, tsetAddList_ are s.asSet().addAll(..)
	srcs := []*TSet[string]{mk(false), mk(false, "a", "z"), holed, mk(true), mk(true, "q", "a")}
	for _, from := range srcs {
		for _, into := range [][]string{{}, {"a"}, {"z", "q", "c"}} {
			want := mk(false, into...)
			wr := want.asSet().addAll(Collection(from.asSet()))
			s1, s2, s3 := mk(false, into...), mk(false, into...), mk(false, into...)
			r1, r2 := tsetAddSet_(s1, from), tsetAddAllOf_(s2, Collection(from.asSet()))
			l := immutableListCopyOf_(timmStr_, from.asSet())
			r3 := tsetAddList_(s3, l)
			same("tsetAddSet_", keys(s1), keys(want))
			same("tsetAddAllOf_", keys(s2), keys(want))
			same("tsetAddList_", keys(s3), keys(want))
			if r1 != wr || r2 != wr || r3 != wr {
				t.Errorf("addAll results %v %v %v, want %v", r1, r2, r3, wr)
			}
			if s1.x != nil && s1.x.sview != nil || s2.x != nil && s2.x.sview != nil {
				t.Error("the helper made the set's view")
			}
		}
	}
	imm := mk(true, "a")
	for _, f := range []func(){
		func() { imm.asSet().addAll(Collection(mk(false).asSet())) },
		func() { tsetAddSet_(imm, mk(false)) },
		func() { tsetAddAllOf_(imm, Collection(mk(false).asSet())) },
		func() { tsetAddList_(imm, immutableListOf_(timmStr_)) },
		func() { tsetAddSet_(mk(true), mk(false)) },
	} {
		guavaThrew_(t, "UnsupportedOperationException", f)
	}
	for _, f := range []func(){
		func() { mk(false).asSet().addAll(Collection((*TSet[string])(nil).asSet())) },
		func() { tsetAddSet_(mk(false), nil) },
		func() { tsetAddList_(mk(false), nil) },
		func() { tsetAddAllOf_(mk(false), nil) },
		func() { tsetAddAllOf_[string](nil, Collection(mk(false).asSet())) },
	} {
		guavaThrew_(t, "errorString", f)
	}
}
