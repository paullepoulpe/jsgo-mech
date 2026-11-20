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
		e := foreach_(c)
		if s := e.src; s != nil {
			// a slice (ArrayList and the like): the cursor's own steps, written out here so that
			// the loop is over the slice (its length read anew at each step, as each_.next reads it)
			for i := 0; i < len(*s); i++ {
				if !yield((*s)[i]) {
					return
				}
			}
			return
		}
		for e.next() {
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

// immElems_ is the elements of an immutable collection, for a for-each loop over it (post-flip
// step ranges -index): a range over them visits what foreach_'s cursor visited, since the
// collection never changes (a multiset's elements with their repetitions, as its iterator
// gives them). A null collection throws NullPointerException as foreach_(nil) did.
func immElems_(c *ImmutableCollection) []any {
	if c == nil {
		panic(new_NullPointerException_0().self())
	}
	return c.elems_()
}

// immCursor_ is an explicit iterator over an immutable collection (post-flip step iters): the
// translated code's hasNext/next/remove on it mean what they meant on the collection's own
// iterator (ImmutableCollection.iterator, a sliceIter_ over its elements), but it is a typed
// pointer, so a function that only advances it keeps it on the stack.
type immCursor_ struct {
	items []any
	i     int
}

// immCursorOf_ is c.iterator(): the elements in iteration order (a multiset's with their
// repetitions); a null c throws NullPointerException as calling c.iterator() did.
func immCursorOf_(c *ImmutableCollection) *immCursor_ { return &immCursor_{items: c.elems_()} }

func (it *immCursor_) hasNext() bool { return it.i < len(it.items) }

func (it *immCursor_) next() any {
	if it.i >= len(it.items) {
		panic(new_NoSuchElementException_0().self())
	}
	v := it.items[it.i]
	it.i++
	return v
}

// remove throws UnsupportedOperationException, as an immutable collection's iterator does.
func (it *immCursor_) remove() { throwUOE_() }

// immCursorT_ is immCursor_ over a typed immutable list (post-flip step tviews):
// immCursorTOf_(l) is immCursorOf_(l.asImm()), whose twin holds l's elements boxed in the same
// order; next() gives the element itself instead of its box. A null l fails as l.asImm() of a
// null list followed by iterator() did (a nil dereference).
type immCursorT_[T any] struct {
	items []T
	i     int
}

func immCursorTOf_[T any](l *TList[T]) *immCursorT_[T] { return &immCursorT_[T]{items: l.items} }

func (it *immCursorT_[T]) hasNext() bool { return it.i < len(it.items) }

func (it *immCursorT_[T]) next() T {
	if it.i >= len(it.items) {
		panic(new_NoSuchElementException_0().self())
	}
	v := it.items[it.i]
	it.i++
	return v
}

// remove throws UnsupportedOperationException, as an immutable collection's iterator does.
func (it *immCursorT_[T]) remove() { throwUOE_() }

// slicePtrElemsBy_ is elemsBy_ over the list p points to (a pointer to a slice seen as an
// erased List): each element boxed, then converted by f, reading the slice anew at every
// step as the list's iterator did.
func slicePtrElemsBy_[T, U any](c *tconv_[T], p *[]T, f func(any) U) iter.Seq[U] {
	return func(yield func(U) bool) {
		for i := 0; i < len(*p); i++ {
			if !yield(f(c.box((*p)[i]))) {
				return
			}
		}
	}
}
