package closure

import "testing"

// The bridge between the translator's typed collections and the post-flip ones (rt_jbridge.go,
// jtconv_.t_): conversions, cursors, entries and constructors behave as the translator's did.
func TestJBridge(t *testing.T) {
	npe := func(f func()) (thrown bool) {
		defer func() {
			if r := recover(); r != nil {
				_, thrown = r.(*NullPointerException)
			}
		}()
		f()
		return false
	}
	str := func(byEquals bool) *jtconv_[string] {
		return &jtconv_[string]{box: func(v string) any { return strobj_(v) }, unbox: func(v any) string { return str_(v) }, null: nullstr_, byEquals: byEquals}
	}
	// One post-flip conversion per translator conversion; hash and eq only for byEquals.
	plain, eq := str(false), str(true)
	if plain.t_() != plain.t_() || plain.t_().hash != nil || plain.t_().eq != nil {
		t.Fatal("t_ of a conversion compared by ==")
	}
	if eq.t_() != eq.t_() || eq.t_().hash == nil || eq.t_().eq == nil {
		t.Fatal("t_ of a conversion compared by equals")
	}
	for _, kc := range []*tconv_[string]{plain.t_(), eq.t_()} {
		m := newTMapCap_(kc, plain.t_(), 100)
		keys := []string{"b", "a", "d", "c", "k5", "k6", "k7", "k8", "k9", "k10", "k11"} // more than tsmall_
		for i, k := range keys {
			if old := m.Put(k, k+"v"); old != nullstr_ {
				t.Fatalf("put %d returned %q", i, old)
			}
		}
		m.Delete("d")
		var ks, vs []string
		for it := m.eachKey(); it.next(); {
			ks = append(ks, it.v)
		}
		for it := m.eachValue(); it.next(); {
			vs = append(vs, it.v)
		}
		if len(ks) != len(keys)-1 || ks[0] != "b" || ks[2] != "c" || vs[2] != "cv" || len(vs) != len(ks) {
			t.Fatalf("keys %v values %v", ks, vs)
		}
		// An entry reads and writes its mapping while the map has it, and keeps its last value
		// after the mapping's removal.
		n := 0
		for it := m.eachEntry(); it.next(); n++ {
			e := it.entry()
			if e.getKey() != ks[n] || e.getValue() != ks[n]+"v" {
				t.Fatalf("entry %d is %q=%q", n, e.getKey(), e.getValue())
			}
			if old := e.setValue("x" + e.getKey()); old != ks[n]+"v" || m.Get(e.getKey()) != "x"+e.getKey() {
				t.Fatalf("setValue of entry %d", n)
			}
			if n == 1 {
				view := e.asEntry()
				if str_(view.getKey()) != "a" || str_(view.getValue()) != "xa" || objstr_(view) != "a=xa" {
					t.Fatalf("erased entry %v", objstr_(view))
				}
				m.Delete("a")
				if e.getValue() != "xa" || e.setValue("gone") != "xa" || e.getValue() != "gone" || m.Has("a") {
					t.Fatal("an entry after its mapping's removal")
				}
			}
		}
		if n != len(ks) {
			t.Fatalf("visited %d of %d entries", n, len(ks))
		}
		o := newTMap_(kc, plain.t_())
		o.Put("z", "1")
		o.putAllT(m)
		if o.Len() != m.Len()+1 || o.Get("b") != "xb" {
			t.Fatal("putAllT")
		}
		c := tmapFromMap_(kc, plain.t_(), m.asMap())
		if c.Len() != m.Len() || c.Get("c") != "xc" {
			t.Fatal("tmapFromMap_")
		}
		if !npe(func() { tmapFromMap_(kc, plain.t_(), nil) }) {
			t.Fatal("tmapFromMap_ of null does not throw NullPointerException")
		}
		s := newTSetCap_(kc, 3)
		for _, k := range keys {
			s.Add(k)
		}
		var seen []string
		for it := s.iterator(); it.hasNext(); {
			k := str_(it.next())
			seen = append(seen, k)
			if k == "a" {
				Iterator_remove(it)
			}
		}
		if len(seen) != len(keys) || s.Has("a") || s.Len() != int32(len(keys)-1) {
			t.Fatalf("set iterator: saw %v, %d left", seen, s.Len())
		}
		var null *TSet[string]
		if !npe(func() { null.iterator() }) {
			t.Fatal("iterator of a null set does not throw NullPointerException")
		}
	}
}
