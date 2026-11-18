package closure

import (
	"math/rand"
	"testing"
)

// The table of distinct values (htab_set_, htab_copy_): a field keeps reading the value stored
// in it, with its flag; storing a held value again gives the same handle; and a value that no
// field holds any more leaves the table, which so stays as small as what is held.
func TestHandleTableCounts(t *testing.T) {
	type obj struct{ i int }
	const slots = 50
	var held [slots]uint32 // the field of 50 objects
	var vals [slots]*obj
	before := len(htabIds_)
	rnd := rand.New(rand.NewSource(2))
	check := func(step int) {
		t.Helper()
		distinct := map[*obj]bool{}
		for j, c := range held {
			var want any
			if vals[j] != nil {
				want = vals[j]
				distinct[vals[j]] = true
			}
			if got := htab_get_(c); got != want {
				t.Fatalf("step %d: slot %d reads %v, not %v", step, j, got, want)
			}
			if c != 0 && (c&1 != 0) != (j%2 == 0) {
				t.Fatalf("step %d: slot %d lost its flag", step, j)
			}
		}
		if len(htabIds_)-before != len(distinct) {
			t.Fatalf("step %d: %d entries for %d held values", step, len(htabIds_)-before, len(distinct))
		}
	}
	for step := 0; step < 20000; step++ {
		i := rnd.Intn(slots)
		switch k := rnd.Intn(10); {
		case k < 4:
			vals[i] = &obj{step}
			htab_set_(&held[i], vals[i], i%2 == 0)
		case k < 6:
			j := rnd.Intn(slots)
			if vals[j] == nil {
				continue
			}
			vals[i] = vals[j]
			htab_set_(&held[i], vals[i], i%2 == 0)
			if held[i]>>1 != held[j]>>1 {
				t.Fatalf("step %d: a held value got a second handle", step)
			}
		case k < 9:
			// A copy between two fields of the same kind (so with the same flag).
			j := (i + 2*rnd.Intn(slots/2)) % slots
			vals[i] = vals[j]
			htab_copy_(&held[i], held[j])
		default:
			vals[i] = nil
			htab_set_(&held[i], nil, false)
		}
		check(step)
	}
	for i := range held {
		vals[i] = nil
		htab_set_(&held[i], nil, false)
	}
	check(-1)
	if htabTab_.n > 1000 {
		t.Fatalf("freed entries are not used again: %d entries", htabTab_.n)
	}
	// A pinned value stays without a field.
	p := &obj{-1}
	c := htab_pin_(p)
	var f uint32
	htab_copy_(&f, c)
	htab_set_(&f, nil, false)
	if htab_get_(c) != any(p) || htab_pin_(p) != c {
		t.Fatal("a pinned value left the table")
	}
}
