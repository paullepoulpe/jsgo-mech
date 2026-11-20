package closure

// Rule "typed library interfaces" (tools/j2go, LibGen): a place whose Java type is a library
// collection interface with type arguments holds the typed interface, which is the erased one
// plus the methods that give or take elements at their Java type (suffix _t), as javac's
// class files have a method and its bridge:
//
//	type IteratorT[T any] interface { Iterator; next_t() T }
//
// So every value of a typed interface is also a value of the erased one, the same object, and
// goes to erased code as it is. The other way an erased value becomes a typed one by
// assertion where its object has the typed methods (a typed collection, a class of the
// program), else through a wrapper that converts the elements as the erased code cast them
// (xAs_); the wrapper never reaches erased code or Object: lgBox_ and X_un_ give the object it
// wraps. The translator names these types only with the rule on; they are declared here for
// both translations.

// lgWrap_ is a value whose identity for erased code is another object: a wrapper (the object
// it wraps) or a typed collection (its erased view or twin).
type lgWrap_ interface{ unwrap_() any }

// lgBox_ is a value of a typed interface as an Object.
func lgBox_(x any) any {
	if w, ok := x.(lgWrap_); ok {
		return w.unwrap_()
	}
	return x
}

// ---- Iterator, Iterable

type IteratorT[T any] interface {
	Iterator
	next_t() T
}

type IterableT[T any] interface {
	Iterable
	iterator_t() IteratorT[T]
}

func Iterator_un_(x Iterator) Iterator {
	if w, ok := x.(lgWrap_); ok {
		u, _ := w.unwrap_().(Iterator)
		return u
	}
	return x
}

func Iterable_un_(x Iterable) Iterable {
	if w, ok := x.(lgWrap_); ok {
		u, _ := w.unwrap_().(Iterable)
		return u
	}
	return x
}

type iteratorAs[T any] struct {
	Iterator
	c *tconv_[T]
}

func (w *iteratorAs[T]) next_t() T    { return w.c.unbox(w.Iterator.next()) }
func (w *iteratorAs[T]) unwrap_() any { return w.Iterator }

// iteratorAs_ is erased iterator v as an iterator of T: itself if it is one, else a wrapper
// that unboxes each element as the erased code did after next().
func iteratorAs_[T any](c *tconv_[T], v Iterator) IteratorT[T] {
	if v == nil {
		return nil
	}
	if t, ok := v.(IteratorT[T]); ok {
		return t
	}
	return &iteratorAs[T]{v, c}
}

type iterableAs[T any] struct {
	Iterable
	c *tconv_[T]
}

func (w *iterableAs[T]) iterator_t() IteratorT[T] { return iteratorAs_(w.c, w.Iterable.iterator()) }
func (w *iterableAs[T]) unwrap_() any             { return w.Iterable }

// iterableAs_ is erased v as an Iterable of T: itself if its object is one (the view of a
// typed collection, a class of the program), the typed list of an immutable list that has
// one, else a wrapper.
func iterableAs_[T any](c *tconv_[T], v Iterable) IterableT[T] {
	if v == nil {
		return nil
	}
	if t, ok := v.(IterableT[T]); ok {
		return t
	}
	if ic, ok := v.(*ImmutableCollection); ok {
		if l, ok := ic.typed_.(*TList[T]); ok {
			return l
		}
	}
	return &iterableAs[T]{v, c}
}

// The typed collections are Iterables of their elements themselves; for erased code they are
// their view (unwrap_).

func (l *TList[T]) asIterable() IterableT[T] {
	if l == nil {
		return nil
	}
	return l
}
func (l *TList[T]) unwrap_() any { return l.asList() }
func (l *TList[T]) iterator_t() IteratorT[T] {
	if l == nil {
		panic(new_NullPointerException_0().self())
	}
	return &tlistIter_[T]{l: l}
}
func (v *tlistView_[T]) iterator_t() IteratorT[T] { return &tlistIter_[T]{l: v.l} }

func (it *tlistIter_[T]) next_t() T {
	if it.i >= len(it.l.items) {
		panic(new_NoSuchElementException_0().self())
	}
	v := it.l.items[it.i]
	it.i++
	return v
}

func (d *TDeque[T]) asIterable() IterableT[T] {
	if d == nil {
		return nil
	}
	return d
}
func (d *TDeque[T]) unwrap_() any { return d.asDeque() }

