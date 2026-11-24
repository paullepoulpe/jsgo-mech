package closure

import "sync/atomic"

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
//
// From post-flip step 402 on (one typed collection family) this file is written for the
// post-flip typed collections (rt_tlist.go, rt_tmap.go): a typed list, set or map is, for typed
// code, its erased view object (tlistView_, tsetView_ / tsortedSetView_, tmapView_ /
// tsortedMapView_; of an immutable collection too, where erased code gets its twin instead),
// and the views of a map are tkeysView_, tvalsView_ and tentriesView_. Since steps 554b and
// 556 each of them is the collection itself under the view's type.

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
		if l, ok := ic.typed_.v.(IterableT[T]); ok {
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

// tview_ is the view of the list that typed code holds as a List or Collection: the one erased
// code sees too, unless the list is an ImmutableList (erased code sees its twin, asList).
func (l *TList[T]) tview_() *tlistView_[T] { return (*tlistView_[T])(l) }
func (l *TList[T]) iterator_t() IteratorT[T] {
	if l == nil {
		panic(new_NullPointerException_0().self())
	}
	return &tlistIter_[T]{l: l}
}
func (v *tlistView_[T]) iterator_t() IteratorT[T] { return &tlistIter_[T]{l: v.tl()} }

func (it *tlistIter_[T]) next_t() T { return it.nextT() }

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
	return &tmapKeyIter_[K, struct{}]{tmapIter_[K, struct{}]{m: &s.TMap, i: s.first(), out: keyOut_[K, struct{}]}}
}
func (v *tsetView_[K]) iterator_t() IteratorT[K] {
	return &tmapKeyIter_[K, struct{}]{tmapIter_[K, struct{}]{m: v.tm(), i: v.tm().first(), out: keyOut_[K, struct{}]}}
}

// stepperT_ is a typed collection that a for-each cursor reads by position, as stepper_ is
// for erased code: the element at or after position i and the position to go on from.
type stepperT_[T any] interface {
	step_t(i int) (v T, next int, ok bool)
}

func (s *TSet[K]) step_t(i int) (v K, next int, ok bool) { return keyStep_(&s.TMap, i) }

func (v *tsetView_[K]) step_t(i int) (K, int, bool) { return keyStep_(v.tm(), i) }

// keyStep_ is step_t over the keys of a typed map or set: the live position at or after i.
func keyStep_[K comparable, V any](m *TMap[K, V], i int) (k K, next int, ok bool) {
	if i == 0 {
		i = m.first()
	}
	for ; i < len(m.keys); i++ {
		if !m.isDead(i) {
			return m.keys[i], i + 1, true
		}
	}
	return k, i, false
}

// iterEach_ is the cursor of a for-each loop over an Iterable of T. Over a typed list it reads
// the slice in place as jsliceEach_ does; over a wrapped erased collection it is the erased
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

