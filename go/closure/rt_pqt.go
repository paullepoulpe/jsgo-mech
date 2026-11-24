package closure

import "unsafe"

// A PriorityQueue made with a typed comparator (post-flip step 559, tools/postflip/tcmp).
//
// The translator gives an erased PriorityQueue the converting closure of a typed comparator
// cmp: func(a, b any) int32 { return cmp(K(a), K(b)) }, K the cast of an Object to the element
// type T. Every comparison of the heap then casts both elements again. newPriorityQueueT_ makes
// the same queue (the same heap code, the same erased comparator for whoever asks for it) that
// keeps, beside each element x, the value K(x) once it has been computed, and compares those
// with cmp.
//
// What makes it the same queue: K is a pure cast (for the same interface value it returns the
// same value or throws the same exception, and has no effect), so skipping a repeated K(x) is
// not observable; K(x) is first evaluated exactly where the erased comparator first evaluated
// it (compare(x, e) is K(x), K(e), cmp, in this order), so an element that is not a T throws
// at the same call; cmp is called with the same pairs in the same order (siftUp and siftDown
// below are PriorityQueue's, step for step). A kept value is used only for the identical
// interface value (sameBox_), so no operation the queue inherits from ArrayList can make it
// stale.

// pqTyped_ is the typed side of a PriorityQueue: the two heap moves, on the queue's items.
type pqTyped_ interface {
	siftUp(es []any, k int, x any)
	siftDown(es []any, k int, x any)
}

type pqEnt_[T any] struct {
	x any // the element t was computed from (nil: none)
	t T   // unbox(x)
}

type pqT_[T any] struct {
	ents  []pqEnt_[T] // ents[k] belongs to items[k] while its x is the identical value
	cmp   func(a, b T) int32
	unbox func(any) T
}

// newPriorityQueueT_ is new PriorityQueue<>(c) for the converting closure c of cmp and unbox:
// with a null cmp the closure is null and the queue orders naturally.
func newPriorityQueueT_[T any](cmp Comparator[T], unbox func(any) T) *PriorityQueue {
	if cmp == nil {
		return new_PriorityQueue(nil)
	}
	this := new_PriorityQueue(func(a, b any) int32 { return cmp(unbox(a), unbox(b)) })
	this.typed = &pqT_[T]{cmp: cmp, unbox: unbox}
	return this
}

// sameBox_ reports whether a and b are the identical interface value (type word and data word).
func sameBox_(a, b any) bool {
	return *(*[2]uintptr)(unsafe.Pointer(&a)) == *(*[2]uintptr)(unsafe.Pointer(&b))
}

// start makes ents as long as the items and returns the kept value of x, the element being
// placed, when the entry just past the items is x's (poll and removeAt take the last element
// off before they place it).
func (m *pqT_[T]) start(n int, x any) (tx T, have bool) {
	if len(m.ents) > n {
		if e := &m.ents[n]; e.x != nil && sameBox_(e.x, x) {
			tx, have = e.t, true
		}
		clear(m.ents[n:])
		m.ents = m.ents[:n]
	}
	for len(m.ents) < n {
		m.ents = append(m.ents, pqEnt_[T]{})
	}
	return tx, have
}

// val is unbox(es[k]), computed unless kept.
func (m *pqT_[T]) val(es []any, k int) T {
	e := &m.ents[k]
	if x := es[k]; e.x == nil || !sameBox_(e.x, x) {
		t := m.unbox(x)
		e.x, e.t = x, t
	}
	return e.t
}

func (m *pqT_[T]) put(k int, x any, tx T, have bool) {
	if have {
		m.ents[k] = pqEnt_[T]{x, tx}
	} else {
		m.ents[k] = pqEnt_[T]{}
	}
}

func (m *pqT_[T]) siftUp(es []any, k int, x any) {
	tx, have := m.start(len(es), x)
	for k > 0 {
		parent := (k - 1) >> 1
		if !have {
			tx, have = m.unbox(x), true
		}
		if m.cmp(tx, m.val(es, parent)) >= 0 {
			break
		}
		es[k] = es[parent]
		m.ents[k] = m.ents[parent]
		k = parent
	}
	es[k] = x
	m.put(k, x, tx, have)
}

func (m *pqT_[T]) siftDown(es []any, k int, x any) {
	tx, have := m.start(len(es), x)
	n := len(es)
	half := n >> 1
	for k < half {
		child := 2*k + 1
		if right := child + 1; right < n {
			tv := m.val(es, child)
			if m.cmp(tv, m.val(es, right)) > 0 {
				child = right
			}
		}
		if !have {
			tx, have = m.unbox(x), true
		}
		if m.cmp(tx, m.val(es, child)) <= 0 {
			break
		}
		es[k] = es[child]
		m.ents[k] = m.ents[child]
		k = child
	}
	es[k] = x
	m.put(k, x, tx, have)
}
