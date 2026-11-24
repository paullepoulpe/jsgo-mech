//go:build !j2go_nofntypes

package closure

import "sort"

// Rule "typed library interfaces", piece `fns` (tools/j2go, LibFns; J2GO_NOLIBGEN=fns is the
// translation without it): the runtime's static library functions and the default methods of
// the library interfaces at their Java generic signatures, as Go generic functions whose
// collections are the typed interfaces of rt_lg.go and whose functions are the func types of
// rule "function types" (so this file is built with that rule only). The translator calls one
// (LibFns.TYPED, a name list) where every collection argument enters at its typed interface
// as it is; any other call keeps the erased function next to it, which this file mirrors step
// by step (the same calls on the collection, in the same order), without the boxes.

// ---- Iterables

// Iterables_getFirstT_ is Iterables.getFirst (Iterables_getFirst).
func Iterables_getFirstT_[T any](p0 IterableT[T], p1 T) T {
	if it := p0.iterator_t(); it.hasNext() {
		return it.next_t()
	}
	return p1
}

// Iterables_getLastT_ is Iterables.getLast (Iterables_getLast).
func Iterables_getLastT_[T any](p0 IterableT[T]) T {
	if l, ok := p0.(ListT[T]); ok {
		if l.isEmpty() {
			panic(new_NoSuchElementException_0().self())
		}
		return l.get_t(l.size() - 1)
	}
	it := p0.iterator_t()
	if !it.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	var v T
	for it.hasNext() {
		v = it.next_t()
	}
	return v
}

// Iterables_getOnlyElementT_ is Iterables.getOnlyElement (Iterables_getOnlyElement).
func Iterables_getOnlyElementT_[T any](p0 IterableT[T]) T {
	it := p0.iterator_t()
	if !it.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	first := it.next_t()
	if !it.hasNext() {
		return first
	}
	msg := "expected one element but was: <" + objstr_(boxT_(first))
	for i := 0; i < 4 && it.hasNext(); i++ {
		msg += ", " + objstr_(boxT_(it.next_t()))
	}
	if it.hasNext() {
		msg += ", ..."
	}
	throwIAE_(msg + ">")
	return first
}

// Iterables_getT_ is Iterables.get (Iterables_get).
func Iterables_getT_[T any](p0 IterableT[T], p1 int32) T {
	if l, ok := p0.(ListT[T]); ok {
		return l.get_t(p1)
	}
	if p1 < 0 {
		panic(new_IndexOutOfBoundsException_String("position (" + istr_(int64(p1)) + ") must not be negative").self())
	}
	var n int32
	for it := p0.iterator_t(); it.hasNext(); n++ {
		v := it.next_t()
		if n == p1 {
			return v
		}
	}
	panic(new_IndexOutOfBoundsException_String("position (" + istr_(int64(p1)) + ") must be less than the number of elements that remained (" + istr_(int64(n)) + ")").self())
}

// Iterables_indexOfT_ is Iterables.indexOf (Iterables_indexOf).
func Iterables_indexOfT_[T any](p0 IterableT[T], p1 Predicate_base[T]) int32 {
	var i int32
	for it := p0.iterator_t(); it.hasNext(); i++ {
		if p1(it.next_t()) {
			return i
		}
	}
	return -1
}

// iterViewT_ is iterView_ of a typed Iterable: an Iterable whose iterators mk makes.
type iterViewT_[T any] struct {
	mk func() IteratorT[T]
}

func (v *iterViewT_[T]) iterator() Iterator       { return v.mk() }
func (v *iterViewT_[T]) iterator_t() IteratorT[T] { return v.mk() }
func (v *iterViewT_[T]) toString() string         { return collToString_(v) }

// filterIterT_ is filterIter_ at the element type (it has no remove, as filterIter_'s).
type filterIterT_[T any] struct {
	it    IteratorT[T]
	keep  Predicate_base[T]
	nx    T
	ready bool
}

func (f *filterIterT_[T]) hasNext() bool {
	for !f.ready && f.it.hasNext() {
		v := f.it.next_t()
		if f.keep(v) {
			f.nx, f.ready = v, true
		}
	}
	return f.ready
}

func (f *filterIterT_[T]) next_t() T {
	if !f.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	v := f.nx
	f.nx, f.ready = null_[T](), false
	return v
}

func (f *filterIterT_[T]) next() any { return boxT_(f.next_t()) }
func (f *filterIterT_[T]) remove()   { panic(new_UnsupportedOperationException_0().self()) }

