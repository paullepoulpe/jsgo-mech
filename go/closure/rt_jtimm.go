package closure

import "sync"

// Typed immutable collections (j2go rule "typed collections"): Guava's ImmutableList<T>,
// ImmutableSet<K> and ImmutableMap<K, V> in a typed declaration are the typed list, set and
// map of rt_tlist.go, rt_tset.go and rt_tmap.go with `twin` set: a *JTList[T] that never
// changes (every change throws UnsupportedOperationException, as Guava's do). So a List<T>
// declaration holds an ArrayList or an ImmutableList in one Go type, as it does in Java.
//
// Erased code sees a typed immutable collection as its twin, the runtime's ImmutableList /
// ImmutableSet / ImmutableMap (rt_guava.go, rt_guava_map.go) with the same elements, boxed:
// asImm(), which asList() / asSet() / asMap() also return for an immutable collection. The
// twin is made the first time erased code needs it (the contents never change, so the copy
// cannot be told from the collection), and it knows its typed collection (jtyped_). An
// immutable collection that erased code made enters a typed declaration through jtlistImm_ /
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
// JTList.twin).
var unmodMark_ = &ImmutableCollection{}

// timmMap_ is what an immutable typed map has beside its mappings.
type timmMap_ struct {
	twin *ImmutableMap // the map as erased code sees it, once that is needed
	keys any           // keySet() as typed code sees it (a *JTSet[K]), once that is needed
}

// immPendingMap_ is shared by the immutable typed maps that have neither yet.
var immPendingMap_ = &timmMap_{}

// jimmNull_ reports whether x is Java's null.
func jimmNull_[T any](c *jtconv_[T], x T) bool { return any(x) == any(c.null) }

func jboxAll_[T any](c *jtconv_[T], s []T) []any {
	out := make([]any, len(s))
	for i, x := range s {
		out[i] = c.box(x)
	}
	return out
}

// immAlt_ holds the typed collections of an erased immutable collection that typed code sees
// at a second Go type (unchecked code: the same list as a List<Object> and a List<String>);
// the first is in jtyped_.
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
// UnsupportedOperationException. (A panic statement keeps JTList.add and set inlinable; a call
// of a function that throws does not.)
type frozen_ struct{}

// ---- lists

// jtlistEmptyImm_ is ImmutableList.of() as a list of T: one per element type; its twin is the
// runtime's empty list.
func jtlistEmptyImm_[T any](c *jtconv_[T]) *JTList[T] {
	if l, ok := c.emptyL.(*JTList[T]); ok {
		return l
	}
	l := &JTList[T]{items: []T{}, c: c, twin: emptyImmList_}
	l.view.l = l
	c.emptyL = l
	return l
}

// tlistOwn_ is the immutable list of items, which it owns (not empty, no nulls).
func tlistOwn_[T any](c *jtconv_[T], items []T) *JTList[T] {
	l := &JTList[T]{items: items, c: c, twin: immPending_}
	l.view.l = l
	return l
}

// jimmutableListOf_ is ImmutableList.of(xs...), which owns xs: a null element throws
// NullPointerException.
func jimmutableListOf_[T any](c *jtconv_[T], xs ...T) *JTList[T] {
	for i, x := range xs {
		if jimmNull_(c, x) {
			throwNPE_("at index " + istr_(int64(i)))
		}
	}
	if len(xs) == 0 {
		return jtlistEmptyImm_(c)
	}
	return tlistOwn_(c, xs)
}

// jtlistImm_ is an ImmutableList that erased code made, entering a typed declaration: the typed
// list of its twin ic, made once (nil for null).
func jtlistImm_[T any](c *jtconv_[T], ic *ImmutableCollection) *JTList[T] {
	if ic == nil {
		return nil
	}
	if l, ok := ic.jtyped_.(*JTList[T]); ok {
		return l
	}
	return tlistImmNew_(c, ic)
}

