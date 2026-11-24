package closure

import "testing"

// Piece `objarg` (rt_lgobj.go): each twin gives what the erased method gives with the boxed
// argument, on the view of a typed map or set (the typed lookup) and on any other map or set (a
// typed map's key set, a wrapped erased collection: the erased method), for a key
// that is there, one that is not and null. The changing methods run on two equal collections,
// one through the twins and one erased; both must end equal.
func TestLgObjargMaps(t *testing.T) {
	keys := []string{"a", "z", nullstr_}
	mk := func(native bool) MapT[string, string] {
		if native {
			m := newTMap_(tc_string, tc_string)
			m.put("a", "A")
			m.put(nullstr_, "N")
			m.put("b", "B")
			return m.asMapT()
		}
		e := new_LinkedHashMap()
		e.put("a", "A")
		e.put(nil, "N")
		e.put("b", "B")
		return mapAs_(tc_string, tc_string, Map(e))
	}
	for _, native := range []bool{true, false} {
		tm, em := mk(native), mk(native)
		if _, ok := tm.(tmapOf_); ok != native {
			t.Fatal("the typed lookup is the view's only", native)
		}
		for _, k := range keys {
			if Map_getK_(tm, k) != em.get_t(strobj_(k)) || Map_containsKeyK_(tm, k) != em.containsKey(strobj_(k)) {
				t.Fatal("get / containsKey", native, k)
			}
		}
		for _, k := range append(keys, "a", nullstr_) {
			if Map_removeK_(tm, k) != em.remove_t(strobj_(k)) {
				t.Fatal("remove", native, k)
			}
		}
		if collToString_(Map_un_(tm).entrySet()) != collToString_(Map_un_(em).entrySet()) || tm.size() != 1 {
			t.Fatal("after remove", native, collToString_(Map_un_(tm).entrySet()))
		}
	}
}

func TestLgObjargSets(t *testing.T) {
	elems := []string{"a", "z", nullstr_}
	mk := func(kind int) SetT[string] {
		switch kind {
		case 0:
			s := newTSet_(tc_string)
			s.add("a")
			s.add(nullstr_)
			s.add("b")
			return s.asSetT()
		case 1:
			m := newTMap_(tc_string, tc_string)
			m.put("a", "A")
			m.put(nullstr_, "N")
			m.put("b", "B")
			return m.keySetT()
		default:
			e := new_LinkedHashSet()
			e.add("a")
			e.add(nil)
			e.add("b")
			return setAs_(tc_string, Set(e))
		}
	}
	for kind := 0; kind < 3; kind++ {
		ts, es := mk(kind), mk(kind)
		if _, ok := ts.(tsetOf_); ok != (kind == 0) {
			t.Fatal("the typed lookup is the typed set's view's only", kind)
		}
		for _, x := range elems {
			if Set_containsE_(ts, x) != es.contains(strobj_(x)) {
				t.Fatal("contains", kind, x)
			}
		}
		for _, x := range append(elems, "a", nullstr_) {
			if Set_removeE_(ts, x) != es.remove_Object(strobj_(x)) {
				t.Fatal("remove", kind, x)
			}
		}
		if collToString_(Set_un_(ts)) != collToString_(Set_un_(es)) {
			t.Fatal("after remove", kind, collToString_(Set_un_(ts)))
		}
	}
}
