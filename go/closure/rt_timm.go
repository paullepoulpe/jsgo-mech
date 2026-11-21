package closure

import "sync"

// Typed immutable collections (j2go rule "typed collections"): Guava's ImmutableList<T>,
// ImmutableSet<K> and ImmutableMap<K, V> in a typed declaration are the typed list, set and
// map of rt_tlist.go, rt_tset.go and rt_tmap.go with `twin` set: a *TList[T] that never
// changes (every change throws UnsupportedOperationException, as Guava's do). So a List<T>
// declaration holds an ArrayList or an ImmutableList in one Go type, as it does in Java.
//
// Erased code sees a typed immutable collection as its twin, the runtime's ImmutableList /
// ImmutableSet / ImmutableMap (rt_guava.go, rt_guava_map.go) with the same elements, boxed:
// asImm(), which asList() / asSet() / asMap() also return for an immutable collection. The
// twin is made the first time erased code needs it (the contents never change, so the copy
// cannot be told from the collection), and it knows its typed collection (typed_). An
// immutable collection that erased code made enters a typed declaration through tlistImm_ /
// tsetImm_ / tmapImm_, which make the typed collection of its twin once. Either way a Java
// object is one typed object and one erased object, each found from the other, so == and
// "copyOf returns its argument" hold on both sides.
//
// Guava's semantics are kept as rt_guava.go keeps them: insertion order (a set or a map keeps
// the first position of an element or key), null elements, keys and values rejected with
// NullPointerException (the same messages), duplicate keys of a map with
// IllegalArgumentException, one empty collection (per element type on the typed side; their
// twin is the runtime's one empty collection), copyOf of an immutable collection the
// collection itself.

// immPending_ marks a typed immutable list or set whose twin is not made yet.
var immPending_ = &ImmutableCollection{}

// unmodMark_ marks a typed list that cannot be changed but is not an ImmutableList (see
// TList.twin).
var unmodMark_ = &ImmutableCollection{}

// timmMap_ is what an immutable typed map has beside its mappings.
type timmMap_ struct {
	twin *ImmutableMap // the map as erased code sees it, once that is needed
	keys any           // keySet() as typed code sees it (a *TSet[K]), once that is needed
}

// immPendingMap_ is shared by the immutable typed maps that have neither yet.
var immPendingMap_ = &timmMap_{}

// immNull_ reports whether x is Java's null.
func immNull_[T any](c *tconv_[T], x T) bool {
	if c.isnull != nil {
		return c.isnull(x)
	}
	return any(x) == any(c.null)
}

func boxAll_[T any](c *tconv_[T], s []T) []any {
	out := make([]any, len(s))
	for i, x := range s {
		out[i] = c.box(x)
	}
	return out
}

// immAlt_ holds the typed collections of an erased immutable collection that typed code sees
// at a second Go type (unchecked code: the same list as a List<Object> and a List<String>);
// the first is in typed_.
var immAlt_ = struct {
	mu sync.Mutex
	m  map[any][]any
}{m: map[any][]any{}}

// immAltGet_ returns the typed collection of type P that erased collection ic has, or makes it.
func immAltGet_[P any](ic any, mk func() P) P {
	immAlt_.mu.Lock()
	defer immAlt_.mu.Unlock()
	for _, x := range immAlt_.m[ic] {
		if p, ok := x.(P); ok {
			return p
		}
	}
	p := mk()
	immAlt_.m[ic] = append(immAlt_.m[ic], p)
	return p
}

func classCastImm_(from, to string) {
	panic(new_ClassCastException_String("class " + from + " cannot be cast to class " + to).self())
}

// frozen_ is the panic of a change to an immutable typed collection: catch_ makes it Java's
// UnsupportedOperationException. (A panic statement keeps TList.add and set inlinable; a call
// of a function that throws does not.)
type frozen_ struct{}

// ---- lists

// tlistEmptyImm_ is ImmutableList.of() as a list of T: one per element type; its twin is the
// runtime's empty list.
func tlistEmptyImm_[T any](c *tconv_[T]) *TList[T] {
	if l, ok := c.emptyL.(*TList[T]); ok {
		return l
	}
	l := &TList[T]{items: []T{}, c: c, twin: emptyImmList_}
	l.view.l = l
	c.emptyL = l
	return l
}

