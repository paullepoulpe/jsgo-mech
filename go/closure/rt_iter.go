package closure

// Range-over-func iterators for translated for-each loops (post-flip step ranges).
//
// Each one walks its collection exactly as the cursor it replaces did (each, eachRev, eachOf_,
// eachRevOf_, foreach_): it reads the collection anew at every step, so what the loop body
// appends is visited and the loop ends early when the body shortens the collection, and it
// starts where the cursor started (a typed map at its first live entry). The cursor was made
// by the statement just before its loop and the range expression is evaluated at the same
// point, so a null collection throws NullPointerException where it did. break, continue,
// return and defer in a range-over-func body mean what they meant in the cursor loop.

import "iter"

// All yields the items of the list from the first, as each did.
func (l *TList[T]) All() iter.Seq[T] { return sliceAll_(&l.items) }

// AllRev yields the items of a deque used at its head (its head is the end), as eachRev did.
func (l *TList[T]) AllRev() iter.Seq[T] { return sliceAllRev_(&l.items) }

// sliceAll_ is eachOf_: the k-th step reads item k of the slice as it is then.
func sliceAll_[T any](s *[]T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for i := 0; i < len(*s); i++ {
			if !yield((*s)[i]) {
				return
			}
		}
	}
}

// sliceAllRev_ is eachRevOf_: the k-th step reads the k-th item from the end.
func sliceAllRev_[T any](s *[]T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for k := 0; ; k++ {
			s := *s
			if k >= len(s) || !yield(s[len(s)-1-k]) {
				return
			}
		}
	}
}

// All yields the entries of the map in insertion order, as each did.
func (m *TMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for i := m.first(); i < len(m.keys); i++ {
			if !m.isDead(i) && !yield(m.keys[i], m.vals[i]) {
				return
			}
		}
	}
}

// Keys is All without the values (a for-each over keySet()).
func (m *TMap[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		for i := m.first(); i < len(m.keys); i++ {
			if !m.isDead(i) && !yield(m.keys[i]) {
				return
			}
		}
	}
}

// Values is All without the keys (a for-each over values()).
func (m *TMap[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for i := m.first(); i < len(m.keys); i++ {
			if !m.isDead(i) && !yield(m.vals[i]) {
				return
			}
		}
	}
}

// All yields the elements of the set in insertion order, as each did.
func (s *TSet[K]) All() iter.Seq[K] { return s.TMap.Keys() }

// elems_ yields the elements of an erased Iterable as foreach_ did: one of the runtime's own
// collections in place, anything else through its iterator().
func elems_(c any) iter.Seq[any] {
	return func(yield func(any) bool) {
		for e := foreach_(c); e.next(); {
			if !yield(e.v) {
				return
			}
		}
	}
}

// elemsAs_ is elems_ with each element cast to T (a Java reference cast: null stays null), the
// cast the loop's first statement made.
func elemsAs_[T any](c any) iter.Seq[T] {
	return func(yield func(T) bool) {
		for e := foreach_(c); e.next(); {
			if !yield(cast_[T](e.v)) {
				return
			}
		}
	}
}

// elemsBy_ is elems_ with each element converted by f (str_, Node_of_, to_X, icast_X: the
// conversion the loop's first statement made).
func elemsBy_[T any](c any, f func(any) T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for e := foreach_(c); e.next(); {
			if !yield(f(e.v)) {
				return
			}
		}
	}
}

// first_ is c.iterator().next(): one of the runtime's own collections is read in place (an empty
// one throws NoSuchElementException, as its iterator's next does), anything else through
// c.iterator().next().
func first_(c any) any {
	e := foreach_(c)
	if e.it != nil {
		return e.it.next()
	}
	if !e.next() {
		panic(new_NoSuchElementException_0().self())
	}
	return e.v
}

// First is s.asSet().iterator().next() unboxed: the first element in insertion order.
func (s *TSet[K]) First() K { return s.keys[s.firstLive()] }

// FirstKey is m.asMap().keySet().iterator().next() unboxed.
func (m *TMap[K, V]) FirstKey() K { return m.keys[m.firstLive()] }

// FirstValue is m.asMap().values().iterator().next() unboxed.
func (m *TMap[K, V]) FirstValue() V { return m.vals[m.firstLive()] }

// firstLive is the position of the first entry; NoSuchElementException when there is none, as
// the views' iterators throw.
func (m *TMap[K, V]) firstLive() int {
	i := m.first()
	if i >= len(m.keys) {
		panic(new_NoSuchElementException_0().self())
	}
	return i
}
