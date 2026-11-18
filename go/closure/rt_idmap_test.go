package closure

import (
	"math/rand"
	"testing"
)

// TestLihmAgainstLinkedIdentityHashMap checks LinkedIdentityHashMap's methods on a typed map
// (rt_idmap.go) against the translated class, with null results and values.
func TestLihmAgainstLinkedIdentityHashMap(t *testing.T) {
	type key struct{ n int }
	keys := make([]*key, 8)
	for i := range keys {
		keys[i] = &key{i % 3} // equal fields, distinct identities
	}
	kc := &tconv_[*key]{box: func(v *key) any { return obj_(v) }, unbox: func(v any) *key { return cast_[*key](v) }}
	vc := &tconv_[any]{box: func(v any) any { return v }, unbox: func(v any) any { return v }}
	r := rand.New(rand.NewSource(11))
	for round := 0; round < 200; round++ {
		m, tm := new_LinkedIdentityHashMap(), newTMap_(kc, vc)
		for op := 0; op < 30; op++ {
			k := keys[r.Intn(len(keys))]
			var v any
			if x := r.Intn(4); x > 0 {
				v = any(int32(x))
			}
			fn := func(any) any { return v }
			mf := func(a, b any) any {
				if a == nil {
					return b
				}
				return any(a.(int32) + 10)
			}
			var a, b any
			switch r.Intn(4) {
			case 0:
				a, b = m.get(obj_(k)), tm.get(k)
			case 1:
				a, b = m.put(obj_(k), v), tm.put(k, v)
			case 2:
				a, b = m.computeIfAbsent(obj_(k), fn), lihmComputeIfAbsent_(tm, k, fn)
			case 3:
				if v != nil {
					a, b = m.merge(obj_(k), v, mf), lihmMerge_(tm, k, v, mf)
				}
			}
			if a != b {
				t.Fatalf("round %d op %d: %v != %v", round, op, a, b)
			}
		}
		var want, got []any
		m.forEach(func(k, v any) { want = append(want, k, v) })
		lihmForEach_(tm, func(k, v any) { got = append(got, k, v) })
		if len(want) != len(got) {
			t.Fatalf("round %d: forEach %v != %v", round, want, got)
		}
		for i := range want {
			if want[i] != got[i] {
				t.Fatalf("round %d: forEach %v != %v", round, want, got)
			}
		}
	}
}

// TestTlistFromRev checks a deque's copy as a list whose end is its head.
func TestTlistFromRev(t *testing.T) {
	src := listOf_([]any{"a", "b", "c"})
	d := new_ArrayDeque_Collection(src)
	l := tlistFromRev_(tcTreeStr_, src)
	for !d.isEmpty() {
		if str_(d.pop()) != l.pop_() {
			t.Fatal("pop order differs")
		}
	}
	if len(l.items) != 0 {
		t.Fatal("lengths differ")
	}
}
