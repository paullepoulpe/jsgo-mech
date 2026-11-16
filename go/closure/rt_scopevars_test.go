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
		for i, e := range m.ents {
			want := i
			if i >= 1 {
				want = i + 1
			}
			if e.name != fmt.Sprint("v", want) || m.get(e.name) == nil {
				t.Fatalf("n=%d: order after remove at %d: %s", n, i, e.name)
			}
		}
		m.put("v1", vars[1])
		if m.ents[len(m.ents)-1].name != "v1" || m.get("v1") != vars[1] {
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