func jeachOf_[T any](x IterableT[T]) iterEach_[T] {
	switch l := x.(type) {
	case nil:
		panic(new_NullPointerException_0().self())
	case *TList[T]:
		return iterEach_[T]{src: &l.items}
	case *tlistView_[T]:
		return iterEach_[T]{src: &l.tl().items}
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
	case *setAs[T]:
		return iterEach_[T]{c: l.c, e: foreach_(l.Set)}
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
// *JTMap[K, V] are native: an IterableT of the keys / of the values, read without a wrapper and
// without a box (they are the map itself under another type, so they cost no object either).

// tmapKeyIter_ / tmapValIter_ are the iterators of those views at the element type; next() and
// remove() are the erased iterator's.
type tmapKeyIter_[K comparable, V any] struct{ tmapIter_[K, V] }

func (it *tmapKeyIter_[K, V]) next_t() K { return it.nextKey() }

type tmapValIter_[K comparable, V any] struct{ tmapIter_[K, V] }

func (it *tmapValIter_[K, V]) next_t() V {
	if !it.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	it.last = int32(it.i)
	it.i++
	return it.m.vals[it.last]
}

func (v *tkeysView_[K, V]) iterator_t() IteratorT[K] {
	return &tmapKeyIter_[K, V]{tmapIter_[K, V]{m: v.tm(), i: v.tm().first(), out: keyOut_[K, V]}}
}

func (v *tkeysView_[K, V]) step_t(i int) (k K, next int, ok bool) { return keyStep_(v.tm(), i) }

func (v *tvalsView_[K, V]) iterator_t() IteratorT[V] {
	return &tmapValIter_[K, V]{tmapIter_[K, V]{m: v.tm(), i: v.tm().first(), out: valOut_[K, V]}}
}

func (v *tvalsView_[K, V]) step_t(i int) (x V, next int, ok bool) {
	m := v.tm()
	if i == 0 {
		i = m.first()
	}
	for ; i < len(m.keys); i++ {
		if !m.isDead(i) {
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
	add_t(x T) bool
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
func (w *collectionAs[T]) add_t(x T) bool           { return w.Collection.add(w.c.box(x)) }
func (w *collectionAs[T]) unwrap_() any             { return w.Collection }

// asCollT_ is what a typed list or set has for typed code: itself as the Collection of its elements.
type asCollT_[T any] interface{ asCollT() CollectionT[T] }

// collectionAs_ is erased v as a Collection of T: v itself where its object is native (the
// view of a typed collection, a view of a typed map), the view of the typed collection of an
// erased immutable list or set, else a wrapper.
func collectionAs_[T any](c *tconv_[T], v Collection) CollectionT[T] {
	if v == nil {
		return nil
	}
	if t, ok := v.(CollectionT[T]); ok {
		return t
	}
	if ic, ok := v.(*ImmutableCollection); ok {
		if l, ok := ic.typed_.v.(asCollT_[T]); ok {
			return l.asCollT()
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
	return l.tview_()
}
func (v *tlistView_[T]) unwrap_() any { return v.tl().asList() }

func (s *TSet[K]) asCollT() CollectionT[K] {
	if s == nil {
		return nil
	}
	return s.asSetT()
}
func (v *tsetView_[K]) unwrap_() any { return v.ts().asSet() }

func (d *TDeque[T]) asCollT() CollectionT[T] {
	if d == nil {
		return nil
	}
	d.asDeque()
	return d.dview
}
func (v *tdequeFace_[T]) unwrap_() any { return v }

// valuesT is values() of a typed map for typed code: the Collection of its values, which is the
// map itself under another type (no object, no box; live, as Java's view). It serves an
// immutable map too (every change throws); erased code and Object get what the erased map's
// values() gives (unwrap_), so the view of a map that can change is one object on both sides.
// A null map throws NullPointerException.
func (m *TMap[K, V]) valuesT() CollectionT[V] {
	_ = m.live
	return (*tvalsView_[K, V])(m)
}
func (v *tvalsView_[K, V]) unwrap_() any {
	if v.tm().imm {
		return v.tm().asMap().values()
	}
	return v
}

// ---- Kind `list`: java.util.List<T>. A typed List is the erased one, a typed Iterable, and
// gives its elements at their type (get_t).

type ListT[T any] interface {
	List
	iterator_t() IteratorT[T]
	add_t(x T) bool
	get_t(i int32) T
	set_t(i int32, x T) T
	remove_t(i int32) T
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

// (the methods that take an element call the erased object through the interface of the one
// method: the erased interfaces of a translation have only the methods some call names)
func (w *listAs[T]) add_t(x T) bool { return w.List.add(w.c.box(x)) }
func (w *listAs[T]) set_t(i int32, x T) T {
	return w.c.unbox(w.List.(interface{ set(int32, any) any }).set(i, w.c.box(x)))
}
func (w *listAs[T]) remove_t(i int32) T {
	return w.c.unbox(w.List.(interface{ remove(int32) any }).remove(i))
}
func (w *listAs[T]) unwrap_() any { return w.List }

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
		if l, ok := ic.typed_.v.(*TList[T]); ok {
			return l.tview_()
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
	return l.tview_()
}

func (v *tlistView_[T]) get_t(i int32) T      { return v.tl().get(i) }
func (v *tlistView_[T]) add_t(x T) bool       { return v.tl().add(x) }
func (v *tlistView_[T]) set_t(i int32, x T) T { return v.tl().set(i, x) }
func (v *tlistView_[T]) remove_t(i int32) T   { return v.tl().removeAt(i) }
func (v *tsetView_[K]) add_t(x K) bool        { return v.ts().Add(x) }

// (the views of a map take no element, as in Java)
func (v *tkeysView_[K, V]) add_t(x K) bool                   { return v.add(nil) }
func (v *tvalsView_[K, V]) add_t(x V) bool                   { return v.add(nil) }
func (v *tentriesView_[K, V]) add_t(x Map_EntryT[K, V]) bool { return v.add(nil) }

// ---- Kind `set`: java.util.Set<T>. A typed Set is the erased one that is also a typed
// Iterable (the methods that take elements are the erased ones).

type SetT[T any] interface {
	Set
	iterator_t() IteratorT[T]
	add_t(x T) bool
}

// Set_un_ is typed value x as erased code knows it.
func Set_un_(x Set) Set {
	if w, ok := x.(lgWrap_); ok {
		u, _ := w.unwrap_().(Set)
		return u
	}
	return x
}

type setAs[T any] struct {
	Set
	c *tconv_[T]
}

func (w *setAs[T]) iterator_t() IteratorT[T] { return iteratorAs_(w.c, w.Set.iterator()) }
func (w *setAs[T]) add_t(x T) bool           { return w.Set.add(w.c.box(x)) }
func (w *setAs[T]) unwrap_() any             { return w.Set }

// asSetT_ is what a typed set has for typed code: itself as the Set of its elements.
type asSetT_[T any] interface{ asSetT() SetT[T] }

// setAs_ is erased v as a Set of T: v itself where its object is native (the view of a typed
// set, the key set of a typed map), the view of the typed set of an erased immutable set, else
// a wrapper.
func setAs_[T any](c *tconv_[T], v Set) SetT[T] {
	if v == nil {
		return nil
	}
	if t, ok := v.(SetT[T]); ok {
		return t
	}
	if ic, ok := v.(*ImmutableCollection); ok {
		if s, ok := ic.typed_.v.(asSetT_[T]); ok {
			return s.asSetT()
		}
	}
	return &setAs[T]{v, c}
}

// asSetT is a typed set as the Set of its elements for typed code: its view, which is part of
// it (nil for null).
func (s *TSet[K]) asSetT() SetT[K] {
	if s == nil {
		return nil
	}
	if s.x != nil && s.x.sort != nil {
		return s.sortedView()
	}
	return (*tsetView_[K])(s)
}

// keySetT is keySet() of a typed map for typed code: the Set of its keys, which is the map
// itself under another type (no object, no box; live). As valuesT, it serves an immutable map
// too, and erased code and Object get what the erased map's keySet() gives (unwrap_).
func (m *TMap[K, V]) keySetT() SetT[K] {
	_ = m.live
	return (*tkeysView_[K, V])(m)
}
func (v *tkeysView_[K, V]) unwrap_() any {
	if v.tm().imm {
		return v.tm().asMap().keySet()
	}
	return v
}

// ---- Kind `map`: java.util.Map<K, V> and Map.Entry<K, V>. A typed Map is the erased one that
// gives its values, its views and its entries at their types.

type MapT[K any, V any] interface {
	Map
	get_t(k any) V
	put_t(k K, x V) V
	remove_t(k any) V
	keySet_t() SetT[K]
	values_t() CollectionT[V]
	entrySet_t() SetT[Map_EntryT[K, V]]
}

type Map_EntryT[K any, V any] interface {
	Map_Entry
	getKey_t() K
	getValue_t() V
	setValue_t(x V) V
}

// Map_un_ / Map_Entry_un_ are typed value x as erased code knows it.
func Map_un_(x Map) Map {
	if w, ok := x.(lgWrap_); ok {
		u, _ := w.unwrap_().(Map)
		return u
	}
	return x
}

func Map_Entry_un_(x Map_Entry) Map_Entry {
	if w, ok := x.(lgWrap_); ok {
		u, _ := w.unwrap_().(Map_Entry)
		return u
	}
	return x
}

type mapAs[K any, V any] struct {
	Map
	kc     *tconv_[K]
	vc     *tconv_[V]
	ks, vs atomic.Pointer[any] // the typed key set and values, once asked for (the views are live)
}

func (w *mapAs[K, V]) get_t(k any) V { return w.vc.unbox(w.Map.get(k)) }
func (w *mapAs[K, V]) put_t(k K, x V) V {
	return w.vc.unbox(w.Map.(interface{ put(any, any) any }).put(w.kc.box(k), w.vc.box(x)))
}
func (w *mapAs[K, V]) remove_t(k any) V {
	return w.vc.unbox(w.Map.(interface{ remove(any) any }).remove(k))
}
func (w *mapAs[K, V]) keySet_t() SetT[K] {
	if p := w.ks.Load(); p != nil {
		return (*p).(SetT[K])
	}
	var s any = setAs_(w.kc, w.Map.keySet())
	w.ks.Store(&s)
	return s.(SetT[K])
}
func (w *mapAs[K, V]) values_t() CollectionT[V] {
	if p := w.vs.Load(); p != nil {
		return (*p).(CollectionT[V])
	}
	var s any = collectionAs_(w.vc, w.Map.values())
	w.vs.Store(&s)
	return s.(CollectionT[V])
}
func (w *mapAs[K, V]) entrySet_t() SetT[Map_EntryT[K, V]] {
	return setAs_(entryConv_(w.kc, w.vc), w.Map.entrySet())
}
func (w *mapAs[K, V]) unwrap_() any { return w.Map }

// entryConv_ is the conversion of the entries of an erased map to typed entries and back.
func entryConv_[K any, V any](kc *tconv_[K], vc *tconv_[V]) *tconv_[Map_EntryT[K, V]] {
	return &tconv_[Map_EntryT[K, V]]{
		box: func(e Map_EntryT[K, V]) any { return lgBox_(e) },
		unbox: func(x any) Map_EntryT[K, V] {
			if x == nil {
				return nil
			}
			return map_EntryAs_(kc, vc, x.(Map_Entry))
		},
	}
}

// asMapT_ is what a typed map has for typed code: itself as the Map of its keys and values.
type asMapT_[K any, V any] interface{ asMapT() MapT[K, V] }

// mapAs_ is erased v as a Map of K to V: v itself where its object is native (the view of a
// typed map), the view of the typed map of an erased immutable map, else a wrapper.
func mapAs_[K any, V any](kc *tconv_[K], vc *tconv_[V], v Map) MapT[K, V] {
	if v == nil {
		return nil
	}
	if t, ok := v.(MapT[K, V]); ok {
		return t
	}
	if im, ok := v.(*ErasedImmutableMap); ok {
		if m, ok := im.typed_.v.(asMapT_[K, V]); ok {
			return m.asMapT()
		}
	}
	if lm, ok := v.(*LinkedHashMap); ok {
		// an erased LinkedHashMap keeps the wrapper made for it: a map that erased code made
		// (for a type variable) and that is read at a typed place on every lookup
		if p := lm.jtyped_.Load(); p != nil {
			if w, ok := (*p).(*mapAs[K, V]); ok && w.kc == kc && w.vc == vc && w.Map == v {
				return w
			}
		}
		var w any = &mapAs[K, V]{Map: v, kc: kc, vc: vc}
		lm.jtyped_.Store(&w)
		return w.(*mapAs[K, V])
	}
	return &mapAs[K, V]{Map: v, kc: kc, vc: vc}
}

type map_EntryAs[K any, V any] struct {
	Map_Entry
	kc *tconv_[K]
	vc *tconv_[V]
}

func (w *map_EntryAs[K, V]) getKey_t() K   { return w.kc.unbox(w.Map_Entry.getKey()) }
func (w *map_EntryAs[K, V]) getValue_t() V { return w.vc.unbox(w.Map_Entry.getValue()) }
func (w *map_EntryAs[K, V]) setValue_t(x V) V {
	return w.vc.unbox(w.Map_Entry.(interface{ setValue(any) any }).setValue(w.vc.box(x)))
}
func (w *map_EntryAs[K, V]) unwrap_() any { return w.Map_Entry }

// map_EntryAs_ is erased v as an entry of K and V: v itself where its object is native (an
// entry of a typed map), else a wrapper.
func map_EntryAs_[K any, V any](kc *tconv_[K], vc *tconv_[V], v Map_Entry) Map_EntryT[K, V] {
	if v == nil {
		return nil
	}
	if t, ok := v.(Map_EntryT[K, V]); ok {
		return t
	}
	return &map_EntryAs[K, V]{v, kc, vc}
}

// asMapT is a typed map as the Map of its keys and values for typed code: its view, which is
// part of it (nil for null; also of an immutable or sorted map, whose changes throw as the
// map's do). Erased code and Object get what asMap() gives (unwrap_).
func (m *TMap[K, V]) asMapT() MapT[K, V] {
	if m == nil {
		return nil
	}
	if m.x != nil && m.x.sort != nil {
		return m.sortedView()
	}
	return (*tmapView_[K, V])(m)
}
func (v *tmapView_[K, V]) unwrap_() any { return v.tm().asMap() }

func (v *tmapView_[K, V]) get_t(x any) V {
	if k, ok := v.tm().kc.try(x); ok {
		if i := v.tm().find(k); i >= 0 {
			return v.tm().vals[i]
		}
	}
	return v.tm().vc.null
}
func (v *tmapView_[K, V]) put_t(k K, x V) V { return v.tm().Put(k, x) }
func (v *tmapView_[K, V]) remove_t(x any) V {
	if v.tm().imm {
		throwUOE_()
	}
	if k, ok := v.tm().kc.try(x); ok {
		if i := v.tm().find(k); i >= 0 {
			return v.tm().removeAt(i)
		}
	}
	return v.tm().vc.null
}
func (v *tmapView_[K, V]) keySet_t() SetT[K]        { return (*tkeysView_[K, V])(v.tm()) }
func (v *tmapView_[K, V]) values_t() CollectionT[V] { return (*tvalsView_[K, V])(v.tm()) }
func (v *tmapView_[K, V]) entrySet_t() SetT[Map_EntryT[K, V]] {
	return (*tentriesView_[K, V])(v.tm())
}

// (a sorted map casts the key, as its erased get and remove do)
func (v *tsortedMapView_[K, V]) get_t(x any) V {
	if i := v.tm().find(v.tm().kc.unbox(x)); i >= 0 {
		return v.tm().vals[i]
	}
	return v.tm().vc.null
}
func (v *tsortedMapView_[K, V]) remove_t(x any) V { return v.tm().Delete(v.tm().kc.unbox(x)) }

// entrySetT is entrySet() of a typed map for typed code (as keySetT, valuesT).
func (m *TMap[K, V]) entrySetT() SetT[Map_EntryT[K, V]] {
	_ = m.live
	return (*tentriesView_[K, V])(m)
}
func (v *tentriesView_[K, V]) unwrap_() any {
	if v.tm().imm {
		return v.tm().asMap().entrySet()
	}
	return v
}

// The entry set of a typed map gives its entries as typed entries: the objects the erased
// iterator gives (one per entry, as each entry of the Java map is an object).
type tmapEntryIter_[K comparable, V any] struct{ tmapIter_[K, V] }

func (it *tmapEntryIter_[K, V]) next_t() Map_EntryT[K, V] {
	if !it.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	it.last = int32(it.i)
	it.i++
	return it.m.entryAt(int(it.last))
}

func (m *TMap[K, V]) entryAt(i int) *tentryView_[K, V] {
	v := &tentryView_[K, V]{own: TEntry[K, V]{m: m, i: int32(i), k: m.keys[i], v: m.vals[i]}}
	v.e = &v.own
	return v
}

func (v *tentriesView_[K, V]) iterator_t() IteratorT[Map_EntryT[K, V]] {
	return &tmapEntryIter_[K, V]{tmapIter_[K, V]{m: v.tm(), i: v.tm().first(), out: entryOut_[K, V]}}
}

func (v *tentriesView_[K, V]) step_t(i int) (e Map_EntryT[K, V], next int, ok bool) {
	m := v.tm()
	if i == 0 {
		i = m.first()
	}
	for ; i < len(m.keys); i++ {
		if !m.isDead(i) {
			return m.entryAt(i), i + 1, true
		}
	}
	return nil, i, false
}

func (v *tentryView_[K, V]) getKey_t() K      { return v.e.k }
func (v *tentryView_[K, V]) getValue_t() V    { return v.e.getValue() }
func (v *tentryView_[K, V]) setValue_t(x V) V { return v.e.setValue(x) }

// asEntryT is an entry of a typed map (the variable of a loop over its entries) as a typed
// entry: its erased view, which has the typed methods.
func (e *TEntry[K, V]) asEntryT() Map_EntryT[K, V] {
	if e == nil {
		return nil
	}
	return &tentryView_[K, V]{e: e}
}

// ---- Piece `cls`: a place of a collection class (ImmutableList<X>) holds the typed interface.

// tlistOfT_ is the value of a place of class ImmutableList / ImmutableCollection entering a
// declaration of rule "typed collections": the immutable typed list whose view it is, else
// tlistImm_ of the object erased code knows (which fails as a cast to the class does).
func tlistOfT_[T any](c *tconv_[T], x Collection) *TList[T] {
	if v, ok := x.(*tlistView_[T]); ok && v.tl().imm && !v.tl().unmod {
		return v.tl()
	}
	return tlistImm_(c, cast_[*ImmutableCollection](lgBox_(x)))
}

// tsetOfT_ is tlistOfT_ for a set.
func tsetOfT_[K comparable](c *tconv_[K], x Collection) *TSet[K] {
	switch v := x.(type) {
	case *tsetView_[K]:
		if v.ts().imm {
			return v.ts()
		}
	case *tsortedSetView_[K]:
		if v.ts().imm {
			return v.ts()
		}
	}
	return tsetImm_(c, cast_[*ImmutableCollection](lgBox_(x)))
}

// tmapOfT_ is tlistOfT_ for a map.
func tmapOfT_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], x Map) *TMap[K, V] {
	switch v := x.(type) {
	case *tmapView_[K, V]:
		if v.tm().imm {
			return v.tm()
		}
	case *tsortedMapView_[K, V]:
		if v.tm().imm {
			return v.tm()
		}
	}
	return tmapImm_(kc, vc, cast_[*ErasedImmutableMap](lgBox_(x)))
}