// tlistOwn_ is the immutable list of items, which it owns (not empty, no nulls).
func tlistOwn_[T any](c *tconv_[T], items []T) *TList[T] {
	l := &TList[T]{items: items, c: c, twin: immPending_}
	l.view.l = l
	return l
}

// immutableListOf_ is ImmutableList.of(xs...), which owns xs: a null element throws
// NullPointerException.
func immutableListOf_[T any](c *tconv_[T], xs ...T) *TList[T] {
	for i, x := range xs {
		if immNull_(c, x) {
			throwNPE_("at index " + istr_(int64(i)))
		}
	}
	if len(xs) == 0 {
		return tlistEmptyImm_(c)
	}
	return tlistOwn_(c, xs)
}

// tlistImm_ is an ImmutableList that erased code made, entering a typed declaration: the typed
// list of its twin ic, made once (nil for null).
func tlistImm_[T any](c *tconv_[T], ic *ImmutableCollection) *TList[T] {
	if ic == nil {
		return nil
	}
	if l, ok := ic.typed_.(*TList[T]); ok {
		return l
	}
	return tlistImmNew_(c, ic)
}

func tlistImmNew_[T any](c *tconv_[T], ic *ImmutableCollection) *TList[T] {
	if ic.kind != icList_ {
		classCastImm_("com.google.common.collect.ImmutableSet", "com.google.common.collect.ImmutableList")
	}
	if len(ic.items) == 0 {
		return tlistEmptyImm_(c)
	}
	mk := func() *TList[T] {
		items := make([]T, len(ic.items))
		for i, x := range ic.items {
			items[i] = c.unbox(x)
		}
		l := &TList[T]{items: items, c: c, twin: ic}
		l.view.l = l
		return l
	}
	if ic.typed_ != nil {
		return immAltGet_(any(ic), mk)
	}
	l := mk()
	ic.typed_ = l
	return l
}

// asImm is an immutable list as erased code sees it: its twin, the runtime's ImmutableList
// (nil for null). A list that is not immutable is not an ImmutableList: ClassCastException.
func (l *TList[T]) asImm() *ImmutableCollection {
	if l == nil {
		return nil
	}
	if l.twin == immPending_ {
		t := &ImmutableCollection{kind: icList_, items: boxAll_(l.c, l.items)}
		t.typed_ = l
		l.twin = t
	} else if l.twin == nil || l.twin == unmodMark_ {
		classCastImm_("java.util.ArrayList", "com.google.common.collect.ImmutableList")
	}
	return l.twin
}

// asOtherList is asList() of null (nil), of an immutable list (its twin) or of a list that
// cannot be changed (its view).
// (not inlined, so that asList is: its common case is the view)
//
//go:noinline
func (l *TList[T]) asOtherList() List {
	if l == nil {
		return nil
	}
	if l.twin == unmodMark_ {
		return &l.view
	}
	return l.asImm()
}

// ---- sets

// tsetEmptyImm_ is ImmutableSet.of() as a set of K: one per element type.
func tsetEmptyImm_[K comparable](c *tconv_[K]) *TSet[K] {
	if s, ok := c.emptyS.(*TSet[K]); ok {
		return s
	}
	s := newTSet_(c)
	s.twin = emptyImmSet_
	c.emptyS = s
	return s
}

// tsetMake_ is the immutable set of the distinct elements of xs, in first-seen order (xs is
// checked for nulls).
func tsetMake_[K comparable](c *tconv_[K], xs []K) *TSet[K] {
	if len(xs) == 0 {
		return tsetEmptyImm_(c)
	}
	s := newTSet_(c)
	if !c.byEquals {
		s.keys = make([]K, 0, len(xs))
	}
	for _, x := range xs {
		s.add0(x)
	}
	s.twin = immPending_
	return s
}

// immutableSetOf_ is ImmutableSet.of(xs...): a null element throws NullPointerException.
func immutableSetOf_[K comparable](c *tconv_[K], xs ...K) *TSet[K] {
	for _, x := range xs {
		if immNull_(c, x) {
			panic(new_NullPointerException_0().self())
		}
	}
	return tsetMake_(c, xs)
}

// tsetImm_ is an ImmutableSet that erased code made, entering a typed declaration (see
// tlistImm_).
func tsetImm_[K comparable](c *tconv_[K], ic *ImmutableCollection) *TSet[K] {
	if ic == nil {
		return nil
	}
	if s, ok := ic.typed_.(*TSet[K]); ok {
		return s
	}
	return tsetImmNew_(c, ic)
}

