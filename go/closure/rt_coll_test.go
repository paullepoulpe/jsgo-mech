package closure

import (
	"math/rand"
	"testing"
)

// A key with its own equals/hashCode; many keys share a hash code.
type tkey_ struct{ n int32 }

func (k *tkey_) equals(o any) bool {
	x, ok := o.(*tkey_)
	return ok && x.n == k.n
}
func (k *tkey_) hashCode() int32 { return k.n % 7 }

// refMap_ is the model: an insertion-ordered list of pairs searched linearly.
type refMap_ struct{ keys, vals []any }

func (m *refMap_) find(k any) int {
	for i, x := range m.keys {
		if objequals_(k, x) {
			return i
		}
	}
	return -1
}

func (m *refMap_) put(k, v any) any {
	if i := m.find(k); i >= 0 {
		old := m.vals[i]
		m.vals[i] = v
		return old
	}
	m.keys, m.vals = append(m.keys, k), append(m.vals, v)
	return nil
}

func (m *refMap_) removeAt(i int) {
	m.keys = append(m.keys[:i:i], m.keys[i+1:]...)
	m.vals = append(m.vals[:i:i], m.vals[i+1:]...)
}

func sameSlice_(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !objequals_(a[i], b[i]) {
			return false
		}
	}
	return true
}

// TestHashMapAgainstModel drives a LinkedHashMap and the model with the same random
// operations (string, boxed int, identity and equals/hashCode keys; removal directly, through
// the views and through iterators; entry objects) and compares contents and order throughout.
func TestHashMapAgainstModel(t *testing.T) {
	for seed := int64(1); seed <= 60; seed++ {
		rnd := rand.New(rand.NewSource(seed))
		span := []int{3, 6, 12, 40, 200}[seed%5]
		idents := make([]any, span)
		for i := range idents {
			idents[i] = new_LinkedHashMap() // compared by identity
		}
		key := func() (any, string, bool) {
			n := rnd.Intn(span)
			switch rnd.Intn(5) {
			case 0:
				return int32(n), "", false
			case 1:
				return &tkey_{int32(n)}, "", false
			case 2:
				return idents[n], "", false
			case 3:
				if n == 0 {
					return nil, nullstr_, true
				}
			}
			s := "k" + istr_(int64(n))
			return s, s, true
		}
		m, ref := new_LinkedHashMap(), &refMap_{}
		var held []Map_Entry // entry objects handed out earlier
		check := func(what string) {
			t.Helper()
			if int(m.size()) != len(ref.keys) || !sameSlice_(m.keySet().toArray_0(), ref.keys) || !sameSlice_(m.values().toArray_0(), ref.vals) {
				t.Fatalf("seed %d after %s: got %v, want %v=%v", seed, what, m.toString(), ref.keys, ref.vals)
			}
			var ks []any
			for it := m.keySet().iterator(); it.hasNext(); {
				ks = append(ks, it.next())
			}
			if !sameSlice_(ks, ref.keys) {
				t.Fatalf("seed %d after %s: iteration %v, want %v", seed, what, ks, ref.keys)
			}
			for _, e := range held {
				// An entry of a mapping that still exists shows the map's value.
				if i := ref.find(e.getKey()); i >= 0 && e.(*hmapEntry_).t != nil && !objequals_(e.getValue(), ref.vals[i]) {
					t.Fatalf("seed %d after %s: entry %v, map has %v", seed, what, e, ref.vals[i])
				}
			}
		}
		for step := 0; step < 600; step++ {
			k, s, isStr := key()
			v := any(int32(step))
			switch op := rnd.Intn(20); {
			case op < 7:
				var got any
				if isStr && rnd.Intn(2) == 0 {
					got = Map_put_s(m, s, v)
				} else {
					got = m.put(k, v)
				}
				if want := ref.put(k, v); !objequals_(got, want) {
					t.Fatalf("seed %d: put returned %v, want %v", seed, got, want)
				}
			case op < 11:
				var got any
				if isStr && rnd.Intn(2) == 0 {
					got = Map_remove_s(m, s)
				} else {
					got = m.remove(k)
				}
				var want any
				if i := ref.find(k); i >= 0 {
					want = ref.vals[i]
					ref.removeAt(i)
				}
				if !objequals_(got, want) {
					t.Fatalf("seed %d: remove returned %v, want %v", seed, got, want)
				}
			case op < 15:
				got, has := m.get(k), m.containsKey(k)
				if isStr {
					got, has = Map_get_s(m, s), Map_containsKey_s(m, s)
				}
				i := ref.find(k)
				if has != (i >= 0) || i >= 0 && !objequals_(got, ref.vals[i]) {
					t.Fatalf("seed %d: get(%v) = %v, %v", seed, k, got, has)
				}
			case op == 15:
				// Remove some entries through an iterator of one of the views.
				var it Iterator
				mode := rnd.Intn(3)
				switch mode {
				case 0:
					it = m.keySet().iterator()
				case 1:
					it = m.values().iterator()
				default:
					it = m.entrySet().iterator()
				}
				for i := 0; it.hasNext(); {
					x := it.next()
					want := ref.keys[i]
					if mode == 1 {
						want = ref.vals[i]
					} else if mode == 2 {
						x = x.(Map_Entry).getKey()
					}
					if !objequals_(x, want) {
						t.Fatalf("seed %d: iterator yields %v, want %v", seed, x, want)
					}
					if rnd.Intn(3) == 0 {
						Iterator_remove(it)
						ref.removeAt(i)
					} else {
						i++
					}
				}
			case op == 16:
				// Entry objects: one per mapping, and setValue writes through.
				for it := m.entrySet().iterator(); it.hasNext(); {
					e := it.next().(Map_Entry)
					if rnd.Intn(4) == 0 {
						held = append(held, e)
					}
					if rnd.Intn(4) == 0 {
						e.setValue(int32(-step))
						ref.vals[ref.find(e.getKey())] = int32(-step)
					}
				}
				for it, i := m.entrySet().iterator(), 0; it.hasNext(); i++ {
					e := it.next().(Map_Entry)
					for _, h := range held {
						if h.(*hmapEntry_).t != nil && objequals_(h.getKey(), e.getKey()) && h != e {
							t.Fatalf("seed %d: two entry objects for key %v", seed, e.getKey())
						}
					}
				}
			case op == 17:
				if len(held) > 0 {
					e := held[rnd.Intn(len(held))]
					if i := ref.find(e.getKey()); i >= 0 && e.(*hmapEntry_).t != nil {
						e.setValue(v)
						ref.vals[i] = v
					}
				}
			case op == 18:
				if m.keySet().remove_Object(k) != (ref.find(k) >= 0) {
					t.Fatalf("seed %d: keySet().remove(%v)", seed, k)
				}
				if i := ref.find(k); i >= 0 {
					ref.removeAt(i)
				}
			default:
				if rnd.Intn(30) == 0 {
					m.clear()
					ref.keys, ref.vals = nil, nil
				}
			}
			check("step " + istr_(int64(step)))
		}
	}
}

