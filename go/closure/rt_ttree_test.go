package closure

import (
	"math/rand"
	"testing"
)

var tcTreeStr_ = &tconv_[string]{box: strobj_, unbox: str_, null: nullstr_}

type ttreeItem_ struct{ n int32 }

var tcTreeItem_ = &tconv_[*ttreeItem_]{box: func(v *ttreeItem_) any { return obj_(v) }, unbox: func(v any) *ttreeItem_ { return cast_[*ttreeItem_](v) }}

// TestTTreeSetAgainstTreeSet checks the typed sorted set against the boxed one, under natural
// ordering (strings) and a comparator (by n descending, so distinct items can compare equal).
func TestTTreeSetAgainstTreeSet(t *testing.T) {
	r := rand.New(rand.NewSource(5))
	items := make([]*ttreeItem_, 12)
	for i := range items {
		items[i] = &ttreeItem_{int32(i % 7)}
	}
	cmp := &cmpFunc_{func(a, b any) int32 { return b.(*ttreeItem_).n - a.(*ttreeItem_).n }}
	same := func(round int, want, got []any) {
		t.Helper()
		if len(want) != len(got) {
			t.Fatalf("round %d: %v != %v", round, want, got)
		}
		for i := range want {
			if want[i] != got[i] {
				t.Fatalf("round %d: %v != %v", round, want, got)
			}
		}
	}
	for round := 0; round < 200; round++ {
		s, ts := new_TreeSet(), newTTreeSet_(tcTreeStr_)
		c, tc := new_TreeSet_Comparator(cmp), newTTreeSetC_(tcTreeItem_, cmp)
		for op := 0; op < 40; op++ {
			k := string(rune('a' + r.Intn(10)))
			it := items[r.Intn(len(items))]
			switch r.Intn(4) {
			case 0, 1:
				if s.add(k) != ts.add(k) || c.add(it) != tc.add(it) {
					t.Fatalf("round %d: add differs", round)
				}
			case 2:
				if s.remove_Object(k) != ts.remove(k) || c.remove_Object(it) != tc.remove(it) {
					t.Fatalf("round %d: remove differs", round)
				}
			case 3:
				if s.contains(k) != ts.contains(k) || c.contains(it) != tc.contains(it) {
					t.Fatalf("round %d: contains differs", round)
				}
			}
		}
		same(round, s.toArray_0(), ts.asSet().toArray_0())
		same(round, c.toArray_0(), tc.asSet().toArray_0())
		var got []any
		for e := ts.each(); e.next(); {
			got = append(got, e.v)
		}
		same(round, s.toArray_0(), got)
		same(round, s.descendingSet().toArray_0(), ts.asSet().descendingSet().toArray_0())
		same(round, s.tailSet("e", false).toArray_0(), ts.asSet().tailSet("e", false).toArray_0())
		if s.toString() != collToString_(ts.asSet()) || s.hashCode() != setHash_(ts.asSet()) || !s.equals(ts.asSet()) {
			t.Fatalf("round %d: erased face differs", round)
		}
		if comparatorOf_(tc.asSet()) != Comparator(cmp) || comparatorOf_(ts.asSet()) != nil {
			t.Fatalf("round %d: comparator()", round)
		}
	}
	// A null element under natural ordering throws as the boxed set does.
	panics := func(f func()) (p bool) {
		defer func() { p = recover() != nil }()
		f()
		return
	}
	s, ts := new_TreeSet(), newTTreeSet_(tcTreeStr_)
	s.add("a")
	ts.add("a")
	if !panics(func() { s.add(nil) }) || !panics(func() { ts.add(nullstr_) }) {
		t.Fatal("a null element under natural ordering does not throw")
	}
}

// TestTTreeMapAgainstTreeMap checks the typed sorted map against the boxed one.
func TestTTreeMapAgainstTreeMap(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	for round := 0; round < 200; round++ {
		m, tm := new_TreeMap(), newTTreeMap_(tcTreeStr_, tcTreeStr_)
		for op := 0; op < 40; op++ {
			k := string(rune('a' + r.Intn(10)))
			v := string(rune('A' + r.Intn(26)))
			switch r.Intn(5) {
			case 0, 1:
				if str_(m.put(k, v)) != tm.put(k, v) {
					t.Fatalf("round %d: put differs", round)
				}
			case 2:
				if str_(m.remove(k)) != tm.remove(k) {
					t.Fatalf("round %d: remove differs", round)
				}
			case 3:
				if str_(m.get(k)) != tm.get(k) || m.containsKey(k) != tm.containsKey(k) {
					t.Fatalf("round %d: get differs", round)
				}
			case 4:
				if str_(Map_putIfAbsent(m, k, v)) != tm.putIfAbsent(k, v) {
					t.Fatalf("round %d: putIfAbsent differs", round)
				}
			}
		}
		if m.toString() != mapToString_(tm.asMap()) || m.hashCode() != mapHash_(tm.asMap()) || !m.equals(tm.asMap()) {
			t.Fatalf("round %d: %s != %s", round, m.toString(), mapToString_(tm.asMap()))
		}
		var ks, vs string
		for e := tm.each(); e.next(); {
			ks += e.k
			vs += e.v
		}
		var wk, wv string
		for _, e := range m.entries {
			wk += e.key.(string)
			wv += e.val.(string)
		}
		if ks != wk || vs != wv {
			t.Fatalf("round %d: iteration differs", round)
		}
		// setValue through entrySet writes the map.
		for it := tm.asMap().entrySet().iterator(); it.hasNext(); {
			it.next().(Map_Entry).setValue("z")
		}
		for e := tm.each(); e.next(); {
			if e.v != "z" {
				t.Fatalf("round %d: setValue not written", round)
			}
		}
	}
}