func tsetImmNew_[K comparable](c *tconv_[K], ic *ImmutableCollection) *TSet[K] {
	if ic.kind == icList_ {
		classCastImm_("com.google.common.collect.ImmutableList", "com.google.common.collect.ImmutableSet")
	}
	if ic == emptyImmSet_ {
		return tsetEmptyImm_(c)
	}
	mk := func() *TSet[K] {
		s := newTSet_(c)
		if ic.kind == icSorted_ {
			// an ImmutableSortedSet: a sorted typed set with its comparator, the elements as
			// they are ordered
			s.eq = &teq_{sorted: true, cmp: ic.cmp}
			s.keys = make([]K, len(ic.items))
			for i, x := range ic.items {
				s.keys[i] = c.unbox(x)
			}
			s.live = int32(len(s.keys))
		} else {
			for _, x := range ic.elems_() {
				s.add0(c.unbox(x))
			}
		}
		s.twin = ic
		return s
	}
	if ic.typed_ != nil {
		return immAltGet_(any(ic), mk)
	}
	s := mk()
	ic.typed_ = s
	return s
}

// liveKeys is the elements of the set in order (the set's own slice if nothing was removed).
func (s *TSet[K]) liveKeys() []K {
	if s.gone == nil {
		return s.keys
	}
	out := make([]K, 0, s.live)
	for i, k := range s.keys {
		if !s.gone[i] {
			out = append(out, k)
		}
	}
	return out
}

// asImm is an immutable set as erased code sees it: its twin, the runtime's ImmutableSet.
func (s *TSet[K]) asImm() *ImmutableCollection {
	if s == nil {
		return nil
	}
	if s.twin == immPending_ {
		// the twin's index is built on its first lookup (ImmutableCollection.find_)
		t := &ImmutableCollection{kind: icSet_, items: boxAll_(s.c, s.keys)}
		t.typed_ = s
		s.twin = t
	} else if s.twin == nil || s.twin == sortedMark_ {
		classCastImm_("java.util."+s.view.goName_(), "com.google.common.collect.ImmutableSet")
	}
	return s.twin
}

// asOtherSet is asSet() of null (nil), of an immutable set (its twin) or of a sorted set (its
// sorted view).
// (not inlined, so that asSet is: its common case is the view)
//
//go:noinline
func (s *TSet[K]) asOtherSet() Set {
	if s == nil {
		return nil
	}
	if s.twin == sortedMark_ {
		return s.sortedView()
	}
	return s.asImm()
}

// ---- maps

// tmapEmptyImm_ is ImmutableMap.of() as a map of K to V: one per key and value type.
func tmapEmptyImm_[K comparable, V any](kc *tconv_[K], vc *tconv_[V]) *TMap[K, V] {
	if m, ok := vc.emptyM.(*TMap[K, V]); ok && m.kc == kc {
		return m
	}
	if m, ok := kc.emptyM.(*TMap[K, V]); ok && m.vc == vc {
		return m
	}
	mk := func() *TMap[K, V] {
		m := newTMap_(kc, vc)
		m.imm = &timmMap_{twin: emptyImmMap_}
		return m
	}
	if vc.emptyM != nil && kc.emptyM != nil {
		// (both conversions have the empty map of another pair of types: found, not made again)
		return immAltGet_(any(emptyImmMap_), mk)
	}
	m := mk()
	if vc.emptyM == nil {
		vc.emptyM = m
	} else {
		kc.emptyM = m
	}
	return m
}

// tmapMake_ is the immutable map of parallel keys and values, as immMap_ makes an
// ImmutableMap: nulls are checked first, then a duplicate key throws
// IllegalArgumentException.
func tmapMake_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], keys []K, vals []V) *TMap[K, V] {
	if len(keys) == 0 {
		return tmapEmptyImm_(kc, vc)
	}
	for i := range keys {
		if immNull_(kc, keys[i]) {
			throwNPE_("null key in entry: null=" + objstr_(vc.box(vals[i])))
		}
		if immNull_(vc, vals[i]) {
			throwNPE_("null value in entry: " + objstr_(kc.box(keys[i])) + "=null")
		}
	}
	m := newTMap_(kc, vc)
	for i, k := range keys {
		j, h := m.locate(k)
		if j >= 0 {
			immDup_(boxAll_(kc, keys), boxAll_(vc, vals))
		}
		m.append(k, vals[i], h)
	}
	m.imm = immPendingMap_
	return m
}