func (s *TSet[K]) asIterable() IterableT[K] {
	if s == nil {
		return nil
	}
	return s
}
func (s *TSet[K]) unwrap_() any { return s.asSet() }
func (s *TSet[K]) iterator_t() IteratorT[K] {
	if s == nil {
		panic(new_NullPointerException_0().self())
	}
	return &tsetIter_[K]{s: s, last: -1}
}
func (v *tsetView_[K]) iterator_t() IteratorT[K] { return &tsetIter_[K]{s: v.s, last: -1} }

func (it *tsetIter_[K]) next_t() K {
	if !it.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	it.last = it.i
	it.i++
	return it.s.keys[it.last]
}

// stepperT_ is a typed collection that a for-each cursor reads by position, as stepper_ is
// for erased code: the element at or after position i and the position to go on from.
type stepperT_[T any] interface {
	step_t(i int) (v T, next int, ok bool)
}

func (s *TSet[K]) step_t(i int) (v K, next int, ok bool) {
	for ; i < len(s.keys); i++ {
		if !s.dead(i) {
			return s.keys[i], i + 1, true
		}
	}
	return v, i, false
}

func (v *tsetView_[K]) step_t(i int) (K, int, bool) { return v.s.step_t(i) }

// iterEach_ is the cursor of a for-each loop over an Iterable of T. Over a typed list it reads
// the slice in place as sliceEach_ does; over a wrapped erased collection it is the erased
// loop's cursor (foreach_: in place for the runtime's own collections) and unboxes each
// element; over anything else it holds the iterator.
type iterEach_[T any] struct {
	src *[]T
	at  stepperT_[T] // a typed collection read by position (a typed set)
	it  IteratorT[T]
	c   *tconv_[T] // set: the cursor e of the wrapped erased collection is read
	e   each_
	i   int
	v   T
}

func eachOf_[T any](x IterableT[T]) iterEach_[T] {
	switch l := x.(type) {
	case nil:
		panic(new_NullPointerException_0().self())
	case *TList[T]:
		return iterEach_[T]{src: &l.items}
	case *tlistView_[T]:
		return iterEach_[T]{src: &l.l.items}
	case *TDeque[T]:
		return iterEach_[T]{src: &l.items}
	case stepperT_[T]:
		return iterEach_[T]{at: l}
	case *iterableAs[T]:
		return iterEach_[T]{c: l.c, e: foreach_(l.Iterable)}
	case *collectionAs[T]:
		return iterEach_[T]{c: l.c, e: foreach_(l.Collection)}
	case *listAs[T]:
		return iterEach_[T]{c: l.c, e: foreach_(l.List)}
	}
	return iterEach_[T]{it: x.iterator_t()}
}

func (e *iterEach_[T]) next() bool {
	if e.src != nil {
		if e.i < len(*e.src) {
			e.v = (*e.src)[e.i]
			e.i++
			return true
		}
		return false
	}
	if e.at != nil {
		var ok bool
		e.v, e.i, ok = e.at.step_t(e.i)
		return ok
	}
	if e.c != nil {
		if e.e.next() {
			e.v = e.c.unbox(e.e.v)
			return true
		}
		return false
	}
	if e.it.hasNext() {
		e.v = e.it.next_t()
		return true
	}
	return false
}

// ---- Views of a typed map (piece `views`): keySet() and values() of the erased view of a
// *TMap[K, V] are native: an IterableT of the keys / of the values, read without a wrapper and
// without a box (they are the map itself under another type, so they cost no object either).

// tmapKeyIter_ / tmapValIter_ are the iterators of those views at the element type; next() and
// remove() are the erased iterator's.
type tmapKeyIter_[K comparable, V any] struct{ tmapIter_[K, V] }

func (it *tmapKeyIter_[K, V]) next_t() K {
	if !it.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	it.last = it.i
	it.i++
	return it.m.keys[it.last]
}

type tmapValIter_[K comparable, V any] struct{ tmapIter_[K, V] }

func (it *tmapValIter_[K, V]) next_t() V {
	if !it.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	it.last = it.i
	it.i++
	return it.m.vals[it.last]
}

func (v *tmapKeySet_[K, V]) iterator_t() IteratorT[K] {
	return &tmapKeyIter_[K, V]{tmapIter_[K, V]{m: v.m, last: -1, mode: tkeys_}}
}

func (v *tmapKeySet_[K, V]) step_t(i int) (k K, next int, ok bool) {
	m := v.m
	for ; i < len(m.keys); i++ {
		if !m.dead(i) {
			return m.keys[i], i + 1, true
		}
	}
	return k, i, false
}

func (v *tmapValues_[K, V]) iterator_t() IteratorT[V] {
	return &tmapValIter_[K, V]{tmapIter_[K, V]{m: v.m, last: -1, mode: tvals_}}
}

