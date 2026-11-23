package closure

import (
	"fmt"
	"testing"
)

// A for-each loop over the erased view of a typed collection reads by position what the
// view's iterator reads, also when the loop's body changes the collection.
func TestTypedViewSteppers(t *testing.T) {
	show := func(v any) string {
		if e, ok := v.(Map_Entry); ok {
			return fmt.Sprint(e.getKey(), "=", e.getValue())
		}
		return fmt.Sprint(v)
	}
	walk := func(c any, viaIter bool, body func(n int)) string {
		out := ""
		n := 0
		if viaIter {
			for it := c.(Iterable).iterator(); it.hasNext(); n++ {
				out += show(it.next()) + ";"
				body(n)
			}
			return out
		}
		e := foreach_(c)
		if e.at == nil {
			t.Fatalf("%T is read through an iterator object", c)
		}
		for ; e.next(); n++ {
			out += show(e.v) + ";"
			body(n)
		}
		return out
	}
	type views struct {
		name string
		mk   func() (view any, grow func(), cut func())
	}
	strs := func(n int) []string {
		var s []string
		for i := 0; i < n; i++ {
			s = append(s, fmt.Sprint("k", i))
		}
		return s
	}
	mkMap := func(pick func(m Map) any) func() (any, func(), func()) {
		return func() (any, func(), func()) {
			m := newTMap_(tc_string, tc_string)
			for _, k := range strs(40) {
				m.Put(k, "v"+k)
			}
			for _, k := range strs(7) { // removed entries at the head and inside
				m.asMap().remove(strobj_(k))
			}
			m.asMap().remove(strobj_("k20"))
			return pick(m.asMap()), func() { m.Put(fmt.Sprint("n", m.asMap().size()), "x") }, func() { m.asMap().remove(strobj_("k30")) }
		}
	}
	cases := []views{
		{"list", func() (any, func(), func()) {
			l := newTList_(tc_string)
			for _, k := range strs(12) {
				l.add(k)
			}
			return l.asList(), func() { l.add("n") }, func() { l.removeAt(l.size() - 1) }
		}},
		{"empty list", func() (any, func(), func()) {
			l := newTList_(tc_string)
			return l.asList(), func() {}, func() {}
		}},
		{"set", func() (any, func(), func()) {
			s := newTSet_(tc_string)
			for _, k := range strs(40) {
				s.Add(k)
			}
			for _, k := range strs(5) {
				s.asSet().remove_Object(strobj_(k))
			}
			return s.asSet(), func() { s.Add(fmt.Sprint("n", s.asSet().size())) }, func() { s.asSet().remove_Object(strobj_("k30")) }
		}},
		{"keys", mkMap(func(m Map) any { return m.keySet() })},
		{"values", mkMap(func(m Map) any { return m.values() })},
		{"entries", mkMap(func(m Map) any { return m.entrySet() })},
	}
	for _, c := range cases {
		for _, change := range []string{"none", "grow", "cut"} {
			var got [2]string
			for i, viaIter := range []bool{true, false} {
				view, grow, cut := c.mk()
				got[i] = walk(view, viaIter, func(n int) {
					if n == 3 && change == "grow" {
						grow()
					}
					if n == 3 && change == "cut" {
						cut()
					}
				})
			}
			if got[0] != got[1] {
				t.Errorf("%s (%s): iterator %q, for-each %q", c.name, change, got[0], got[1])
			}
			if change == "none" && c.name != "empty list" && got[1] == "" {
				t.Errorf("%s: nothing read", c.name)
			}
		}
	}
}
