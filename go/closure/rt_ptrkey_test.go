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

type plainObj_ struct{ a int64 }

type selfObj_ struct{ self_ any }

func (o *selfObj_) self() any { return o.self_ }

// find remembers the last pointer type that has no equals/hashCode, and boxT_ the last type
// that is its own box. Keys and values of other types in between, in tables of both kinds,
// must get what they got before.
func TestTypeMemos(t *testing.T) {
	set := new_LinkedHashSet() // uses equals/hashCode where a key has them
	ident := identitySet_()    // never does
	var plains []*plainObj_
	var keys []*tkey_
	for i := 0; i < 300; i++ {
		p, k := &plainObj_{}, &tkey_{n: int32(i)}
		plains, keys = append(plains, p), append(keys, k)
		if i%2 == 0 {
			set.add(p)
			set.add(k)
			ident.add(p)
			ident.add(k)
		}
	}
	for round := 0; round < 3; round++ {
		for i := range plains {
			in := i%2 == 0
			if set.contains(plains[i]) != in || ident.contains(plains[i]) != in {
				t.Fatalf("plain %d", i)
			}
			// an equal key that is another object: found by equals, not by identity
			eq := &tkey_{n: int32(i)}
			if set.contains(keys[i]) != in || set.contains(eq) != in {
				t.Fatalf("key %d in the hashing set", i)
			}
			if ident.contains(keys[i]) != in || ident.contains(eq) {
				t.Fatalf("key %d in the identity set", i)
			}
			if round == 1 {
				// the identity table first: it must not make find take tkey_ for a type
				// without equals/hashCode
				if ident.contains(eq) || set.contains(eq) != in {
					t.Fatalf("key %d after the identity set", i)
				}
			}
		}
	}

	p := &plainObj_{}
	real := &plainObj_{}
	s := &selfObj_{self_: real}
	var nilp *plainObj_
	var nils []int32
	some := []int32{1}
	for round := 0; round < 3; round++ {
		if boxT_(p) != any(p) || boxT_(nilp) != nil {
			t.Fatal("a pointer is its own box, a nil pointer is null")
		}
		if boxT_(s) != any(real) {
			t.Fatal("an object with self() boxes as its most derived object")
		}
		if boxT_(nils) != nil || boxT_(some) == nil {
			t.Fatal("a nil slice is null, another slice is not")
		}
		if boxT_(int32(3)) != any(int32(3)) || boxT_("x") != any("x") {
			t.Fatal("values")
		}
		if boxT_[any](p) != any(p) || boxT_[any](nil) != nil {
			t.Fatal("an Object")
		}
	}
}

// elems_ over a list reads the list's current length at each step, as the cursor does: what
// the body appends is visited, what it removes is not.
func TestElemsOfAGrowingList(t *testing.T) {
	list := new_ArrayList_0()
	for i := 0; i < 5; i++ {
		list.add(int32(i))
	}
	var got []int32
	for el := range elems_(list) {
		v := el.(int32)
		got = append(got, v)
		if v == 1 {
			list.add(int32(10))
		}
		if v == 3 {
			list.remove(list.size() - 1)
			list.remove(list.size() - 1)
		}
	}
	want := []int32{0, 1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	n := 0
	for range elems_(list) {
		n++
		if n == 2 {
			break
		}
	}
	if n != 2 {
		t.Fatal("break")
	}
}