func tlistImmNew_[T any](c *jtconv_[T], ic *ImmutableCollection) *JTList[T] {
	if ic.kind != icList_ {
		classCastImm_("com.google.common.collect.ImmutableSet", "com.google.common.collect.ImmutableList")
	}
	if len(ic.items) == 0 {
		return jtlistEmptyImm_(c)
	}
	mk := func() *JTList[T] {
		items := make([]T, len(ic.items))
		for i, x := range ic.items {
			items[i] = c.unbox(x)
		}
		l := &JTList[T]{items: items, c: c, twin: ic}
		l.view.l = l
		return l
	}
	if ic.jtyped_ != nil {
		return immAltGet_(any(ic), mk)
	}
	l := mk()
	ic.jtyped_ = l
	return l
}

// asImm is an immutable list as erased code sees it: its twin, the runtime's ImmutableList
// (nil for null). A list that is not immutable is not an ImmutableList: ClassCastException.
func (l *JTList[T]) asImm() *ImmutableCollection {
	if l == nil {
		return nil
	}
	if l.twin == immPending_ {
		t := &ImmutableCollection{kind: icList_, items: jboxAll_(l.c, l.items)}
		t.jtyped_ = l
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
func (l *JTList[T]) asOtherList() List {
	if l == nil {
		return nil
	}
	if l.twin == unmodMark_ {
		return &l.view
	}
	return l.asImm()
}

// ---- sets

// jtsetEmptyImm_ is ImmutableSet.of() as a set of K: one per element type.
func jtsetEmptyImm_[K comparable](c *jtconv_[K]) *JTSet[K] {
	if s, ok := c.emptyS.(*JTSet[K]); ok {
		return s
	}
	s := jnewTSet_(c)
	s.twin = emptyImmSet_
	c.emptyS = s
	return s
}

// jtsetMake_ is the immutable set of the distinct elements of xs, in first-seen order (xs is
// checked for nulls).
func jtsetMake_[K comparable](c *jtconv_[K], xs []K) *JTSet[K] {
	if len(xs) == 0 {
		return jtsetEmptyImm_(c)
	}
	s := jnewTSet_(c)
	if !c.byEquals {
		s.keys = make([]K, 0, len(xs))
	}
	for _, x := range xs {
		s.add0(x)
	}
	s.twin = immPending_
	return s
}

// jimmutableSetOf_ is ImmutableSet.of(xs...): a null element throws NullPointerException.
func jimmutableSetOf_[K comparable](c *jtconv_[K], xs ...K) *JTSet[K] {
	for _, x := range xs {
		if jimmNull_(c, x) {
			panic(new_NullPointerException_0().self())
		}
	}
	return jtsetMake_(c, xs)
}

// tsetImm_ is an ImmutableSet that erased code made, entering a typed declaration (see
// jtlistImm_).
func tsetImm_[K comparable](c *jtconv_[K], ic *ImmutableCollection) *JTSet[K] {
	if ic == nil {
		return nil
	}
	if s, ok := ic.jtyped_.(*JTSet[K]); ok {
		return s
	}
	return tsetImmNew_(c, ic)
}

func tsetImmNew_[K comparable](c *jtconv_[K], ic *ImmutableCollection) *JTSet[K] {
	if ic.kind == icList_ {
		classCastImm_("com.google.common.collect.ImmutableList", "com.google.common.collect.ImmutableSet")
	}
	if ic == emptyImmSet_ {
		return jtsetEmptyImm_(c)
	}
	mk := func() *JTSet[K] {
		s := jnewTSet_(c)
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
	if ic.jtyped_ != nil {
		return immAltGet_(any(ic), mk)
	}
	s := mk()
	ic.jtyped_ = s
	return s
}

// liveKeys is the elements of the set in order (the set's own slice if nothing was removed).
func (s *JTSet[K]) liveKeys() []K {
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
func (s *JTSet[K]) asImm() *ImmutableCollection {
	if s == nil {
		return nil
	}
	if s.twin == immPending_ {
		// the twin's index is built on its first lookup (ImmutableCollection.find_)
		t := &ImmutableCollection{kind: icSet_, items: jboxAll_(s.c, s.keys)}
		t.jtyped_ = s
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
func (s *JTSet[K]) asOtherSet() Set {
	if s == nil {
		return nil
	}
	if s.twin == sortedMark_ {
		return s.sortedView()
	}
	return s.asImm()
}

// ---- maps

// jtmapEmptyImm_ is ImmutableMap.of() as a map of K to V: one per key and value type.
func jtmapEmptyImm_[K comparable, V any](kc *jtconv_[K], vc *jtconv_[V]) *JTMap[K, V] {
	if m, ok := vc.emptyM.(*JTMap[K, V]); ok && m.kc == kc {
		return m
	}
	if m, ok := kc.emptyM.(*JTMap[K, V]); ok && m.vc == vc {
		return m
	}
	m := jnewTMap_(kc, vc)
	m.imm = &timmMap_{twin: emptyImmMap_}
	if vc.emptyM == nil {
		vc.emptyM = m
	} else if kc.emptyM == nil {
		kc.emptyM = m
	} else {
		return immAltGet_(any(emptyImmMap_), func() *JTMap[K, V] { return m })
	}
	return m
}

// jtmapMake_ is the immutable map of parallel keys and values, as immMap_ makes an
// ImmutableMap: nulls are checked first, then a duplicate key throws
// IllegalArgumentException.
func jtmapMake_[K comparable, V any](kc *jtconv_[K], vc *jtconv_[V], keys []K, vals []V) *JTMap[K, V] {
	if len(keys) == 0 {
		return jtmapEmptyImm_(kc, vc)
	}
	for i := range keys {
		if jimmNull_(kc, keys[i]) {
			throwNPE_("null key in entry: null=" + objstr_(vc.box(vals[i])))
		}
		if jimmNull_(vc, vals[i]) {
			throwNPE_("null value in entry: " + objstr_(kc.box(keys[i])) + "=null")
		}
	}
	m := jnewTMap_(kc, vc)
	for i, k := range keys {
		j, h := m.locate(k)
		if j >= 0 {
			immDup_(jboxAll_(kc, keys), jboxAll_(vc, vals))
		}
		m.append(k, vals[i], h)
	}
	m.imm = immPendingMap_
	return m
}

// immutableMapOf1_ ... immutableMapOf5_ are ImmutableMap.of(k1, v1, ...): the arguments in
// Java's order.
func immutableMapOf1_[K comparable, V any](kc *jtconv_[K], vc *jtconv_[V], k1 K, v1 V) *JTMap[K, V] {
	return jtmapMake_(kc, vc, []K{k1}, []V{v1})
}

func immutableMapOf2_[K comparable, V any](kc *jtconv_[K], vc *jtconv_[V], k1 K, v1 V, k2 K, v2 V) *JTMap[K, V] {
	return jtmapMake_(kc, vc, []K{k1, k2}, []V{v1, v2})
}

func immutableMapOf3_[K comparable, V any](kc *jtconv_[K], vc *jtconv_[V], k1 K, v1 V, k2 K, v2 V, k3 K, v3 V) *JTMap[K, V] {
	return jtmapMake_(kc, vc, []K{k1, k2, k3}, []V{v1, v2, v3})
}

func immutableMapOf4_[K comparable, V any](kc *jtconv_[K], vc *jtconv_[V], k1 K, v1 V, k2 K, v2 V, k3 K, v3 V, k4 K, v4 V) *JTMap[K, V] {
	return jtmapMake_(kc, vc, []K{k1, k2, k3, k4}, []V{v1, v2, v3, v4})
}

func immutableMapOf5_[K comparable, V any](kc *jtconv_[K], vc *jtconv_[V], k1 K, v1 V, k2 K, v2 V, k3 K, v3 V, k4 K, v4 V, k5 K, v5 V) *JTMap[K, V] {
	return jtmapMake_(kc, vc, []K{k1, k2, k3, k4, k5}, []V{v1, v2, v3, v4, v5})
}

// tmapImm_ is an ImmutableMap that erased code made, entering a typed declaration (see
// jtlistImm_).
func tmapImm_[K comparable, V any](kc *jtconv_[K], vc *jtconv_[V], im *ImmutableMap) *JTMap[K, V] {
	if im == nil {
		return nil
	}
	if m, ok := im.jtyped_.(*JTMap[K, V]); ok {
		return m
	}
	return tmapImmNew_(kc, vc, im)
}

func tmapImmNew_[K comparable, V any](kc *jtconv_[K], vc *jtconv_[V], im *ImmutableMap) *JTMap[K, V] {
	if im == emptyImmMap_ {
		return jtmapEmptyImm_(kc, vc)
	}
	mk := func() *JTMap[K, V] {
		m := jnewTMap_(kc, vc)
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
	if im.jtyped_ != nil {
		return immAltGet_(any(im), mk)
	}
	m := mk()
	im.jtyped_ = m
	return m
}

// asImm is an immutable map as erased code sees it: its twin, the runtime's ImmutableMap.
func (m *JTMap[K, V]) asImm() *ImmutableMap {
	if m == nil {
		return nil
	}
	if m.imm == nil || m.imm == sortedMapMark_ {
		classCastImm_("java.util."+m.view.goName_(), "com.google.common.collect.ImmutableMap")
	}
	x := m.info()
	if x.twin == nil {
		// the twin's index is built on its first lookup (ImmutableMap.index_)
		t := &ImmutableMap{kind: imMap_, keys: jboxAll_(m.kc, m.keys), vals: jboxAll_(m.vc, m.vals)}
		t.jtyped_ = m
		if s, ok := x.keys.(*JTSet[K]); ok {
			// its key set is the one that typed code has
			t.keySet_ = s.asImm()
		}
		x.twin = t
	}
	return x.twin
}

// info is what the immutable map has beside its mappings, to fill in.
func (m *JTMap[K, V]) info() *timmMap_ {
	if m.imm == immPendingMap_ {
		m.imm = &timmMap_{}
	}
	return m.imm
}

// keySetImm is keySet() of an immutable map (an ImmutableSet<K>) for a typed declaration: one
// typed set, the same object as the key set that erased code gets from the map's twin.
func (m *JTMap[K, V]) keySetImm() *JTSet[K] {
	if m.imm == nil || m.imm == sortedMapMark_ {
		classCastImm_("java.util."+m.view.goName_(), "com.google.common.collect.ImmutableMap")
	}
	x := m.info()
	if s, ok := x.keys.(*JTSet[K]); ok {
		return s
	}
	var s *JTSet[K]
	if x.twin != nil {
		// the key set of the twin, as a typed set
		s = tsetImm_(m.kc, x.twin.keySet().(*ImmutableCollection))
	} else {
		s = jnewTSet_(m.kc)
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
func (m *JTMap[K, V]) asOtherMap() Map {
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
func immutableListCopyOfT_[T any](c *jtconv_[T], l *JTList[T]) *JTList[T] {
	if l.twin != nil && l.twin != unmodMark_ {
		return l
	}
	return jimmutableListCopyOfArr_(c, l.items)
}

// jimmutableListCopyOfArr_ is ImmutableList.copyOf(a) of an array a.
func jimmutableListCopyOfArr_[T any](c *jtconv_[T], a []T) *JTList[T] {
	for i, x := range a {
		if jimmNull_(c, x) {
			throwNPE_("at index " + istr_(int64(i)))
		}
	}
	if len(a) == 0 {
		return jtlistEmptyImm_(c)
	}
	return tlistOwn_(c, append([]T(nil), a...))
}

// jimmutableListCopyOf_ is ImmutableList.copyOf(from) of a collection as erased code sees it:
// an immutable collection gives its asList() (itself, for a list).
func jimmutableListCopyOf_[T any](c *jtconv_[T], from Iterable) *JTList[T] {
	switch x := from.(type) {
	case *ImmutableCollection:
		return jtlistImm_(c, x.asList())
	case *jtlistView_[T]:
		return jimmutableListCopyOfArr_(c, x.l.items)
	}
	src := toSlice_(from)
	checkNoNulls_(src)
	if len(src) == 0 {
		return jtlistEmptyImm_(c)
	}
	items := make([]T, len(src))
	for i, x := range src {
		items[i] = c.unbox(x)
	}
	return tlistOwn_(c, items)
}

// immutableSetCopyOfT_ is ImmutableSet.copyOf(s) of a typed set of the same element type.
func immutableSetCopyOfT_[K comparable](c *jtconv_[K], s *JTSet[K]) *JTSet[K] {
	if s.twin != nil && s.twin != sortedMark_ && (s.eq == nil || !s.eq.sorted) {
		return s
	}
	return jimmutableSetOf_(c, s.liveKeys()...)
}

// jimmutableSetCopyOf_ is ImmutableSet.copyOf(from) of a collection as erased code sees it: an
// ImmutableSet (not a sorted one) is itself.
func jimmutableSetCopyOf_[K comparable](c *jtconv_[K], from Iterable) *JTSet[K] {
	switch x := from.(type) {
	case *ImmutableCollection:
		if x.kind == icSet_ {
			return tsetImm_(c, x)
		}
	case *jtsetView_[K]:
		return jimmutableSetOf_(c, x.s.liveKeys()...)
	case *jtlistView_[K]:
		return jimmutableSetOf_(c, x.l.items...)
	}
	src := toSlice_(from)
	xs := make([]K, len(src))
	for i, x := range src {
		if x == nil {
			panic(new_NullPointerException_0().self())
		}
		xs[i] = c.unbox(x)
	}
	return jtsetMake_(c, xs)
}

// liveKV is the keys and values of the map in order (the map's own slices if nothing was
// removed).
func (m *JTMap[K, V]) liveKV() ([]K, []V) {
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
func immutableMapCopyOfT_[K comparable, V any](kc *jtconv_[K], vc *jtconv_[V], m *JTMap[K, V]) *JTMap[K, V] {
	if m.imm != nil && m.imm != sortedMapMark_ && (m.eq == nil || !m.eq.sorted) {
		return m
	}
	keys, vals := m.liveKV()
	return jtmapMake_(kc, vc, keys, vals)
}

// jimmutableMapCopyOf_ is ImmutableMap.copyOf(from) of a map as erased code sees it: an
// ImmutableMap (not a sorted one) is itself.
func jimmutableMapCopyOf_[K comparable, V any](kc *jtconv_[K], vc *jtconv_[V], from Map) *JTMap[K, V] {
	switch x := from.(type) {
	case *ImmutableMap:
		if x.kind != imSorted_ {
			return tmapImm_(kc, vc, x)
		}
	case *jtmapView_[K, V]:
		keys, vals := x.m.liveKV()
		return jtmapMake_(kc, vc, keys, vals)
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
	return jtmapMake_(kc, vc, keys, vals)
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
	c     *jtconv_[T]
}

// newTListBuilder_ is ImmutableList.builder() or new ImmutableList.Builder<>().
func newTListBuilder_[T any](c *jtconv_[T]) *TListBuilder[T] { return &TListBuilder[T]{c: c} }

// newTListBuilderN_ is ImmutableList.builderWithExpectedSize(n).
func newTListBuilderN_[T any](c *jtconv_[T], n int32) *TListBuilder[T] {
	return &TListBuilder[T]{c: c, items: make([]T, 0, n)}
}

func (b *TListBuilder[T]) add(x T) *TListBuilder[T] {
	if jimmNull_(b.c, x) {
		panic(new_NullPointerException_0().self())
	}
	b.items = append(b.items, x)
	return b
}

// addAll adds the elements of a collection as erased code sees it, each as add does.
func (b *TListBuilder[T]) addAll(from Iterable) *TListBuilder[T] {
	switch x := from.(type) {
	case *jtlistView_[T]:
		for _, e := range x.l.items {
			b.add(e)
		}
		return b
	case *ImmutableCollection:
		if l, ok := x.jtyped_.(*JTList[T]); ok {
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
func (b *TListBuilder[T]) build() *JTList[T] {
	if len(b.items) == 0 {
		return jtlistEmptyImm_(b.c)
	}
	return tlistOwn_(b.c, b.items[:len(b.items):len(b.items)])
}

type TSetBuilder[K comparable] struct {
	items []K
	c     *jtconv_[K]
}

// newTSetBuilder_ is ImmutableSet.builder() or new ImmutableSet.Builder<>().
func newTSetBuilder_[K comparable](c *jtconv_[K]) *TSetBuilder[K] { return &TSetBuilder[K]{c: c} }

// newTSetBuilderN_ is ImmutableSet.builderWithExpectedSize(n).
func newTSetBuilderN_[K comparable](c *jtconv_[K], n int32) *TSetBuilder[K] {
	return &TSetBuilder[K]{c: c, items: make([]K, 0, n)}
}

func (b *TSetBuilder[K]) add(x K) *TSetBuilder[K] {
	if jimmNull_(b.c, x) {
		panic(new_NullPointerException_0().self())
	}
	b.items = append(b.items, x)
	return b
}

func (b *TSetBuilder[K]) addAll(from Iterable) *TSetBuilder[K] {
	switch x := from.(type) {
	case *jtsetView_[K]:
		for _, e := range x.s.liveKeys() {
			b.add(e)
		}
		return b
	case *jtlistView_[K]:
		for _, e := range x.l.items {
			b.add(e)
		}
		return b
	case *ImmutableCollection:
		switch t := x.jtyped_.(type) {
		case *JTSet[K]:
			b.items = append(b.items, t.keys...)
			return b
		case *JTList[K]:
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
func (b *TSetBuilder[K]) build() *JTSet[K] { return jtsetMake_(b.c, b.items) }

type TMapBuilder[K comparable, V any] struct {
	keys []K
	vals []V
	kc   *jtconv_[K]
	vc   *jtconv_[V]
}

// newTMapBuilder_ is ImmutableMap.builder() or new ImmutableMap.Builder<>().
func newTMapBuilder_[K comparable, V any](kc *jtconv_[K], vc *jtconv_[V]) *TMapBuilder[K, V] {
	return &TMapBuilder[K, V]{kc: kc, vc: vc}
}

// newTMapBuilderN_ is ImmutableMap.builderWithExpectedSize(n).
func newTMapBuilderN_[K comparable, V any](kc *jtconv_[K], vc *jtconv_[V], n int32) *TMapBuilder[K, V] {
	return &TMapBuilder[K, V]{kc: kc, vc: vc, keys: make([]K, 0, n), vals: make([]V, 0, n)}
}

func (b *TMapBuilder[K, V]) put(k K, v V) *TMapBuilder[K, V] {
	if jimmNull_(b.kc, k) {
		throwNPE_("null key in entry: null=" + objstr_(b.vc.box(v)))
	}
	if jimmNull_(b.vc, v) {
		throwNPE_("null value in entry: " + objstr_(b.kc.box(k)) + "=null")
	}
	b.keys = append(b.keys, k)
	b.vals = append(b.vals, v)
	return b
}

// putAll puts the mappings of a map as erased code sees it, each as put does.
func (b *TMapBuilder[K, V]) putAll(from Map) *TMapBuilder[K, V] {
	var t *JTMap[K, V]
	switch x := from.(type) {
	case *jtmapView_[K, V]:
		t = x.m
	case *ImmutableMap:
		t, _ = x.jtyped_.(*JTMap[K, V])
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

func (b *TMapBuilder[K, V]) build() *JTMap[K, V] { return jtmapMake_(b.kc, b.vc, b.keys, b.vals) }

func (b *TMapBuilder[K, V]) buildOrThrow() *JTMap[K, V] {
	return jtmapMake_(b.kc, b.vc, b.keys, b.vals)
}

// buildKeepingLast is the map in which a key put more than once keeps its first position and
// has its last value (as immMap_ makes it).
func (b *TMapBuilder[K, V]) buildKeepingLast() *JTMap[K, V] {
	if len(b.keys) == 0 {
		return jtmapEmptyImm_(b.kc, b.vc)
	}
	m := jnewTMap_(b.kc, b.vc)
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
