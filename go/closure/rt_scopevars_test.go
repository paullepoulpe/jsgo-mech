package closure

import (
	"fmt"
	"testing"
)

// scopeVars_ behaves as a LinkedHashMap<String, V>: insertion order, replacement in place,
// removal closing the gap, below and above the indexed size.
func TestScopeVars(t *testing.T) {
	for _, n := range []int{3, 4, 5, 8, 9, 30, 200} {
		m := newScopeVars_()
		vars := make([]*AbstractVar, n)
		for i := range vars {
			vars[i] = &AbstractVar{}
			m.put(fmt.Sprint("v", i), vars[i])
		}
		if m.size() != int32(n) {
			t.Fatalf("n=%d: size %d", n, m.size())
		}
		var h uint32
		for i := range vars {
			name := fmt.Sprint("v", i)
			if m.get(name) != vars[i] || m.lookup(name, &h) != vars[i] || !m.containsKey(name) {
				t.Fatalf("n=%d: get %s", n, name)
			}
			h = 0
		}
		if m.get("absent") != nil || m.containsKey("absent") {
			t.Fatalf("n=%d: absent found", n)
		}
		repl := &AbstractVar{}
		m.put("v0", repl)
		if m.get("v0") != repl || m.ents[0].v != repl || m.size() != int32(n) {
			t.Fatalf("n=%d: replace", n)
		}
		m.remove("v1")
		m.remove("absent")
		if m.size() != int32(n-1) || m.get("v1") != nil {
			t.Fatalf("n=%d: remove", n)
		}
		for i := range m.ents {
			e := &m.ents[i]
			want := i
			if i >= 1 {
				want = i + 1
			}
			if e.name() != fmt.Sprint("v", want) || m.get(e.name()) == nil {
				t.Fatalf("n=%d: order after remove at %d: %s", n, i, e.name())
			}
		}
		m.put("v1", vars[1])
		if m.ents[len(m.ents)-1].name() != "v1" || m.get("v1") != vars[1] {
			t.Fatalf("n=%d: re-put goes last", n)
		}
		m.clear()
		if m.size() != 0 || m.get("v2") != nil {
			t.Fatalf("n=%d: clear", n)
		}
	}
	var nilTable *scopeVars_
	var h uint32
	if nilTable.size() != 0 || nilTable.get("x") != nil || nilTable.lookup("x", &h) != nil || nilTable.containsKey("x") {
		t.Fatal("nil table")
	}
}

// A lookup by the string id of a name finds what the lookup by the name finds, in tables
// below and above the indexed size, with ids (putId) and without (put), also when a table
// with ids is given a name without one, and after a removal has rebuilt the slots.
func TestScopeVarsById(t *testing.T) {
	for _, n := range []int{1, 4, 8, 9, 10, 13, 64, 500, 1001, 1002} {
		withIds, mixed := n%2 == 0, n == 10 || n == 1002
		m := newScopeVars_()
		vars := make([]*AbstractVar, n)
		put := func(name string, v *AbstractVar) {
			if withIds {
				m.putId(name, strid_(name), v)
			} else {
				m.put(name, v)
			}
		}
		for i := range vars {
			vars[i] = &AbstractVar{}
			if i == n-1 && mixed {
				m.put(fmt.Sprint("idv", i), vars[i]) // the table drops its ids
			} else {
				put(fmt.Sprint("idv", i), vars[i])
			}
		}
		check := func(what string) {
			for i := range vars {
				name := fmt.Sprint("idv", i)
				if got := m.lookupId(strid_(name)); got != m.get(name) {
					t.Fatalf("n=%d %s: lookupId(%s) = %p, get = %p", n, what, name, got, m.get(name))
				}
			}
			if m.lookupId(strid_("idv-absent")) != nil || m.findId(strid_("")) != -1 {
				t.Fatalf("n=%d %s: absent found", n, what)
			}
		}
		check("filled")
		if m.lookupId(strid_("idv0")) != vars[0] {
			t.Fatalf("n=%d: idv0", n)
		}
		m.remove(fmt.Sprint("idv", n/2))
		check("after remove")
		repl := &AbstractVar{}
		put(fmt.Sprint("idv", n-1), repl)
		check("after replace")
		put("idv-new", repl)
		if m.lookupId(strid_("idv-new")) != repl {
			t.Fatalf("n=%d: new", n)
		}
		m.clear()
		if m.lookupId(strid_("idv0")) != nil {
			t.Fatalf("n=%d: clear", n)
		}
	}
	var nilTable *scopeVars_
	if nilTable.lookupId(1) != nil {
		t.Fatal("nil table")
	}
	for _, s := range []string{"arguments", "super", "this", "exports", "", "x", "thiss"} {
		if AbstractScope_ImplicitVar_ofId_(strid_(s)) != AbstractScope_ImplicitVar_of(s) {
			t.Fatalf("implicit var of %q", s)
		}
	}
}
