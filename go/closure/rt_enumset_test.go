package closure

import (
	"math/rand"
	"testing"
)

// tenum_ is an enum of 70 constants (more than one word of bits); tenum2_ is another enum.
type tenum_ struct{ Enum }

func (this *tenum_) up_Enum() *Enum { return &this.Enum }

type tenum2_ struct{ Enum }

func (this *tenum2_) up_Enum() *Enum { return &this.Enum }

func tenums_() ([]*tenum_, *tenum2_) {
	var vs []*tenum_
	for i := 0; i < 70; i++ {
		v := &tenum_{}
		v.self_ = v
		v.init_Enum("C"+istr_(int64(i)), int32(i))
		vs = append(vs, v)
	}
	registerEnum_("test.TEnum", enumValues_(vs))
	o := &tenum2_{}
	o.self_ = o
	o.init_Enum("D0", 0)
	registerEnum_("test.TEnum2", enumValues_([]*tenum2_{o}))
	return vs, o
}

// TestEnumSetAgainstTreeSet checks EnumSet against the sorted set it replaces.
func TestEnumSetAgainstTreeSet(t *testing.T) {
	vs, other := tenums_()
	r := rand.New(rand.NewSource(3))
	for round := 0; round < 200; round++ {
		s, m := EnumSet_noneOf(classOf_("test.TEnum")), &TreeSet{cmp: ordinalCmp_()}
		for op := 0; op < 60; op++ {
			v := any(vs[r.Intn(len(vs))])
			switch r.Intn(4) {
			case 0, 1:
				if s.add(v) != m.add(v) {
					t.Fatalf("add %v", v)
				}
			case 2:
				if s.remove_Object(v) != m.remove_Object(v) {
					t.Fatalf("remove %v", v)
				}
			case 3:
				for it := s.iterator(); it.hasNext(); {
					if x := it.next(); r.Intn(5) == 0 {
						Iterator_remove(it)
						m.remove_Object(x)
					}
				}
			}
			if s.size() != m.size() || s.isEmpty() != m.isEmpty() || s.toString() != m.toString() || s.hashCode() != m.hashCode() {
				t.Fatalf("round %d: %s vs %s", round, s.toString(), m.toString())
			}
			if !s.equals(m) || !s.containsAll(m) || !m.containsAll(s) || s.contains(other) || s.contains(nil) || s.contains("C1") {
				t.Fatalf("round %d: membership", round)
			}
			c := EnumSet_copyOf(s)
			if !c.equals(s) || !s.equals(c) || !s.containsAll(c) {
				t.Fatalf("round %d: copy", round)
			}
			if c.add(vs[69]) && (s.containsAll(c) || s.equals(c)) {
				t.Fatalf("round %d: copy differs", round)
			}
		}
	}
	if all := EnumSet_allOf(classOf_("test.TEnum")); all.size() != 70 || all.toArray_0()[69] != any(vs[69]) {
		t.Fatalf("allOf")
	}
	e := EnumSet_of(other.up_Enum())
	e.clear()
	if !EnumSet_noneOf(nil).equals(e) || !e.equals(EnumSet_noneOf(nil)) {
		t.Fatalf("empty sets of two enums are equal")
	}
}

// TestImmutableCopyOfEnumSet checks ImmutableSet.copyOf of an EnumSet (indexed by ordinal) against
// the copy of the same constants from a list.
func TestImmutableCopyOfEnumSet(t *testing.T) {
	vs, other := tenums_()
	r := rand.New(rand.NewSource(5))
	for round := 0; round < 100; round++ {
		s := EnumSet_noneOf(classOf_("test.TEnum"))
		for i := r.Intn(20); i > 0; i-- {
			s.add(any(vs[r.Intn(len(vs))]))
		}
		got, want := ImmutableSet_copyOf(s), immSet_(copySlice_(new_ArrayList_Collection(s)))
		if got.toString() != want.toString() || got.hashCode() != want.hashCode() || !got.equals(want) {
			t.Fatalf("round %d: %s vs %s", round, got.toString(), want.toString())
		}
		probe := EnumSet_noneOf(classOf_("test.TEnum"))
		for i := r.Intn(3); i > 0; i-- {
			probe.add(any(vs[r.Intn(len(vs))]))
		}
		for _, v := range append([]any{other, "C1"}, enumValues_(vs)...) {
			if got.contains(v) != want.contains(v) {
				t.Fatalf("round %d: contains %v", round, v)
			}
		}
		if got.containsAll(probe) != want.containsAll(probe) {
			t.Fatalf("round %d: containsAll %s in %s", round, probe.toString(), got.toString())
		}
	}
	if ImmutableSet_copyOf(EnumSet_noneOf(nil)) != emptyImmSet_ {
		t.Fatal("copy of an empty EnumSet is not the empty set")
	}
}

// TestEnumMapAgainstTreeMap checks EnumMap against the ordinal-ordered TreeMap it replaces, and
// Java's null semantics (null values are kept; null and foreign keys are not keys).
func TestEnumMapAgainstTreeMap(t *testing.T) {
	vs, other := tenums_()
	r := rand.New(rand.NewSource(5))
	for round := 0; round < 200; round++ {
		var e *EnumMap
		if round%2 == 0 {
			e = new_EnumMap(classOf_("test.TEnum"))
		} else {
			e = new_EnumMap(classOf_("test.NotRegistered")) // the enum is taken from the first key
		}
		m := &TreeMap{cmp: ordinalCmp_()}
		for op := 0; op < 60; op++ {
			k := any(vs[r.Intn(len(vs))])
			var v any
			if r.Intn(4) > 0 {
				v = strobj_(istr_(int64(r.Intn(5))))
			}
			switch r.Intn(5) {
			case 0, 1:
				if a, b := e.put(k, v), m.put(k, v); !objequals_(a, b) {
					t.Fatalf("put %v: %v vs %v", k, a, b)
				}
			case 2:
				if a, b := e.remove(k), m.remove(k); !objequals_(a, b) {
					t.Fatalf("remove %v", k)
				}
			case 3:
				for it := e.entrySet().iterator(); it.hasNext(); {
					x := it.next().(Map_Entry)
					if r.Intn(3) == 0 {
						x.setValue(v)
						m.put(x.getKey(), v)
					}
				}
			case 4:
				if e.containsValue(v) != m.containsValue(v) || !objequals_(e.get(k), m.get(k)) || e.containsKey(k) != m.containsKey(k) {
					t.Fatalf("lookup %v", k)
				}
			}
			if e.size() != m.size() || e.isEmpty() != m.isEmpty() || e.toString() != m.toString() || e.hashCode() != m.hashCode() {
				t.Fatalf("round %d: %s vs %s", round, e.toString(), m.toString())
			}
			if !e.equals(m) || !m.equals(e) || collToString_(e.keySet()) != collToString_(m.keySet()) || collToString_(e.values()) != collToString_(m.values()) {
				t.Fatalf("round %d: views", round)
			}
			if e.containsKey(other) || e.containsKey(nil) || e.containsKey("C1") || e.get(nil) != nil || e.remove(other) != nil {
				t.Fatalf("round %d: foreign keys", round)
			}
		}
	}
	e := new_EnumMap(classOf_("test.TEnum"))
	e.put(vs[3], nil)
	if !e.containsKey(vs[3]) || e.get(vs[3]) != nil || e.size() != 1 || !e.containsValue(nil) || e.toString() != "{C3=null}" {
		t.Fatalf("a null value is a mapping: %s", e.toString())
	}
	for _, bad := range []any{nil, other} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("put of %v does not throw", bad)
				}
			}()
			e.put(bad, nil)
		}()
	}
}
