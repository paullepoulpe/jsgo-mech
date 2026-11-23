package closure

import "testing"

// TestNestedTypedCollections: a typed collection held in a typed container is boxed as its
// view and unboxed to itself (one view per collection: its identity is kept), null stays
// null, and erased code that stores through the container's view stores the same object.
func TestNestedTypedCollections(t *testing.T) {
	lc := tlistConv_[string]()
	l := newTList_(tc_string)
	l.add("a")
	if b := lc.box(l); b != any(l.asList()) || lc.unbox(b) != l {
		t.Error("a typed list should box as its view and unbox to itself")
	}
	if lc.box(nil) != nil || lc.unbox(nil) != nil || !lc.isNull(nil) || lc.isNull(l) {
		t.Error("a null typed list should box as null")
	}
	m := newTMap_(tc_string, lc)
	m.Put("k", l)
	if got := m.asMap().get(strobj_("k")); got != any(l.asList()) {
		t.Error("the erased view of the map should give the list's view")
	}
	// Map.computeIfAbsent through the view, with a function that makes a typed list's view.
	made := newTList_(tc_string)
	r := Map_computeIfAbsent_f(m.asMap(), strobj_("n"), func(any) any { return any(made.asList()) })
	if tlistOf_[string](r) != made || m.Get("n") != made {
		t.Error("computeIfAbsent through the view should store the typed list")
	}
	if tlistOf_[string](Map_computeIfAbsent_f(m.asMap(), strobj_("k"), func(any) any { t.Error("present"); return nil })) != l {
		t.Error("computeIfAbsent should answer the list that is there")
	}
	sc := tsetConv_[string]()
	s := newTSet_(tc_string)
	if sc.unbox(sc.box(s)) != s || tsetOf_[string](any(s.asSet())) != s || sc.box(nil) != nil {
		t.Error("a typed set should box as its view and unbox to itself")
	}
	mc := tmapConv_[string, string]()
	mm := newTMap_(tc_string, tc_string)
	if mc.unbox(mc.box(mm)) != mm || tmapOf_[string, string](any(mm.asMap())) != mm || mc.box(nil) != nil {
		t.Error("a typed map should box as its view and unbox to itself")
	}
}

// TestTypedComputeIfAbsent: ComputeIfAbsent with a typed function answers the value that is
// there, stores and answers the function's result, and stores nothing for null.
func TestTypedComputeIfAbsent(t *testing.T) {
	m := newTMap_(tc_string, tc_string)
	calls := 0
	f := func(k string) string { calls++; return k + "!" }
	if m.ComputeIfAbsent("a", f) != "a!" || m.ComputeIfAbsent("a", f) != "a!" || calls != 1 || m.Len() != 1 {
		t.Error("ComputeIfAbsent should compute once and store")
	}
	if got := m.ComputeIfAbsent("b", func(string) string { return nullstr_ }); got != nullstr_ || m.Has("b") {
		t.Error("ComputeIfAbsent should store nothing for null")
	}
	m.Put("c", nullstr_)
	if m.ComputeIfAbsent("c", f) != "c!" || m.Get("c") != "c!" {
		t.Error("ComputeIfAbsent should replace a null value")
	}
}

// TestDequeView: a deque that is a typed list is read from head to tail through tdequeView_,
// whichever end of the list is its head; addAll through the view of a list in the deque's
// own order adds at the tail.
func TestDequeView(t *testing.T) {
	// A stack: pushed at its head, which is the end of the list.
	st := newTList_(tc_string)
	st.push_("a")
	st.push_("b")
	st.push_("c")
	if got := String_join_s(".", tdequeView_(st, true)); got != "c.b.a" {
		t.Errorf("head to tail of a stack: %q", got)
	}
	// A queue: added at its tail, the end of the list; its head is the start.
	q := newTList_(tc_string)
	q.push_("x")
	tdequeView_(q, false).addAll(Collection(tdequeView_(st, true)))
	if got := String_join_s(".", tdequeView_(q, false)); got != "x.c.b.a" {
		t.Errorf("addAll at the tail: %q", got)
	}
	if q.popFirst_() != "x" || q.popFirst_() != "c" {
		t.Error("the head of the queue is the start of the list")
	}
	var null *TList[string]
	if tdequeView_(null, true) != nil || tdequeView_(null, false) != nil {
		t.Error("the view of a null deque should be null")
	}
}
