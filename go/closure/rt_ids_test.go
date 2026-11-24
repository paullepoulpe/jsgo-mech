package closure

import (
	"math/rand"
	"testing"
	"unsafe"
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

// Values that keep their handle themselves (htabHolder_, post-flip htabself): the handles, the
// counts and the reuse of freed entries are those of the table without slots, which the test
// models (a map from interface words to a handle, the freed handles reused last freed first),
// over random stores, copies and clears of a mix of values: objects with a slot, objects of a
// class that embeds one (the promoted slot), the embedded part itself (the same address with
// another type: the same slot, another value, which the map keeps), objects without a slot,
// a holder whose slot holds a handle that is not its own (as a copied struct would), and
// pinned values.
type htabSlotObj_ struct {
	i       int
	htabId_ uint32
}

func (o *htabSlotObj_) htabSlot_() *uint32 { return &o.htabId_ }

type htabSlotSub_ struct {
	htabSlotObj_
	j int
}

func TestHandleTableSlots(t *testing.T) {
	type plain struct{ i int }
	const slots = 64
	var held [slots]uint32
	var vals [slots]any
	// the model: handles by interface words, counts, the free list
	ids := map[any]uint32{}
	refs := map[uint32]uint32{}
	pinned := map[uint32]bool{}
	free := append([]uint32(nil), htabFree_...)
	next := htabTab_.n
	ref := func(v any) uint32 {
		id, ok := ids[v]
		if !ok {
			if n := len(free); n > 0 {
				id, free = free[n-1], free[:n-1]
			} else {
				id, next = next, next+1
			}
			ids[v] = id
		}
		if !pinned[id] {
			refs[id]++
		}
		return id
	}
	unref := func(c uint32) {
		id := c >> 1
		if pinned[id] {
			return
		}
		if refs[id]--; refs[id] == 0 {
			delete(refs, id)
			for v, i := range ids {
				if i == id {
					delete(ids, v)
				}
			}
			free = append(free, id)
		}
	}
	set := func(i int, v any) {
		old := held[i]
		var want uint32
		if v != nil {
			want = ref(v) << 1
		}
		if old != 0 {
			unref(old)
		}
		htab_set_(&held[i], v, false)
		if held[i] != want {
			t.Fatalf("slot %d: handle %d, the table without slots gives %d", i, held[i]>>1, want>>1)
		}
		vals[i] = v
	}
	rnd := rand.New(rand.NewSource(3))
	var pool []any
	for i := 0; i < 40; i++ {
		switch i % 5 {
		case 0, 1:
			pool = append(pool, &htabSlotObj_{i: i})
		case 2:
			s := &htabSlotSub_{j: i}
			pool = append(pool, s, &s.htabSlotObj_)
		case 3:
			pool = append(pool, &plain{i})
		case 4:
			pool = append(pool, &htabSlotObj_{i: i, htabId_: uint32(1 + rnd.Intn(int(htabTab_.n)+50))})
		}
	}
	for step := 0; step < 50000; step++ {
		i := rnd.Intn(slots)
		switch k := rnd.Intn(20); {
		case k < 12:
			set(i, pool[rnd.Intn(len(pool))])
		case k < 16:
			j := rnd.Intn(slots)
			if held[j] != 0 {
				ref(vals[j])
			}
			if held[i] != 0 {
				unref(held[i])
			}
			htab_copy_(&held[i], held[j])
			vals[i] = vals[j]
		case k < 19:
			set(i, nil)
		default:
			if step%97 == 0 { // pin a value for ever
				v := pool[rnd.Intn(len(pool))]
				id := ref(v)
				pinned[id] = true
				delete(refs, id)
				if c := htab_pin_(v); c != id<<1 {
					t.Fatalf("step %d: pinned handle %d, the table without slots gives %d", step, c>>1, id)
				}
			}
		}
		for j, c := range held {
			if got := htab_get_(c); got != vals[j] {
				t.Fatalf("step %d: slot %d reads %v, not %v", step, j, got, vals[j])
			}
		}
		if step%1000 == 0 || step == 49999 {
			for id, n := range refs {
				if p := htabTab_.ptr(id); p.refs != n {
					t.Fatalf("step %d: entry %d counted %d, the table without slots %d", step, id, p.refs, n)
				}
			}
			if len(free) != len(htabFree_) {
				t.Fatalf("step %d: %d free entries, the table without slots %d", step, len(htabFree_), len(free))
			}
			for x := range free {
				if free[x] != htabFree_[x] {
					t.Fatalf("step %d: free list differs at %d", step, x)
				}
			}
		}
	}
	if htabTab_.n != next {
		t.Fatalf("%d entries made, the table without slots makes %d", htabTab_.n, next)
	}
	if htabHoldersInMap_ == 0 || htabInSlots_ == 0 {
		t.Fatalf("not both ways taken: %d in slots, %d holders in the map", htabInSlots_, htabHoldersInMap_)
	}
	for i := range held {
		set(i, nil)
	}
	for _, v := range pool {
		if h, ok := v.(htabHolder_); ok && *h.htabSlot_() != 0 {
			if p := htabTab_.ptr(*h.htabSlot_()); p.refs != 0 && p.refs != htabPinned_ && *(*eface_)(unsafe.Pointer(&p.v)) == *(*eface_)(unsafe.Pointer(&v)) {
				t.Fatalf("a value no field holds keeps its entry")
			}
		}
	}
}