// immutableMapOf1_ ... immutableMapOf5_ are ImmutableMap.of(k1, v1, ...): the arguments in
// Java's order.
func immutableMapOf1_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], k1 K, v1 V) *TMap[K, V] {
	return tmapMake_(kc, vc, []K{k1}, []V{v1})
}

func immutableMapOf2_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], k1 K, v1 V, k2 K, v2 V) *TMap[K, V] {
	return tmapMake_(kc, vc, []K{k1, k2}, []V{v1, v2})
}

func immutableMapOf3_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], k1 K, v1 V, k2 K, v2 V, k3 K, v3 V) *TMap[K, V] {
	return tmapMake_(kc, vc, []K{k1, k2, k3}, []V{v1, v2, v3})
}

func immutableMapOf4_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], k1 K, v1 V, k2 K, v2 V, k3 K, v3 V, k4 K, v4 V) *TMap[K, V] {
	return tmapMake_(kc, vc, []K{k1, k2, k3, k4}, []V{v1, v2, v3, v4})
}

func immutableMapOf5_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], k1 K, v1 V, k2 K, v2 V, k3 K, v3 V, k4 K, v4 V, k5 K, v5 V) *TMap[K, V] {
	return tmapMake_(kc, vc, []K{k1, k2, k3, k4, k5}, []V{v1, v2, v3, v4, v5})
}

// tmapImm_ is an ImmutableMap that erased code made, entering a typed declaration (see
// tlistImm_).
func tmapImm_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], im *ImmutableMap) *TMap[K, V] {
	if im == nil {
		return nil
	}
	if m, ok := im.typed_.(*TMap[K, V]); ok {
		return m
	}
	return tmapImmNew_(kc, vc, im)
}

func tmapImmNew_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], im *ImmutableMap) *TMap[K, V] {
	if im == emptyImmMap_ {
		return tmapEmptyImm_(kc, vc)
	}
	mk := func() *TMap[K, V] {
		m := newTMap_(kc, vc)
		if im.kind == imSorted_ {
			// an ImmutableSortedMap: a sorted typed map with its comparator
			m.eq = &teq_{sorted: true, cmp: im.cmp}
		}
		for i, x := range im.keys {
			k := kc.unbox(x)
			_, h := m.locate(k)
			m.append(k, vc.unbox(im.vals[i]), h)
		}
		m.imm = &timmMap_{twin: im}
		return m
	}
	if im.typed_ != nil {
		return immAltGet_(any(im), mk)
	}
	m := mk()
	im.typed_ = m
	return m
}

// asImm is an immutable map as erased code sees it: its twin, the runtime's ImmutableMap.
func (m *TMap[K, V]) asImm() *ImmutableMap {
	if m == nil {
		return nil
	}
	if m.imm == nil || m.imm == sortedMapMark_ {
		classCastImm_("java.util."+m.view.goName_(), "com.google.common.collect.ImmutableMap")
	}
	x := m.info()
	if x.twin == nil {
		// the twin's index is built on its first lookup (ImmutableMap.index_)
		t := &ImmutableMap{kind: imMap_, keys: boxAll_(m.kc, m.keys), vals: boxAll_(m.vc, m.vals)}
		t.typed_ = m
		if s, ok := x.keys.(*TSet[K]); ok {
			// its key set is the one that typed code has
			t.keySet_ = s.asImm()
		}
		x.twin = t
	}
	return x.twin
}

// info is what the immutable map has beside its mappings, to fill in.
func (m *TMap[K, V]) info() *timmMap_ {
	if m.imm == immPendingMap_ {
		m.imm = &timmMap_{}
	}
	return m.imm
}

