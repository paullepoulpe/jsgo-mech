package closure

import (
	"math/rand"
	"testing"
)

// TestRangeIteratorsMatchCursors walks typed lists, slices, maps and sets with each
// range-over-func iterator of rt_iter.go and with the cursor it replaces (post-flip step
// ranges), running the same body on both: one that appends, one that shortens, one that
// removes and re-adds map entries (compacting the table), and one that breaks early. Both
// walks must see the same elements in the same order.
func TestRangeIteratorsMatchCursors(t *testing.T) {
	kc := &tconv_[int32]{box: func(v int32) any { return v }, unbox: func(v any) int32 { return v.(int32) }}
	type listBody func(s *[]int32, x int32) bool // false: break
	bodies := map[string]listBody{
		"append": func(s *[]int32, x int32) bool {
			if x < 5 && len(*s) < 20 { // bounded: walking from the end revisits after an append
				*s = append(*s, x+100)
			}
			return true
		},
		"shorten": func(s *[]int32, x int32) bool {
			if x == 7 {
				*s = (*s)[1 : len(*s)-1]
			}
			return true
		},
		"break": func(s *[]int32, x int32) bool { return x != 4 },
	}
	fresh := func() []int32 {
		var s []int32
		for i := int32(0); i < 12; i++ {
			s = append(s, i)
		}
		return s
	}
	for name, body := range bodies {
		// TList.All against each, TList.AllRev against eachRev.
		for _, rev := range []bool{false, true} {
			var a, b []int32
			l1, l2 := newTList_(kc), newTList_(kc)
			l1.items, l2.items = fresh(), fresh()
			cur := l1.each()
			if rev {
				for e := l1.eachRev(); e.next(); {
					a = append(a, e.v)
					if !body(&l1.items, e.v) {
						break
					}
				}
			} else {
				for cur.next() {
					a = append(a, cur.v)
					if !body(&l1.items, cur.v) {
						break
					}
				}
			}
			seq := l2.All()
			if rev {
				seq = l2.AllRev()
			}
			for x := range seq {
				b = append(b, x)
				if !body(&l2.items, x) {
					break
				}
			}
			if !sameInts_(a, b) {
				t.Errorf("%s (rev %v): cursor %v, range %v", name, rev, a, b)
			}
		}
		// sliceAll_ against eachOf_, sliceAllRev_ against eachRevOf_.
		for _, rev := range []bool{false, true} {
			var a, b []int32
			s1, s2 := fresh(), fresh()
			if rev {
				for e := eachRevOf_(&s1); e.next(); {
					a = append(a, e.v)
					if !body(&s1, e.v) {
						break
					}
				}
				for x := range sliceAllRev_(&s2) {
					b = append(b, x)
					if !body(&s2, x) {
						break
					}
				}
			} else {
				for e := eachOf_(&s1); e.next(); {
					a = append(a, e.v)
					if !body(&s1, e.v) {
						break
					}
				}
				for x := range sliceAll_(&s2) {
					b = append(b, x)
					if !body(&s2, x) {
						break
					}
				}
			}
			if !sameInts_(a, b) {
				t.Errorf("slice %s (rev %v): cursor %v, range %v", name, rev, a, b)
			}
		}
	}
	// Maps and sets, with removals (some compacting the table) and additions in the body.
	for seed := int64(1); seed <= 20; seed++ {
		build := func() (*TMap[int32, int32], *rand.Rand) {
			rnd := rand.New(rand.NewSource(seed))
			m := newTMap_(kc, kc)
			for i := 0; i < 40; i++ {
				m.put(int32(rnd.Intn(60)), int32(i))
			}
			for i := 0; i < 10; i++ {
				m.remove(int32(rnd.Intn(60)))
			}
			return m, rnd
		}
		body := func(m *TMap[int32, int32], rnd *rand.Rand, k int32) {
			switch rnd.Intn(4) {
			case 0:
				m.remove(int32(rnd.Intn(60)))
			case 1:
				m.put(int32(60+rnd.Intn(10)), k)
			}
		}
		var a, b, c, d []int32
		m1, r1 := build()
		for e := m1.each(); e.next(); {
			a = append(a, e.k, e.v)
			body(m1, r1, e.k)
		}
		m2, r2 := build()
		for k, v := range m2.All() {
			b = append(b, k, v)
			body(m2, r2, k)
		}
		m3, r3 := build()
		for k := range m3.Keys() {
			c = append(c, k, m3.get(k))
			body(m3, r3, k)
		}
		m4, r4 := build()
		var vs []int32
		for e := m4.each(); e.next(); {
			vs = append(vs, e.v)
			body(m4, r4, e.v)
		}
		m5, r5 := build()
		for v := range m5.Values() {
			d = append(d, v)
			body(m5, r5, v)
		}
		if !sameInts_(a, b) {
			t.Errorf("seed %d: TMap each %v, All %v", seed, a, b)
		}
		if !sameInts_(a, c) {
			t.Errorf("seed %d: TMap each %v, Keys %v", seed, a, c)
		}
		if !sameInts_(vs, d) {
			t.Errorf("seed %d: TMap each %v, Values %v", seed, vs, d)
		}
		// Sets: All against each, First against the view's iterator.
		s1, s2 := newTSet_(kc), newTSet_(kc)
		rnd := rand.New(rand.NewSource(seed))
		for i := 0; i < 30; i++ {
			k := int32(rnd.Intn(40))
			s1.add(k)
			s2.add(k)
		}
		for i := 0; i < 8; i++ {
			k := int32(rnd.Intn(40))
			s1.remove(k)
			s2.remove(k)
		}
		var x, y []int32
		for e := s1.each(); e.next(); {
			x = append(x, e.v)
			if e.v%3 == 0 {
				s1.remove(e.v + 1)
			}
		}
		for k := range s2.All() {
			y = append(y, k)
			if k%3 == 0 {
				s2.remove(k + 1)
			}
		}
		if !sameInts_(x, y) {
			t.Errorf("seed %d: TSet each %v, All %v", seed, x, y)
		}
		if s2.size() > 0 && s2.First() != s2.asSet().iterator().next().(int32) {
			t.Errorf("seed %d: First disagrees with the view's iterator", seed)
		}
	}
	// The erased walks against foreach_, and first_ against iterator().next().
	list := new_ArrayList_0()
	set := new_LinkedHashSet()
	for i := int32(0); i < 10; i++ {
		list.add(i)
		set.add(i * 2)
	}
	for _, c := range []Iterable{list, set, immList_(list.toArray_0()), new_ArrayList_0()} {
		var a, b, e []any
		for it := foreach_(c); it.next(); {
			a = append(a, it.v)
		}
		for v := range elems_(c) {
			b = append(b, v)
		}
		for v := range elemsAs_[int32](c) {
			e = append(e, v)
		}
		if !sameSlice_(a, b) || !sameSlice_(a, e) {
			t.Errorf("%T: foreach_ %v, elems_ %v, elemsAs_ %v", c, a, b, e)
		}
		if len(a) > 0 && first_(c) != c.iterator().next() {
			t.Errorf("%T: first_ disagrees with iterator().next()", c)
		}
	}
	defer func() {
		if _, ok := recover().(*NoSuchElementException); !ok {
			t.Error("first_ of an empty list should throw NoSuchElementException")
		}
	}()
	first_(new_ArrayList_0())
}

