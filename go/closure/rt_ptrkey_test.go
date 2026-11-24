package closure

import "testing"

type ptrKeyInner_ struct{ a int64 }
type ptrKeyOuter_ struct {
	in ptrKeyInner_
	b  int64
}

// Objects without equals/hashCode are keys by identity. primHash_ hashes such a key by its
// address alone, so two keys of different types at one address (an object and its first
// field) meet in a slot and must still be told apart.
func TestPointerKeys(t *testing.T) {
	m := new_LinkedHashMap()
	var outers []*ptrKeyOuter_
	for i := 0; i < 2000; i++ {
		o := &ptrKeyOuter_{}
		outers = append(outers, o)
		if i%2 == 0 {
			m.put(o, int32(i))
		}
		if i%3 == 0 {
			m.put(&o.in, int32(-i))
		}
	}
	m.put(int32(7), "seven")
	m.put(true, "true")
	for i, o := range outers {
		if got, want := m.containsKey(o), i%2 == 0; got != want {
			t.Fatalf("outer %d: containsKey %v, want %v", i, got, want)
		}
		if i%2 == 0 && m.get(o) != any(int32(i)) {
			t.Fatalf("outer %d: get %v", i, m.get(o))
		}
		if got, want := m.containsKey(&o.in), i%3 == 0; got != want {
			t.Fatalf("inner %d: containsKey %v, want %v", i, got, want)
		}
		if i%3 == 0 && m.get(&o.in) != any(int32(-i)) {
			t.Fatalf("inner %d: get %v", i, m.get(&o.in))
		}
	}
	if m.get(int32(7)) != any("seven") || m.get(true) != any("true") || m.containsKey(int32(8)) || m.containsKey(false) {
		t.Fatal("boxed keys")
	}
	for i, o := range outers {
		if i%4 == 0 {
			m.remove(o)
		}
	}
	for i, o := range outers {
		if got, want := m.containsKey(o), i%2 == 0 && i%4 != 0; got != want {
			t.Fatalf("after remove, outer %d: containsKey %v, want %v", i, got, want)
		}
	}
	if primHash_(outers[0]) != primHash_(&outers[0].in) {
		t.Fatal("a pointer key's hash is its address's")
	}
}

type benchObj_ struct{ a, b int64 }

// The loop of SortedDependencies.getStrongDependenciesOf: every element of a long erased list
// is looked up in a small set of objects without equals/hashCode.
func BenchmarkErasedListContains(b *testing.B) {
	list := new_ArrayList_0()
	set := new_LinkedHashSet()
	for i := 0; i < 20000; i++ {
		o := &benchObj_{}
		list.add(o)
		if i%200 == 0 {
			set.add(o)
		}
	}
	b.ResetTimer()
	n := 0
	for i := 0; i < b.N; i++ {
		for el := range elems_(list) {
			input := junboxT_[*benchObj_](el)
			if set.contains(boxT_(input)) {
				n++
			}
		}
	}
	if n != 100*b.N {
		b.Fatal(n)
	}
}