// Iterables_filterT_ is Iterables.filter(Iterable, Predicate) (Iterables_filter).
func Iterables_filterT_[T any](p0 IterableT[T], p1 Predicate_base[T]) IterableT[T] {
	return &iterViewT_[T]{func() IteratorT[T] { return &filterIterT_[T]{it: p0.iterator_t(), keep: p1} }}
}

// mapIterT_ is mapIter_ at the element types.
type mapIterT_[F, T any] struct {
	it IteratorT[F]
	f  Function_base[F, T]
}

func (m *mapIterT_[F, T]) hasNext() bool { return m.it.hasNext() }
func (m *mapIterT_[F, T]) next_t() T     { return m.f(m.it.next_t()) }
func (m *mapIterT_[F, T]) next() any     { return boxT_(m.next_t()) }
func (m *mapIterT_[F, T]) remove()       { Iterator_remove(m.it) }

// Iterables_transformT_ is Iterables.transform (Iterables_transform).
func Iterables_transformT_[F, T any](p0 IterableT[F], p1 Function_base[F, T]) IterableT[T] {
	return &iterViewT_[T]{func() IteratorT[T] { return &mapIterT_[F, T]{it: p0.iterator_t(), f: p1} }}
}

// ---- MoreObjects

// MoreObjects_firstNonNullT_ is MoreObjects.firstNonNull (MoreObjects_firstNonNull).
func MoreObjects_firstNonNullT_[T any](a T, b T) T {
	if !isnull_(a) {
		return a
	}
	if !isnull_(b) {
		return b
	}
	panic(new_NullPointerException_String("Both parameters are null").self())
}

// ---- the default methods of Iterable, List and Map

// Iterable_forEachT_ is Iterable.forEach (Iterable_forEach).
func Iterable_forEachT_[T any](it IterableT[T], c Consumer[T]) {
	for i := it.iterator_t(); i.hasNext(); {
		c(i.next_t())
	}
}

// List_sortT_ is List.sort and Collections.sort(List, Comparator) (List_sort): the elements
// sorted in a copy (stable, as sortSlice_ sorts) and set back in their order; a null
// comparator is the natural ordering.
func List_sortT_[T any](l ListT[T], c Comparator[T]) {
	n := l.size()
	s := make([]T, n)
	for i := int32(0); i < n; i++ {
		s[i] = l.get_t(i)
	}
	if c == nil {
		c = naturalT_[T]
	}
	sort.SliceStable(s, func(i, j int) bool { return c(s[i], s[j]) < 0 })
	for i, v := range s {
		l.set_t(int32(i), v)
	}
}

// naturalT_ compares two values by their natural ordering, as naturalOrder_ compares their boxes
// (two Strings as they are).
func naturalT_[T any](a, b T) int32 {
	if p, ok := any(&a).(*string); ok {
		if q := any(&b).(*string); *p != nullstr_ && *q != nullstr_ {
			return String_compareTo(*p, *q)
		}
	}
	if r, ok := cmpTypedT_(a, b); ok {
		return r
	}
	return compareNatural_(boxT_(a), boxT_(b))
}

// Map_forEachT_ is Map.forEach (Map_forEach).
func Map_forEachT_[K, V any](m MapT[K, V], f BiConsumer[K, V]) {
	for it := m.entrySet_t().iterator_t(); it.hasNext(); {
		e := it.next_t()
		f(e.getKey_t(), e.getValue_t())
	}
}

// Map_replaceAllT_ is Map.replaceAll (Map_replaceAll).
func Map_replaceAllT_[K, V any](m MapT[K, V], f BiFunction[K, V, V]) {
	for it := m.entrySet_t().iterator_t(); it.hasNext(); {
		e := it.next_t()
		e.setValue_t(f(e.getKey_t(), e.getValue_t()))
	}
}

// Map_computeIfAbsentT_ is Map.computeIfAbsent (Map_computeIfAbsent).
func Map_computeIfAbsentT_[K, V any](m MapT[K, V], k K, f Function[K, V]) V {
	if old := m.get_t(boxT_(k)); !isnull_(old) {
		return old
	}
	v := f(k)
	if !isnull_(v) {
		m.put_t(k, v)
	}
	return v
}

// Map_getOrDefaultT_ is Map.getOrDefault (Map_getOrDefault).
func Map_getOrDefaultT_[K, V any](m MapT[K, V], k any, def V) V {
	if v := m.get_t(k); !isnull_(v) || m.containsKey(k) {
		return v
	}
	return def
}

// Map_get_sT_ is Map.get with a String key on a map whose keys are Strings (Map_get_s): the
// typed map's own lookup, else the erased helper's and the cast of erased code.
func Map_get_sT_[V any](m MapT[string, V], k string) V {
	if v, ok := m.(*tmapView_[string, V]); ok && k != nullstr_ {
		return v.m.get(k)
	}
	return unboxT_[V](Map_get_s(Map_un_(m), k))
}