func sameInts_(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestImmCursorMatchesIterator walks immutable lists and sets (empty, with duplicates folded
// by the set) with immCursorOf_ and with the collection's own iterator: same elements, then
// NoSuchElementException from next at the end; remove throws UnsupportedOperationException.
func TestImmCursorMatchesIterator(t *testing.T) {
	throws := func(f func()) (thrown bool) {
		defer func() { thrown = recover() != nil }()
		f()
		return false
	}
	for _, c := range []*ImmutableCollection{
		immList_(nil), immList_([]any{"a", "b", "a"}), immSet_([]any{"a", "b", "a", "c"}),
	} {
		it, cur := c.iterator(), immCursorOf_(c)
		for it.hasNext() {
			if !cur.hasNext() || cur.next() != it.next() {
				t.Fatalf("cursor differs from iterator over %v", c.elems_())
			}
		}
		if cur.hasNext() || !throws(func() { cur.next() }) || !throws(func() { cur.remove() }) {
			t.Fatalf("cursor end differs over %v", c.elems_())
		}
	}
	if !throws(func() { immElems_(nil) }) || !throws(func() { immCursorOf_(nil) }) {
		t.Fatal("a null immutable collection does not throw")
	}
}

// TestBoxedKVMatchesEntrySet: mapKV_ of a typed map's view (ImmutableMap.copyOf) gives the keys
// and values that walking the view's entry set gives, after removals and re-additions.
func TestBoxedKVMatchesEntrySet(t *testing.T) {
	kc := &tconv_[int32]{box: func(v int32) any { return v }, unbox: func(v any) int32 { return v.(int32) }}
	for seed := int64(1); seed <= 20; seed++ {
		rnd := rand.New(rand.NewSource(seed))
		m := newTMap_(kc, kc)
		for i := 0; i < 60; i++ {
			if rnd.Intn(3) == 0 {
				m.remove(int32(rnd.Intn(40)))
			} else {
				m.put(int32(rnd.Intn(40)), int32(i))
			}
		}
		var ks, vs []any
		for it := m.asMap().entrySet().iterator(); it.hasNext(); {
			e := it.next().(Map_Entry)
			ks, vs = append(ks, e.getKey()), append(vs, e.getValue())
		}
		gk, gv := mapKV_(m.asMap())
		if !sameSlice_(gk, ks) || !sameSlice_(gv, vs) || (gk == nil) != (ks == nil) {
			t.Fatalf("seed %d: mapKV_ %v=%v, entry set %v=%v", seed, gk, gv, ks, vs)
		}
	}
}