// keySetImm is keySet() of an immutable map (an ImmutableSet<K>) for a typed declaration: one
// typed set, the same object as the key set that erased code gets from the map's twin.
func (m *TMap[K, V]) keySetImm() *TSet[K] {
	if m.imm == nil || m.imm == sortedMapMark_ {
		classCastImm_("java.util."+m.view.goName_(), "com.google.common.collect.ImmutableMap")
	}
	x := m.info()
	if s, ok := x.keys.(*TSet[K]); ok {
		return s
	}
	var s *TSet[K]
	if x.twin != nil {
		// the key set of the twin, as a typed set
		s = tsetImm_(m.kc, x.twin.keySet().(*ImmutableCollection))
	} else {
		s = newTSet_(m.kc)
		for _, k := range m.keys {
			s.add0(k)
		}
		s.twin = immPending_
	}
	x.keys = s
	return s
}

// asOtherMap is asMap() of null (nil), of an immutable map (its twin) or of a sorted map (its
// sorted view).
// (not inlined, so that asMap is: its common case is the view)
//
//go:noinline
func (m *TMap[K, V]) asOtherMap() Map {
	if m == nil {
		return nil
	}
	if m.imm == sortedMapMark_ {
		return m.sortedView()
	}
	return m.asImm()
}

// ---- copyOf

// immutableListCopyOfT_ is ImmutableList.copyOf(l) of a typed list of the same element type:
// an immutable list is itself; any other is copied (a null element throws
// NullPointerException).
func immutableListCopyOfT_[T any](c *tconv_[T], l *TList[T]) *TList[T] {
	if l.twin != nil && l.twin != unmodMark_ {
		return l
	}
	return immutableListCopyOfArr_(c, l.items)
}

// immutableListCopyOfArr_ is ImmutableList.copyOf(a) of an array a.
func immutableListCopyOfArr_[T any](c *tconv_[T], a []T) *TList[T] {
	for i, x := range a {
		if immNull_(c, x) {
			throwNPE_("at index " + istr_(int64(i)))
		}
	}
	if len(a) == 0 {
		return tlistEmptyImm_(c)
	}
	return tlistOwn_(c, append([]T(nil), a...))
}

// immutableListCopyOf_ is ImmutableList.copyOf(from) of a collection as erased code sees it:
// an immutable collection gives its asList() (itself, for a list).
func immutableListCopyOf_[T any](c *tconv_[T], from Iterable) *TList[T] {
	switch x := from.(type) {
	case *ImmutableCollection:
		return tlistImm_(c, x.asList())
	case *tlistView_[T]:
		return immutableListCopyOfArr_(c, x.l.items)
	}
	src := toSlice_(from)
	checkNoNulls_(src)
	if len(src) == 0 {
		return tlistEmptyImm_(c)
	}
	items := make([]T, len(src))
	for i, x := range src {
		items[i] = c.unbox(x)
	}
	return tlistOwn_(c, items)
}

// immutableSetCopyOfT_ is ImmutableSet.copyOf(s) of a typed set of the same element type.
func immutableSetCopyOfT_[K comparable](c *tconv_[K], s *TSet[K]) *TSet[K] {
	if s.twin != nil && s.twin != sortedMark_ && (s.eq == nil || !s.eq.sorted) {
		return s
	}
	return immutableSetOf_(c, s.liveKeys()...)
}

// immutableSetCopyOf_ is ImmutableSet.copyOf(from) of a collection as erased code sees it: an
// ImmutableSet (not a sorted one) is itself.
func immutableSetCopyOf_[K comparable](c *tconv_[K], from Iterable) *TSet[K] {
	switch x := from.(type) {
	case *ImmutableCollection:
		if x.kind == icSet_ {
			return tsetImm_(c, x)
		}
	case *tsetView_[K]:
		return immutableSetOf_(c, x.s.liveKeys()...)
	case *tlistView_[K]:
		return immutableSetOf_(c, x.l.items...)
	}
	src := toSlice_(from)
	xs := make([]K, len(src))
	for i, x := range src {
		if x == nil {
			panic(new_NullPointerException_0().self())
		}
		xs[i] = c.unbox(x)
	}
	return tsetMake_(c, xs)
}

// liveKV is the keys and values of the map in order (the map's own slices if nothing was
// removed).
func (m *TMap[K, V]) liveKV() ([]K, []V) {
	if m.gone == nil {
		return m.keys, m.vals
	}
	keys, vals := make([]K, 0, m.live), make([]V, 0, m.live)
	for i, k := range m.keys {
		if !m.gone[i] {
			keys, vals = append(keys, k), append(vals, m.vals[i])
		}
	}
	return keys, vals
}