func (v *tmapValues_[K, V]) step_t(i int) (x V, next int, ok bool) {
	m := v.m
	for ; i < len(m.keys); i++ {
		if !m.dead(i) {
			return m.vals[i], i + 1, true
		}
	}
	return x, i, false
}

// ---- Kind `coll`: java.util.Collection<T>. A typed Collection is the erased one that is also
// a typed Iterable (the methods that take elements are the erased ones).

type CollectionT[T any] interface {
	Collection
	iterator_t() IteratorT[T]
}

// Collection_un_ is typed value x as erased code knows it.
func Collection_un_(x Collection) Collection {
	if w, ok := x.(lgWrap_); ok {
		u, _ := w.unwrap_().(Collection)
		return u
	}
	return x
}

type collectionAs[T any] struct {
	Collection
	c *tconv_[T]
}

func (w *collectionAs[T]) iterator_t() IteratorT[T] { return iteratorAs_(w.c, w.Collection.iterator()) }
func (w *collectionAs[T]) unwrap_() any             { return w.Collection }

// collectionAs_ is erased v as a Collection of T: v itself where its object is native (the
// view of a typed collection, a view of a typed map), the view of the typed collection of an
// erased immutable collection, else a wrapper.
func collectionAs_[T any](c *tconv_[T], v Collection) CollectionT[T] {
	if v == nil {
		return nil
	}
	if t, ok := v.(CollectionT[T]); ok {
		return t
	}
	if ic, ok := v.(*ImmutableCollection); ok {
		if l, ok := ic.typed_.(*TList[T]); ok {
			return &l.view
		}
	}
	return &collectionAs[T]{v, c}
}

// asCollT is a typed list, set or deque as the Collection of its elements: its view, which is
// part of it (nil for null). Erased code and Object get what asList() / asSet() / asDeque()
// give (unwrap_: the twin of an immutable collection).
func (l *TList[T]) asCollT() CollectionT[T] {
	if l == nil {
		return nil
	}
	return &l.view
}
func (v *tlistView_[T]) unwrap_() any { return v.l.asList() }

func (s *TSet[K]) asCollT() CollectionT[K] {
	if s == nil {
		return nil
	}
	return &s.view
}
func (v *tsetView_[K]) unwrap_() any { return v.s.asSet() }

func (d *TDeque[T]) asCollT() CollectionT[T] {
	if d == nil {
		return nil
	}
	return &d.dview
}
func (v *tdequeView_[T]) unwrap_() any { return v }

// valuesT is values() of a typed map for typed code: the Collection of its values, which is the
// map itself under another type (no object, no box; live, as Java's view). It serves an
// immutable map too (every change throws); erased code and Object get what the erased map's
// values() gives (unwrap_), so the view of a map that can change is one object on both sides.
// A null map throws NullPointerException.
func (m *TMap[K, V]) valuesT() CollectionT[V] {
	_ = m.live
	return (*tmapValues_[K, V])(&m.view)
}
func (v *tmapValues_[K, V]) unwrap_() any { return v.m.asMap().values() }

// ---- Kind `list`: java.util.List<T>. A typed List is the erased one, a typed Iterable, and
// gives its elements at their type (get_t).

type ListT[T any] interface {
	List
	iterator_t() IteratorT[T]
	get_t(i int32) T
}

// List_un_ is typed value x as erased code knows it.
func List_un_(x List) List {
	if w, ok := x.(lgWrap_); ok {
		u, _ := w.unwrap_().(List)
		return u
	}
	return x
}

type listAs[T any] struct {
	List
	c *tconv_[T]
}

func (w *listAs[T]) iterator_t() IteratorT[T] { return iteratorAs_(w.c, w.List.iterator()) }
func (w *listAs[T]) get_t(i int32) T          { return w.c.unbox(w.List.get(i)) }
func (w *listAs[T]) unwrap_() any             { return w.List }

// listAs_ is erased v as a List of T: v itself where its object is native (the view of a typed
// list), the view of the typed list of an erased immutable list, else a wrapper.
func listAs_[T any](c *tconv_[T], v List) ListT[T] {
	if v == nil {
		return nil
	}
	if t, ok := v.(ListT[T]); ok {
		return t
	}
	if ic, ok := v.(*ImmutableCollection); ok {
		if l, ok := ic.typed_.(*TList[T]); ok {
			return &l.view
		}
	}
	return &listAs[T]{v, c}
}

// asListT is a typed list as the List of its elements for typed code: its view, which is part
// of it (nil for null).
func (l *TList[T]) asListT() ListT[T] {
	if l == nil {
		return nil
	}
	return &l.view
}

func (v *tlistView_[T]) get_t(i int32) T { return v.l.get(i) }