// TestHashSetAgainstModel does the same for LinkedHashSet, including the string entry points.
func TestHashSetAgainstModel(t *testing.T) {
	for seed := int64(1); seed <= 40; seed++ {
		rnd := rand.New(rand.NewSource(seed))
		span := []int{3, 6, 12, 40, 200}[seed%5]
		s, ref := new_LinkedHashSet(), &refMap_{}
		for step := 0; step < 600; step++ {
			n := rnd.Intn(span)
			var k any = int32(n)
			str := "k" + istr_(int64(n))
			isStr := rnd.Intn(2) == 0
			if isStr {
				k = str
			} else if n%3 == 0 {
				k = &tkey_{int32(n)}
			}
			i := ref.find(k)
			switch op := rnd.Intn(10); {
			case op < 4:
				got := false
				if isStr && rnd.Intn(2) == 0 {
					got = Collection_add_s(s, str)
				} else {
					got = s.add(k)
				}
				if got != (i < 0) {
					t.Fatalf("seed %d: add(%v) = %v", seed, k, got)
				}
				if i < 0 {
					ref.put(k, nil)
				}
			case op < 7:
				got := false
				if isStr && rnd.Intn(2) == 0 {
					got = Collection_remove_s(s, str)
				} else {
					got = s.remove_Object(k)
				}
				if got != (i >= 0) {
					t.Fatalf("seed %d: remove(%v) = %v", seed, k, got)
				}
				if i >= 0 {
					ref.removeAt(i)
				}
			case op < 9:
				got := s.contains(k)
				if isStr {
					got = Collection_contains_s(s, str)
				}
				if got != (i >= 0) {
					t.Fatalf("seed %d: contains(%v) = %v", seed, k, got)
				}
			default:
				for it, j := s.iterator(), 0; it.hasNext(); {
					if x := it.next(); !objequals_(x, ref.keys[j]) {
						t.Fatalf("seed %d: iterator yields %v, want %v", seed, x, ref.keys[j])
					}
					if rnd.Intn(4) == 0 {
						Iterator_remove(it)
						ref.removeAt(j)
					} else {
						j++
					}
				}
			}
			if !sameSlice_(s.toArray_0(), ref.keys) || int(s.size()) != len(ref.keys) {
				t.Fatalf("seed %d step %d: got %v, want %v", seed, step, s.toString(), ref.keys)
			}
		}
	}
}
