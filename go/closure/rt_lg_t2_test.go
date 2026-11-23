package closure

import "testing"

// Kind `enumset` (rt_lg_t2.go): the typed makers give the elements in the order and with the
// membership of the erased EnumSet functions.
func TestLgT2EnumSet(t *testing.T) {
	var es []*Enum
	var boxed []any
	for i, n := range []string{"RED", "GREEN", "BLUE"} {
		e := &Enum{name_: n, ordinal_: int32(i)}
		e.self_ = e
		es = append(es, e)
		boxed = append(boxed, e)
	}
	cl := &Class{name_: "t2test.Hue"}
	registerEnum_(cl.name_, boxed)
	tc := &tconv_[*Enum]{box: func(v *Enum) any { return v }, unbox: func(v any) *Enum { return v.(*Enum) }}
	str := func(s *TSet[*Enum]) string { return collToString_(s.asSet()) }
	if a, b := str(tenumSetAllOf_(tc, cl)), EnumSet_allOf(cl).toString(); a != b || a != "[RED, GREEN, BLUE]" {
		t.Fatal(a, b)
	}
	n := tenumSetNoneOf_(tc, cl)
	n.add(es[2])
	n.add(es[0])
	n.add(es[2])
	if str(n) != "[RED, BLUE]" || n.size() != 2 || !n.asSetT().contains(es[0]) || n.asSetT().contains(es[1]) {
		t.Fatal(str(n))
	}
	if a, b := str(tenumSetOfArr_(tc, es[2], []*Enum{es[1], es[2]})), EnumSet_of_Enum_EnumArr(es[2], []*Enum{es[1], es[2]}).toString(); a != b || a != "[GREEN, BLUE]" {
		t.Fatal(a, b)
	}
	if a := str(tenumSetOf_(tc, es[1])); a != "[GREEN]" {
		t.Fatal(a)
	}
	src := new_LinkedHashSet()
	src.add(es[2])
	src.add(es[1])
	c := tenumSetCopyOf_(tc, Collection(src))
	if str(c) != EnumSet_copyOf(Collection(src)).toString() || !setEquals_(c.asSet(), EnumSet_copyOf(Collection(src))) {
		t.Fatal(str(c))
	}
}

// Kind `deque`: the typed deque is native at QueueT / DequeT; an erased deque or queue goes
// through a wrapper that unboxes, and erased code gets the object back.
func TestLgT2Deque(t *testing.T) {
	d := newTDeque_(tc_string)
	dt := d.asDequeT()
	if (*TDeque[string])(nil).asDequeT() != nil || (*TDeque[string])(nil).asQueueT() != nil {
		t.Fatal("null as a Deque is not nil")
	}
	dt.push_t("a")
	dt.push_t("b")
	if Deque_un_(dt) != d.asDeque() || lgBox_(dt) != any(d.asDeque()) || dequeAs_(tc_string, d.asDeque()) != dt || queueAs_(tc_string, Queue(d.asDeque())) != d.asQueueT() {
		t.Fatal("the typed deque as a Deque is not its view")
	}
	if dt.peek_t() != "b" || dt.pop_t() != "b" || d.asQueueT().poll_t() != "a" || dt.poll_t() != tc_string.null || dt.peek_t() != tc_string.null {
		t.Fatal(d.size())
	}
	ad := new_ArrayDeque()
	w := dequeAs_(tc_string, Deque(ad))
	w.push_t("x")
	w.add_t("y")
	if w.peek_t() != "x" || Deque_un_(w) != Deque(ad) || lgBox_(w) != any(ad) || w.size() != 2 || w.remove_0_t() != "x" || w.pop_t() != "y" || !w.isEmpty() {
		t.Fatal("the wrapper of an erased deque")
	}
	pq := new_PriorityQueue(nil)
	q := queueAs_(tc_string, Queue(pq))
	q.add_t("b")
	q.add_t("a")
	if q.peek_t() != "a" || q.poll_t() != "a" || q.poll_t() != "b" || q.poll_t() != tc_string.null || Queue_un_(q) != Queue(pq) {
		t.Fatal("the wrapper of an erased queue")
	}
}
