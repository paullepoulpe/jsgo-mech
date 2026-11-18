package closure

import (
	"math/rand"
	"slices"
	"testing"
)

// The comparator combinators and the PriorityQueue's int-key path call the key function on the
// same elements in the same order as the closures they replace (the JDK's lambdas), and the
// queue gives the elements out in the same order.
func TestComparatorCombinatorsKeepCallOrder(t *testing.T) {
	type item struct{ pri, id int32 }
	r := rand.New(rand.NewSource(7))
	var items []any
	for i := int32(0); i < 300; i++ {
		items = append(items, &item{int32(r.Intn(40)), i})
	}
	for _, rev := range []bool{false, true} {
		var gotCalls, wantCalls []int32
		key := func(calls *[]int32) func(any) int32 {
			return func(v any) int32 { it := v.(*item); *calls = append(*calls, it.id); return it.pri }
		}
		// The model: the JDK's comparingInt and reversed, as closures behind a Comparator_fn,
		// which the queue cannot see through.
		wk := key(&wantCalls)
		var model Comparator = &Comparator_fn{func(a, b any) int32 { return Integer_compare(wk(a), wk(b)) }}
		if rev {
			inner := model
			model = &Comparator_fn{func(a, b any) int32 { return inner.compare(b, a) }}
		}
		var c Comparator = Comparator_comparingInt(&ToIntFunction_fn{key(&gotCalls)})
		if rev {
			c = Comparator_reversed(c)
		}
		if _, ok := c.(*intKeyCmp_); !ok {
			t.Fatalf("comparingInt (reversed %v) is %T, not an int-key comparator", rev, c)
		}
		got, want := new_PriorityQueue(c), new_PriorityQueue(model)
		var gotOut, wantOut []int32
		for i, v := range items {
			got.add(v)
			want.add(v)
			if i%3 == 2 {
				gotOut = append(gotOut, got.poll().(*item).id)
				wantOut = append(wantOut, want.poll().(*item).id)
			}
		}
		got.remove_Object(items[10])
		want.remove_Object(items[10])
		for !want.isEmpty() {
			gotOut = append(gotOut, got.poll().(*item).id)
			wantOut = append(wantOut, want.poll().(*item).id)
		}
		if !slices.Equal(gotOut, wantOut) {
			t.Errorf("reversed %v: poll order differs", rev)
		}
		if !slices.Equal(gotCalls, wantCalls) {
			t.Errorf("reversed %v: key calls differ (%d against %d)", rev, len(gotCalls), len(wantCalls))
		}
	}

	// reversed twice is the comparator itself; thenComparing breaks ties with its second.
	byPri := Comparator_comparingInt(&ToIntFunction_fn{func(v any) int32 { return v.(*item).pri }})
	if c := Comparator_reversed(Comparator_reversed(byPri)).(*intKeyCmp_); c.rev {
		t.Errorf("reversed twice is reversed")
	}
	byID := &Comparator_fn{func(a, b any) int32 { return Integer_compare(a.(*item).id, b.(*item).id) }}
	if r := Comparator_reversed(Comparator_reversed(byID)); r != Comparator(byID) {
		t.Errorf("reversed twice of a comparator is not that comparator")
	}
	a, b := &item{1, 2}, &item{1, 3}
	if Comparator_thenComparing_Comparator(byPri, byID).compare(a, b) >= 0 ||
		Comparator_thenComparing_Comparator(byPri, Comparator_reversed(byID)).compare(a, b) <= 0 {
		t.Errorf("thenComparing does not break ties with its second comparator")
	}
}
