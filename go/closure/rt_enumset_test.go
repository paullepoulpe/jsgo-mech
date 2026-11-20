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
		s, m := EnumSet_noneOf(classOf_("test.TEnum")), new_TreeSet_Comparator(ordinalCmp_())
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
