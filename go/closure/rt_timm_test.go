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
	if !s.contains("a") || s.contains("c") || s.size() != 2 {
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
	if m.get("j") != "w" || m.get("z") != nullstr_ {
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
