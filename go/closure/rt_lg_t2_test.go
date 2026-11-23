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