// immutableMapCopyOfT_ is ImmutableMap.copyOf(m) of a typed map of the same types.
func immutableMapCopyOfT_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], m *TMap[K, V]) *TMap[K, V] {
	if m.imm != nil && m.imm != sortedMapMark_ && (m.eq == nil || !m.eq.sorted) {
		return m
	}
	keys, vals := m.liveKV()
	return tmapMake_(kc, vc, keys, vals)
}

// immutableMapCopyOf_ is ImmutableMap.copyOf(from) of a map as erased code sees it: an
// ImmutableMap (not a sorted one) is itself.
func immutableMapCopyOf_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], from Map) *TMap[K, V] {
	switch x := from.(type) {
	case *ImmutableMap:
		if x.kind != imSorted_ {
			return tmapImm_(kc, vc, x)
		}
	case *tmapView_[K, V]:
		keys, vals := x.m.liveKV()
		return tmapMake_(kc, vc, keys, vals)
	}
	bk, bv := mapKV_(from)
	for i := range bk {
		if bk[i] == nil {
			throwNPE_("null key in entry: null=" + objstr_(bv[i]))
		}
		if bv[i] == nil {
			throwNPE_("null value in entry: " + objstr_(bk[i]) + "=null")
		}
	}
	keys, vals := make([]K, len(bk)), make([]V, len(bv))
	for i := range bk {
		keys[i], vals[i] = kc.unbox(bk[i]), vc.unbox(bv[i])
	}
	return tmapMake_(kc, vc, keys, vals)
}

// ---- builders
//
// ImmutableList.Builder<T>, ImmutableSet.Builder<K> and ImmutableMap.Builder<K, V> in a typed
// declaration: the elements as they are, checked for null where Guava checks them (add and
// addAll: a NullPointerException without a message, as Guava's checkNotNull). Erased
// code never sees a typed builder (Colls: a builder declaration is typed only if every use is
// add, addAll, put, putAll or a build).

type TListBuilder[T any] struct {
	items []T
	c     *tconv_[T]
}

// newTListBuilder_ is ImmutableList.builder() or new ImmutableList.Builder<>().
func newTListBuilder_[T any](c *tconv_[T]) *TListBuilder[T] { return &TListBuilder[T]{c: c} }

// newTListBuilderN_ is ImmutableList.builderWithExpectedSize(n).
func newTListBuilderN_[T any](c *tconv_[T], n int32) *TListBuilder[T] {
	return &TListBuilder[T]{c: c, items: make([]T, 0, n)}
}

func (b *TListBuilder[T]) add(x T) *TListBuilder[T] {
	if immNull_(b.c, x) {
		panic(new_NullPointerException_0().self())
	}
	b.items = append(b.items, x)
	return b
}

// addAll adds the elements of a collection as erased code sees it, each as add does.
func (b *TListBuilder[T]) addAll(from Iterable) *TListBuilder[T] {
	switch x := from.(type) {
	case *tlistView_[T]:
		for _, e := range x.l.items {
			b.add(e)
		}
		return b
	case *ImmutableCollection:
		if l, ok := x.typed_.(*TList[T]); ok {
			b.items = append(b.items, l.items...)
			return b
		}
	}
	for _, e := range toSlice_(from) {
		if e == nil {
			panic(new_NullPointerException_0().self())
		}
		b.items = append(b.items, b.c.unbox(e))
	}
	return b
}

// build is the list of the elements added so far. It shares them with the builder, which only
// appends: the capacity is cut, so a later add never reaches a list already built.
func (b *TListBuilder[T]) build() *TList[T] {
	if len(b.items) == 0 {
		return tlistEmptyImm_(b.c)
	}
	return tlistOwn_(b.c, b.items[:len(b.items):len(b.items)])
}

type TSetBuilder[K comparable] struct {
	items []K
	c     *tconv_[K]
}

// newTSetBuilder_ is ImmutableSet.builder() or new ImmutableSet.Builder<>().
func newTSetBuilder_[K comparable](c *tconv_[K]) *TSetBuilder[K] { return &TSetBuilder[K]{c: c} }

// newTSetBuilderN_ is ImmutableSet.builderWithExpectedSize(n).
func newTSetBuilderN_[K comparable](c *tconv_[K], n int32) *TSetBuilder[K] {
	return &TSetBuilder[K]{c: c, items: make([]K, 0, n)}
}

