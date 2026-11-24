package closure

import (
	"fmt"
	"math/rand"
	"testing"
)

type pqtItem_ struct{ pri, id int32 }

// TestPriorityQueueTyped: a queue made by newPriorityQueueT_ and the erased queue given the
// converting closure do the same thing: the same results of every operation, the same calls of
// the comparator and of the cast in the same order (a repeated cast of the same element
// aside), the same exception at the same call for an element that is not a T.
func TestPriorityQueueTyped(t *testing.T) {
	type trace struct {
		cmps  []string
		casts map[any]bool
		out   []string
	}
	mk := func(tr *trace) (func(a, b *pqtItem_) int32, func(v any) *pqtItem_) {
		cmp := func(a, b *pqtItem_) int32 {
			tr.cmps = append(tr.cmps, fmt.Sprint(a.id, "/", b.id))
			return Integer_compare(a.pri, b.pri)
		}
		unbox := func(v any) *pqtItem_ {
			tr.casts[v] = true
			return cast_[*pqtItem_](v)
		}
		return cmp, unbox
	}
	for seed := int64(1); seed <= 20; seed++ {
		var ta, tb trace
		ta.casts, tb.casts = map[any]bool{}, map[any]bool{}
		ca, ua := mk(&ta)
		cb, ub := mk(&tb)
		erased := new_PriorityQueue(func(p0 any, p1 any) int32 { return ca(ua(p0), ua(p1)) })
		typed := newPriorityQueueT_[*pqtItem_](cb, ub)
		r := rand.New(rand.NewSource(seed))
		var live []*pqtItem_
		step := func(tr *trace, q *PriorityQueue, op int, it *pqtItem_, bad any) {
			defer func() {
				if e := recover(); e != nil {
					tr.out = append(tr.out, fmt.Sprintf("throws %T", e))
				}
			}()
			switch op {
			case 0:
				q.add(it)
				tr.out = append(tr.out, "add")
			case 1:
				v := q.poll()
				tr.out = append(tr.out, fmt.Sprint("poll ", v))
			case 2:
				tr.out = append(tr.out, fmt.Sprint("remove ", q.remove_Object(it)))
			case 3:
				q.clear()
				tr.out = append(tr.out, "clear")
			case 4:
				q.add(bad)
				tr.out = append(tr.out, "add bad")
			case 5:
				tr.out = append(tr.out, fmt.Sprint("peek ", q.peek(), " size ", q.size()))
			}
		}
		for i := 0; i < 400; i++ {
			op := []int{0, 0, 0, 1, 1, 2, 5, 0, 1, 0}[r.Intn(10)]
			if i%97 == 96 {
				op = 3
			}
			if seed%5 == 0 && i == 150 {
				op = 4
			}
			it := &pqtItem_{pri: int32(r.Intn(12)), id: int32(i)}
			if op == 2 && len(live) > 0 {
				it = live[r.Intn(len(live))]
			}
			if op == 0 {
				live = append(live, it)
			}
			step(&ta, erased, op, it, "not an item")
			step(&tb, typed, op, it, "not an item")
			if fmt.Sprint(erased.items) != fmt.Sprint(typed.items) {
				t.Fatalf("seed %d step %d: items differ", seed, i)
			}
		}
		if fmt.Sprint(ta.out) != fmt.Sprint(tb.out) {
			t.Fatalf("seed %d: results differ", seed)
		}
		if fmt.Sprint(ta.cmps) != fmt.Sprint(tb.cmps) {
			t.Fatalf("seed %d: comparator calls differ (%d, %d)", seed, len(ta.cmps), len(tb.cmps))
		}
		if len(ta.casts) != len(tb.casts) {
			t.Fatalf("seed %d: elements cast differ (%d, %d)", seed, len(ta.casts), len(tb.casts))
		}
	}
	if q := newPriorityQueueT_[*pqtItem_](nil, cast_[*pqtItem_]); q.cmp != nil || q.typed != nil {
		t.Fatal("a null comparator: the natural-order queue")
	}
}
