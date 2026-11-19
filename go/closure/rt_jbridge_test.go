package closure

import "testing"

// The erased for-each cursor over the views of typed collections (step_) visits what the views'
// iterators visit, in their order, also after removals.
func TestJBridgeStep(t *testing.T) {
	c := &tconv_[string]{box: func(v string) any { return strobj_(v) }, unbox: func(v any) string { return str_(v) }, null: nullstr_}
	l := newTList_(c)
	m := newTMap_(c, c)
	s := newTSet_(c)
	for _, k := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"} {
		l.add(k)
		m.Put(k, k+"v")
		s.Add(k)
	}
	m.Delete("a")
	m.Delete("e")
	s.Delete("k")
	for name, coll := range map[string]Iterable{"list": l.asList(), "set": s.asSet(), "keys": m.asMap().keySet(), "values": m.asMap().values()} {
		var a, b []string
		for it := coll.iterator(); it.hasNext(); {
			a = append(a, str_(it.next()))
		}
		it := foreach_(coll)
		if it.at == nil {
			t.Fatalf("%s: the cursor does not step through the view", name)
		}
		for it.next() {
			b = append(b, str_(it.v))
		}
		if len(a) == 0 || len(a) != len(b) {
			t.Fatalf("%s: iterator %v, cursor %v", name, a, b)
		}
		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("%s: iterator %v, cursor %v", name, a, b)
			}
		}
	}
	n := 0
	for it := foreach_(m.asMap().entrySet()); it.next(); n++ {
		e := it.v.(Map_Entry)
		if str_(e.getValue()) != str_(e.getKey())+"v" {
			t.Fatalf("entry %v", objstr_(e))
		}
	}
	if n != 9 {
		t.Fatalf("%d entries", n)
	}
}