func (b *TSetBuilder[K]) add(x K) *TSetBuilder[K] {
	if immNull_(b.c, x) {
		panic(new_NullPointerException_0().self())
	}
	b.items = append(b.items, x)
	return b
}

func (b *TSetBuilder[K]) addAll(from Iterable) *TSetBuilder[K] {
	switch x := from.(type) {
	case *tsetView_[K]:
		for _, e := range x.s.liveKeys() {
			b.add(e)
		}
		return b
	case *tlistView_[K]:
		for _, e := range x.l.items {
			b.add(e)
		}
		return b
	case *ImmutableCollection:
		switch t := x.typed_.(type) {
		case *TSet[K]:
			b.items = append(b.items, t.keys...)
			return b
		case *TList[K]:
			b.items = append(b.items, t.items...)
			return b
		}
	}
	for _, e := range toSlice_(from) {
		if e == nil {
			panic(new_NullPointerException_0().self())
		}
		b.items = append(b.items, b.c.unbox(e))
	}
	return b
}

// build is the set of the distinct elements added so far, in first-seen order.
func (b *TSetBuilder[K]) build() *TSet[K] { return tsetMake_(b.c, b.items) }

type TMapBuilder[K comparable, V any] struct {
	keys []K
	vals []V
	kc   *tconv_[K]
	vc   *tconv_[V]
}

// newTMapBuilder_ is ImmutableMap.builder() or new ImmutableMap.Builder<>().
func newTMapBuilder_[K comparable, V any](kc *tconv_[K], vc *tconv_[V]) *TMapBuilder[K, V] {
	return &TMapBuilder[K, V]{kc: kc, vc: vc}
}

// newTMapBuilderN_ is ImmutableMap.builderWithExpectedSize(n).
func newTMapBuilderN_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], n int32) *TMapBuilder[K, V] {
	return &TMapBuilder[K, V]{kc: kc, vc: vc, keys: make([]K, 0, n), vals: make([]V, 0, n)}
}

func (b *TMapBuilder[K, V]) put(k K, v V) *TMapBuilder[K, V] {
	if immNull_(b.kc, k) {
		throwNPE_("null key in entry: null=" + objstr_(b.vc.box(v)))
	}
	if immNull_(b.vc, v) {
		throwNPE_("null value in entry: " + objstr_(b.kc.box(k)) + "=null")
	}
	b.keys = append(b.keys, k)
	b.vals = append(b.vals, v)
	return b
}

// putAll puts the mappings of a map as erased code sees it, each as put does.
func (b *TMapBuilder[K, V]) putAll(from Map) *TMapBuilder[K, V] {
	var t *TMap[K, V]
	switch x := from.(type) {
	case *tmapView_[K, V]:
		t = x.m
	case *ImmutableMap:
		t, _ = x.typed_.(*TMap[K, V])
	}
	if t != nil {
		keys, vals := t.liveKV()
		for i, k := range keys {
			b.put(k, vals[i])
		}
		return b
	}
	keys, vals := mapKV_(from)
	for i := range keys {
		if keys[i] == nil {
			throwNPE_("null key in entry: null=" + objstr_(vals[i]))
		}
		if vals[i] == nil {
			throwNPE_("null value in entry: " + objstr_(keys[i]) + "=null")
		}
		b.keys = append(b.keys, b.kc.unbox(keys[i]))
		b.vals = append(b.vals, b.vc.unbox(vals[i]))
	}
	return b
}

func (b *TMapBuilder[K, V]) build() *TMap[K, V] { return tmapMake_(b.kc, b.vc, b.keys, b.vals) }

func (b *TMapBuilder[K, V]) buildOrThrow() *TMap[K, V] { return tmapMake_(b.kc, b.vc, b.keys, b.vals) }

// buildKeepingLast is the map in which a key put more than once keeps its first position and
// has its last value (as immMap_ makes it).
func (b *TMapBuilder[K, V]) buildKeepingLast() *TMap[K, V] {
	if len(b.keys) == 0 {
		return tmapEmptyImm_(b.kc, b.vc)
	}
	m := newTMap_(b.kc, b.vc)
	for i, k := range b.keys {
		j, h := m.locate(k)
		if j >= 0 {
			m.vals[j] = b.vals[i]
		} else {
			m.append(k, b.vals[i], h)
		}
	}
	m.imm = immPendingMap_
	return m
}