// Map_getOrDefault_sT_ is Map.getOrDefault with a String key on a map whose keys are Strings
// (Map_getOrDefault_s).
func Map_getOrDefault_sT_[V any](m MapT[string, V], k string, def V) V {
	if v, ok := m.(*tmapView_[string, V]); ok && k != nullstr_ {
		return v.m.getOrDefault(k, def)
	}
	u := Map_un_(m)
	if v := Map_get_s(u, k); v != nil || Map_containsKey_s(u, k) {
		return unboxT_[V](v)
	}
	return def
}

// ---- constructors with a comparator, and Splitter

// newTSortedSetCT_ is newTSortedSetC_ with the comparator at the element type: the set compares
// its keys with it as they are (tsortCmp_); erased code that asks for the comparator gets it at
// Object.
func newTSortedSetCT_[K comparable](c *tconv_[K], cmp Comparator[K]) *TSet[K] {
	s := newTSortedSetC_(c, cmpErased_(c, cmp))
	if cmp != nil {
		s.eq.cmpT = (func(K, K) int32)(cmp)
	}
	return s
}

// newTSortedMapCT_ is newTSortedMapC_ with the comparator at the key type.
func newTSortedMapCT_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], cmp Comparator[K]) *TMap[K, V] {
	m := newTSortedMapC_(kc, vc, cmpErased_(kc, cmp))
	if cmp != nil {
		m.eq.cmpT = (func(K, K) int32)(cmp)
	}
	return m
}

// cmpErased_ is comparator cmp of keys of type K as erased code calls it (null stays null).
func cmpErased_[K any](c *tconv_[K], cmp Comparator[K]) Comparator_e {
	if cmp == nil {
		return nil
	}
	return func(a, b any) int32 { return cmp(c.unbox(a), c.unbox(b)) }
}

// splitToListST_ is splitToListT_ of a String (no box).
func splitToListST_(c *tconv_[string], sp *Splitter, s string) *TList[string] {
	if s == nullstr_ {
		return splitToListT_(c, sp, nil)
	}
	l := &TList[string]{c: c, items: sp.splitStrs_(s), twin: unmodMark_}
	l.view.l = l
	return l
}

// Collections_sortT_ is Collections.sort(List) (Collections_sort).
func Collections_sortT_[T any](l ListT[T]) { List_sortT_(l, nil) }

// Map_mergeT_ is Map.merge (Map_merge).
func Map_mergeT_[K, V any](m MapT[K, V], k K, v V, f BiFunction[V, V, V]) V {
	old := m.get_t(boxT_(k))
	nv := v
	if !isnull_(old) {
		nv = f(old, v)
	}
	if isnull_(nv) {
		m.remove_t(boxT_(k))
	} else {
		m.put_t(k, nv)
	}
	return nv
}

// ---- SequencedCollection (piece `seqcoll`, J2GO_NOLIBGEN=seqcoll): its default methods, on any
// receiver at the typed Collection of its elements (LibFns.RECV_AT), as the runtime's helpers
// of rt_coll.go do them (SequencedCollection_getFirst ...), step by step.

// SequencedCollection_getFirstT_ is SequencedCollection.getFirst (SequencedCollection_getFirst).
func SequencedCollection_getFirstT_[T any](c CollectionT[T]) T {
	it := c.iterator_t()
	if !it.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	return it.next_t()
}

// SequencedCollection_getLastT_ is SequencedCollection.getLast (SequencedCollection_getLast).
func SequencedCollection_getLastT_[T any](c CollectionT[T]) T {
	if l, ok := c.(ListT[T]); ok {
		n := l.size()
		if n == 0 {
			panic(new_NoSuchElementException_0().self())
		}
		return l.get_t(n - 1)
	}
	it := c.iterator_t()
	if !it.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	var v T
	for it.hasNext() {
		v = it.next_t()
	}
	return v
}

// SequencedCollection_removeLastT_ is SequencedCollection.removeLast (SequencedCollection_removeLast).
func SequencedCollection_removeLastT_[T any](c CollectionT[T]) T {
	v := SequencedCollection_getLastT_(c)
	if l, ok := c.(interface{ remove(int32) any }); ok {
		l.remove(c.size() - 1)
	} else {
		c.remove_Object(boxT_(v))
	}
	return v
}

// SequencedCollection_addLastT_ is SequencedCollection.addLast (SequencedCollection_addLast).
func SequencedCollection_addLastT_[T any](c CollectionT[T], v T) { c.add_t(v) }
