package closure

import "sync"

// Typed immutable collections: Guava's ImmutableList<T>, ImmutableSet<E> and
// ImmutableMap<K, V> as the generic ImmutableList[T], ImmutableSet[K] and ImmutableMap[K, V],
// with the builders ImmutableList_Builder[T], ImmutableSet_Builder[K] and
// ImmutableMap_Builder[K, V]: the types a translation that keeps Java's type arguments emits
// (post-flip, tools/postflip/colls -kind immlist, immset, immmap retypes the erased flip's
// declarations to them). An immutable collection is the typed collection of rt_tlist.go and
// rt_tmap.go (ImmutableList[T] is TList[T]) with its imm flag set, so it is also what a
// List<T>, Set<T> or Map<K, V> of the typed code holds, without conversion; every change
// throws UnsupportedOperationException. The methods are Guava's (size, isEmpty, get,
// contains, containsKey, getOrDefault; add, addAll, put, putAll, build, buildOrThrow,
// buildKeepingLast on the builders); the static factories are functions taking the element
// conversions (tconv_, which box an element as erased code boxes it):
//
//	ImmutableList.of(a, b)        immutableListOf_(tc, a, b)       (of() too)
//	ImmutableList.copyOf(c)       immutableListCopyOf_(tc, c)      (an Iterable)
//	ImmutableList.copyOf(array)   immutableListCopyOfArr_(tc, a)
//	ImmutableList.builder()       immutableListBuilder_(tc)        (builderWithExpectedSize: N)
//	ImmutableSet.of / copyOf / builder: immutableSetOf_, immutableSetCopyOf_, immutableSetBuilder_
//	ImmutableMap.of(k1, v1, k2, v2): immutableMapOf_(kc, vc, []K{k1, k2}, []V{v1, v2});
//	ImmutableMap.copyOf / builder: immutableMapCopyOf_, immutableMapBuilder_
//
// Guava's semantics are kept as rt_guava.go keeps them: insertion order (a set or map keeps
// the first position of an element or key), null elements, keys and values rejected with
// NullPointerException (the same messages), duplicate map keys with
// IllegalArgumentException, the empty collections one object each (per element type here;
// their erased face is the runtime's singleton), copyOf of an immutable collection the
// collection itself.
//
// Erased code sees a typed immutable collection as its twin: the runtime's
// ErasedImmutableList, ErasedImmutableSet or ErasedImmutableMap with the same elements
// (asImm, also asList/asSet/asMap). A typed collection made from an erased one (tlistImm_,
// tsetFrom_, tmapFrom_) has that one as its twin; one made typed gets a twin, boxed, the
// first time erased code needs it. A twin keeps its typed collection (typed_), so a value
// that crosses to erased code and back is the same object, as in Java.

type ImmutableList[T any] = TList[T]
type ImmutableSet[K comparable] = TSet[K]
type ImmutableMap[K comparable, V any] = TMap[K, V]

// immNull_ reports whether x is Java's null as the erased code would box it.
func immNull_[T any](c *tconv_[T], x T) bool {
	if p, ok := any(&x).(*string); ok {
		return *p == nullstr_
	}
	return c.box(x) == nil
}

// boxAll_ is a new slice of the boxed elements of s.
func boxAll_[T any](c *tconv_[T], s []T) []any {
	out := make([]any, len(s))
	for i, x := range s {
		out[i] = c.box(x)
	}
	return out
}

// ---- lists

// tlistEmptyImm_ is ImmutableList.of(): one empty list per element type, whose twin is the
// runtime's empty list.
func tlistEmptyImm_[T any](c *tconv_[T]) *ImmutableList[T] {
	if c.emptyL == nil {
		l := &TList[T]{items: []T{}, c: c, imm: true}
		l.twin = emptyImmList_
		c.emptyL = l
	}
	return c.emptyL
}

// immutableListOf_ is ImmutableList.of(xs...): a null element throws NullPointerException.
func immutableListOf_[T any](c *tconv_[T], xs ...T) *ImmutableList[T] {
	for i, x := range xs {
		if immNull_(c, x) {
			throwNPE_("at index " + istr_(int64(i)))
		}
	}
	if len(xs) == 0 {
		return tlistEmptyImm_(c)
	}
	return &TList[T]{items: xs, c: c, imm: true}
}

// immutableListCopyOf_ is ImmutableList.copyOf(from): an immutable collection gives its asList() (an
// immutable list itself), as a typed list; any other collection is copied, and a null
// element throws NullPointerException.
func immutableListCopyOf_[T any](c *tconv_[T], from Iterable) *ImmutableList[T] {
	if ic, ok := from.(*ImmutableCollection); ok {
		return tlistImm_(c, Collection(ic.asList()))
	}
	if v, ok := from.(*tlistView_[T]); ok && v.c == c {
		return immutableListCopyOfArr_(c, v.items)
	}
	src := copySlice_(from)
	if len(src) == 0 {
		return tlistEmptyImm_(c)
	}
	checkNoNulls_(src)
	return &TList[T]{items: unboxAll_(c, src), c: c, imm: true}
}

// immutableListCopyOfArr_ is ImmutableList.copyOf(a) of an array (or a slice) a: its elements,
// checked for null, are copied, since the array can change later.
func immutableListCopyOfArr_[T any](c *tconv_[T], a []T) *ImmutableList[T] {
	if len(a) == 0 {
		return tlistEmptyImm_(c)
	}
	for i, x := range a {
		if immNull_(c, x) {
			throwNPE_("at index " + istr_(int64(i)))
		}
	}
	return &TList[T]{items: append([]T(nil), a...), c: c, imm: true}
}

// asImm is an immutable list as erased code sees it: its ErasedImmutableList twin.
func (l *TList[T]) asImm() *ErasedImmutableList {
	if l == nil {
		return nil
	}
	if l.unmod {
		// a list that cannot be changed and is no ImmutableList
		classCastImm_("java.util.ArrayList", "com.google.common.collect.ErasedImmutableList")
	}
	if l.twin == nil {
		if len(l.items) == 0 {
			l.twin = emptyImmList_
		} else {
			l.twin = &ImmutableCollection{kind: icList_, items: boxAll_(l.c, l.items)}
			l.twin.typed_ = immTyped_{l.c, l}
		}
	}
	return l.twin
}

// ImmutableList_Builder is ImmutableList.Builder<T> with unboxed elements. build() shares the elements
// so far with the list built, as Guava's builder does: the builder only appends, and the
// capacity is cut so that later adds never reach a list already built.
type ImmutableList_Builder[T any] struct {
	items []T
	c     *tconv_[T]
}

func immutableListBuilder_[T any](c *tconv_[T]) *ImmutableList_Builder[T] {
	return &ImmutableList_Builder[T]{c: c}
}

// immutableListBuilderN_ is ImmutableList.builderWithExpectedSize(n).
func immutableListBuilderN_[T any](c *tconv_[T], n int32) *ImmutableList_Builder[T] {
	return &ImmutableList_Builder[T]{c: c, items: make([]T, 0, n)}
}

func (b *ImmutableList_Builder[T]) add(x T) *ImmutableList_Builder[T] {
	if immNull_(b.c, x) {
		panic(new_NullPointerException_0().self()) // (Guava's checkNotNull: no message)
	}
	if cap(b.items) == 0 {
		b.items = make([]T, 0, 4) // as Guava's builder starts
	}
	b.items = append(b.items, x)
	return b
}

// addAll adds each element of an erased Iterable as add does.
func (b *ImmutableList_Builder[T]) addAll(from Iterable) *ImmutableList_Builder[T] {
	if v, ok := from.(*tlistView_[T]); ok && v.c == b.c {
		return b.addAllT(v.tl())
	}
	if ic, ok := from.(*ImmutableCollection); ok && ic != nil {
		if l, ok := ic.typed_.v.(*ImmutableList[T]); ok && ic.typed_.conv == any(b.c) {
			return b.addAllT(l)
		}
	}
	for _, x := range readSlice_(from) {
		if x == nil {
			panic(new_NullPointerException_0().self())
		}
		if cap(b.items) == 0 {
			b.items = make([]T, 0, 4)
		}
		b.items = append(b.items, b.c.unbox(x))
	}
	return b
}

// addAllT is addAll of a typed list of the same element type.
func (b *ImmutableList_Builder[T]) addAllT(l *ImmutableList[T]) *ImmutableList_Builder[T] {
	for _, x := range l.items {
		b.add(x)
	}
	return b
}

func (b *ImmutableList_Builder[T]) build() *ImmutableList[T] {
	if len(b.items) == 0 {
		return tlistEmptyImm_(b.c)
	}
	return &TList[T]{items: b.items[:len(b.items):len(b.items)], c: b.c, imm: true}
}

// ---- sets

// tsetEmptyImm_ is ImmutableSet.of(): one empty set per element type, whose twin is the
// runtime's empty set.
func tsetEmptyImm_[K comparable](kc *tconv_[K]) *ImmutableSet[K] {
	if s, ok := kc.emptyS.(*ImmutableSet[K]); ok {
		return s
	}
	s := newTSet_(kc)
	s.imm = true
	s.ext().twin = emptyImmSet_
	kc.emptyS = s
	return s
}

// immutableSetOf_ is ImmutableSet.of(xs...): the distinct elements in first-seen order; a null
// element throws NullPointerException.
func immutableSetOf_[K comparable](kc *tconv_[K], xs ...K) *ImmutableSet[K] {
	for _, x := range xs {
		if immNull_(kc, x) {
			panic(new_NullPointerException_0().self()) // (as immSet_: no message)
		}
	}
	return tsetMake_(kc, xs)
}

// tsetMake_ is the immutable set of the distinct elements of xs (checked for null).
func tsetMake_[K comparable](kc *tconv_[K], xs []K) *ImmutableSet[K] {
	if len(xs) == 0 {
		return tsetEmptyImm_(kc)
	}
	s := newTSet_(kc)
	s.keys = make([]K, 0, len(xs))
	s.vals = make([]struct{}, 0, len(xs))
	for _, x := range xs {
		if s.find(x) < 0 {
			s.Put(x, struct{}{})
		}
	}
	s.imm = true
	return s
}

// immutableSetCopyOf_ is ImmutableSet.copyOf(from): an immutable set is itself, as a typed set; any
// other collection is copied (a null element throws NullPointerException).
func immutableSetCopyOf_[K comparable](kc *tconv_[K], from Iterable) *ImmutableSet[K] {
	if ic, ok := from.(*ImmutableCollection); ok && ic.kind == icSet_ {
		if ic == emptyImmSet_ {
			return tsetEmptyImm_(kc)
		}
		return tsetFrom_(kc, Collection(ic), true)
	}
	if v, ok := from.(*tsetView_[K]); ok && v.m.kc == kc {
		var xs []K
		for i := v.m.first(); i < len(v.m.keys); i++ {
			if !v.m.isDead(i) {
				xs = append(xs, v.m.keys[i])
			}
		}
		return immutableSetOf_(kc, xs...) // (its elements checked for null, as any other's)
	}
	src := copySlice_(from)
	xs := make([]K, len(src))
	for i, x := range src {
		if x == nil {
			panic(new_NullPointerException_0().self())
		}
		xs[i] = kc.unbox(x)
	}
	return tsetMake_(kc, xs)
}

// asImm is an immutable set as erased code sees it: its ErasedImmutableSet twin.
func (s *TSet[K]) asImm() *ErasedImmutableSet {
	if s == nil {
		return nil
	}
	x := s.ext()
	if x.twin == nil {
		if s.live == 0 {
			x.twin = emptyImmSet_
		} else {
			t := immSet_(boxAll_(s.kc, s.keys))
			t.typed_ = immTyped_{s.kc, s}
			x.twin = t
		}
	}
	return x.twin.(*ImmutableCollection)
}

// ImmutableSet_Builder is ImmutableSet.Builder<K> with unboxed elements: build() is the set of the
// distinct elements added so far, in first-seen order.
type ImmutableSet_Builder[K comparable] struct {
	items []K
	c     *tconv_[K]
}

func immutableSetBuilder_[K comparable](c *tconv_[K]) *ImmutableSet_Builder[K] {
	return &ImmutableSet_Builder[K]{c: c}
}

// immutableSetBuilderN_ is ImmutableSet.builderWithExpectedSize(n).
func immutableSetBuilderN_[K comparable](c *tconv_[K], n int32) *ImmutableSet_Builder[K] {
	return &ImmutableSet_Builder[K]{c: c, items: make([]K, 0, n)}
}

func (b *ImmutableSet_Builder[K]) add(x K) *ImmutableSet_Builder[K] {
	if immNull_(b.c, x) {
		panic(new_NullPointerException_0().self()) // (Guava's checkNotNull: no message)
	}
	if cap(b.items) == 0 {
		b.items = make([]K, 0, 4)
	}
	b.items = append(b.items, x)
	return b
}

// addAll adds each element of an erased Iterable as add does.
func (b *ImmutableSet_Builder[K]) addAll(from Iterable) *ImmutableSet_Builder[K] {
	for _, x := range readSlice_(from) {
		if x == nil {
			panic(new_NullPointerException_0().self())
		}
		if cap(b.items) == 0 {
			b.items = make([]K, 0, 4)
		}
		b.items = append(b.items, b.c.unbox(x))
	}
	return b
}

func (b *ImmutableSet_Builder[K]) build() *ImmutableSet[K] { return tsetMake_(b.c, b.items) }

// ---- maps

// tmapEmptyImm_ is ImmutableMap.of(): one empty map per key and value type, whose twin is the
// runtime's empty map.
func tmapEmptyImm_[K comparable, V any](kc *tconv_[K], vc *tconv_[V]) *ImmutableMap[K, V] {
	if m, ok := vc.emptyM.(*ImmutableMap[K, V]); ok && m.kc == kc {
		return m
	}
	if vc.emptyM != nil {
		// the value type has the empty map of other keys: this one is kept by both conversions
		emptyMaps_.mu.Lock()
		defer emptyMaps_.mu.Unlock()
		if m, ok := emptyMaps_.m[[2]any{kc, vc}].(*ImmutableMap[K, V]); ok {
			return m
		}
	}
	m := newTMap_(kc, vc)
	m.imm = true
	m.ext().twin = emptyImmMap_
	if vc.emptyM == nil {
		vc.emptyM = m
	} else {
		emptyMaps_.m[[2]any{kc, vc}] = m
	}
	return m
}

// emptyMaps_ holds the empty immutable maps of a value type that has more than one (maps of
// several key types to it), by key and value conversion: ImmutableMap.of() is one object.
var emptyMaps_ = struct {
	mu sync.Mutex
	m  map[[2]any]any
}{m: map[[2]any]any{}}

// immutableMapOf_ is ImmutableMap.of(k1, v1, ...) of the keys and values, which it owns: a null key
// or value throws NullPointerException, a duplicate key IllegalArgumentException.
func immutableMapOf_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], keys []K, vals []V) *ImmutableMap[K, V] {
	return tmapMake_(kc, vc, keys, vals, false)
}

// tmapMake_ is the immutable map of parallel keys and values, as immMap_ makes an ImmutableMap:
// nulls are checked first, then duplicate keys throw (immDup_'s message), or with keepLast a key
// put more than once keeps its first position and has its last value.
func tmapMake_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], keys []K, vals []V, keepLast bool) *ImmutableMap[K, V] {
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
	m.keys = make([]K, 0, len(keys))
	m.vals = make([]V, 0, len(keys))
	for i, k := range keys {
		if j := m.find(k); j >= 0 {
			if !keepLast {
				immDup_(boxAll_(kc, keys), boxAll_(vc, vals))
			}
			m.vals[j] = vals[i]
			continue
		}
		m.Put(k, vals[i])
	}
	m.imm = true
	return m
}

// immutableMapCopyOf_ is ImmutableMap.copyOf(from): an immutable map (not a sorted one) is itself, as a
// typed map; any other is copied, its nulls and duplicate keys checked as ImmutableMap.of does.
func immutableMapCopyOf_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], from Map) *ImmutableMap[K, V] {
	if im, ok := from.(*ErasedImmutableMap); ok && im.kind != imSorted_ {
		if im == emptyImmMap_ {
			return tmapEmptyImm_(kc, vc)
		}
		return tmapFrom_(kc, vc, Map(im), true)
	}
	if v, ok := from.(*tmapView_[K, V]); ok && v.m.kc == kc && v.m.vc == vc {
		var keys []K
		var vals []V
		for i := v.m.first(); i < len(v.m.keys); i++ {
			if !v.m.isDead(i) {
				keys = append(keys, v.m.keys[i])
				vals = append(vals, v.m.vals[i])
			}
		}
		return tmapMake_(kc, vc, keys, vals, false)
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
	return tmapMake_(kc, vc, keys, vals, false)
}

// asImm is an immutable map as erased code sees it: its ErasedImmutableMap twin.
func (m *TMap[K, V]) asImm() *ErasedImmutableMap {
	if m == nil {
		return nil
	}
	x := m.ext()
	if x.twin == nil {
		if m.live == 0 {
			x.twin = emptyImmMap_
		} else {
			keys, vals := boxAll_(m.kc, m.keys), boxAll_(m.vc, m.vals)
			t := &ErasedImmutableMap{kind: imMap_, keys: keys, vals: vals}
			t.typed_ = immTyped_{m.vc, m}
			if x.keySet != nil {
				// its key set is the one that typed code has
				t.keySet_ = x.keySet.asImm()
			}
			x.twin = t
		}
	}
	return x.twin.(*ErasedImmutableMap)
}

// ImmutableMap_Builder is ImmutableMap.Builder<K, V> with unboxed keys and values.
type ImmutableMap_Builder[K comparable, V any] struct {
	keys []K
	vals []V
	kc   *tconv_[K]
	vc   *tconv_[V]
}

func immutableMapBuilder_[K comparable, V any](kc *tconv_[K], vc *tconv_[V]) *ImmutableMap_Builder[K, V] {
	return &ImmutableMap_Builder[K, V]{kc: kc, vc: vc}
}

func (b *ImmutableMap_Builder[K, V]) put(k K, v V) *ImmutableMap_Builder[K, V] {
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

// putAll puts each entry of an erased Map as put does.
func (b *ImmutableMap_Builder[K, V]) putAll(from Map) *ImmutableMap_Builder[K, V] {
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

func (b *ImmutableMap_Builder[K, V]) build() *ImmutableMap[K, V] { return b.buildOrThrow() }

func (b *ImmutableMap_Builder[K, V]) buildOrThrow() *ImmutableMap[K, V] {
	return tmapMake_(b.kc, b.vc, b.keys, b.vals, false)
}

func (b *ImmutableMap_Builder[K, V]) buildKeepingLast() *ImmutableMap[K, V] {
	return tmapMake_(b.kc, b.vc, b.keys, b.vals, true)
}

// ---- typed copies of typed immutable collections (step colls -kind imm*: a value entering a
// typed class of an earlier step without going through its twin)

// immItems_ is sliceImm_(c, l) of a typed immutable list: its items (nil for a nil list).
func immItems_[T any](l *ImmutableList[T]) []T {
	if l == nil {
		return nil
	}
	return l.items
}

// tlistCopyT_ is new ArrayList<>(l) of a typed list l (a nil l throws NullPointerException).
func tlistCopyT_[T any](c *tconv_[T], l *TList[T]) *TList[T] {
	return &TList[T]{c: c, items: append(make([]T, 0, len(l.items)), l.items...)}
}

// tsetCopyT_ is new LinkedHashSet<>(s) of a typed set s.
func tsetCopyT_[K comparable](kc *tconv_[K], s *TSet[K]) *TSet[K] {
	out := newTSet_(kc)
	for i := s.first(); i < len(s.keys); i++ {
		if !s.isDead(i) {
			out.Add(s.keys[i])
		}
	}
	return out
}

// tmapCopyT_ is new LinkedHashMap<>(m) of a typed map m.
func tmapCopyT_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], m *TMap[K, V]) *TMap[K, V] {
	out := newTMap_(kc, vc)
	for i := m.first(); i < len(m.keys); i++ {
		if !m.isDead(i) {
			out.Put(m.keys[i], m.vals[i])
		}
	}
	return out
}

// ---- equals and hashCode of typed immutable collections, as the erased ones compute them
// (rt_guava.go, rt_guava_map.go): obj is Objects.equals / Objects.hashCode (null is equal to
// null, hashes to 0), else a.equals(b) / a.hashCode() (a null a throws NullPointerException).

// elemEq_ is Java's x.equals(y) of two non-null elements.
func elemEq_[T any](c *tconv_[T], x, y T) bool {
	if p, ok := any(&x).(*string); ok {
		return *p == *any(&y).(*string)
	}
	if c.eq != nil {
		return c.eq(x, y)
	}
	return objequals_(c.box(x), c.box(y))
}

// elemHash_ is Java's x.hashCode() of a non-null element.
func elemHash_[T any](c *tconv_[T], x T) int32 {
	if p, ok := any(&x).(*string); ok {
		return String_hashCode(*p)
	}
	if c.hash != nil {
		return c.hash(x)
	}
	return objhash_(c.box(x))
}

func immNulls_(aNil, bNil, obj bool) (r, done bool) {
	if aNil && !obj {
		panic(new_NullPointerException_0().self())
	}
	if aNil || bNil {
		return aNil && bNil, true
	}
	return false, false
}

func tlistEq_[T any](a, b *TList[T], obj bool) bool {
	if r, done := immNulls_(a == nil, b == nil, obj); done {
		return r
	}
	if a == b {
		return true
	}
	if len(a.items) != len(b.items) {
		return false
	}
	for i, x := range a.items {
		if !elemEq_(a.c, x, b.items[i]) {
			return false
		}
	}
	return true
}

func tlistHash_[T any](a *TList[T], obj bool) int32 {
	if a == nil {
		if obj {
			return 0
		}
		panic(new_NullPointerException_0().self())
	}
	var h int32 = 1
	for _, x := range a.items {
		h = 31*h + elemHash_(a.c, x)
	}
	return h
}

func tsetEq_[K comparable](a, b *TSet[K], obj bool) bool {
	if r, done := immNulls_(a == nil, b == nil, obj); done {
		return r
	}
	if a == b {
		return true
	}
	if a.live != b.live {
		return false
	}
	for i := b.first(); i < len(b.keys); i++ {
		if !b.isDead(i) && a.find(b.keys[i]) < 0 {
			return false
		}
	}
	return true
}

func tsetHash_[K comparable](a *TSet[K], obj bool) int32 {
	if a == nil {
		if obj {
			return 0
		}
		panic(new_NullPointerException_0().self())
	}
	if a.imm && a.x != nil && a.x.hashed {
		return a.x.hash
	}
	var h int32
	for i := a.first(); i < len(a.keys); i++ {
		if !a.isDead(i) {
			h += elemHash_(a.kc, a.keys[i])
		}
	}
	if a.imm {
		// An immutable set's hash code never changes: computed once, as the erased set keeps it.
		x := a.ext()
		x.hash, x.hashed = h, true
	}
	return h
}

func tmapEq_[K comparable, V any](a, b *TMap[K, V], obj bool) bool {
	if r, done := immNulls_(a == nil, b == nil, obj); done {
		return r
	}
	if a == b {
		return true
	}
	if a.live != b.live {
		return false
	}
	for i := a.first(); i < len(a.keys); i++ {
		if a.isDead(i) {
			continue
		}
		j := b.find(a.keys[i])
		if j < 0 || !elemEq_(a.vc, a.vals[i], b.vals[j]) {
			return false
		}
	}
	return true
}

func tmapHash_[K comparable, V any](a *TMap[K, V], obj bool) int32 {
	if a == nil {
		if obj {
			return 0
		}
		panic(new_NullPointerException_0().self())
	}
	var h int32
	for i := a.first(); i < len(a.keys); i++ {
		if !a.isDead(i) {
			h += elemHash_(a.kc, a.keys[i]) ^ elemHash_(a.vc, a.vals[i])
		}
	}
	return h
}

// ---- builds for erased code: a builder whose collection goes to erased code builds the
// erased collection itself (the elements boxed once), as the erased builder built it.

func (b *ImmutableList_Builder[T]) buildErased() *ErasedImmutableList {
	return immList_(boxAll_(b.c, b.items))
}

func (b *ImmutableSet_Builder[K]) buildErased() *ErasedImmutableSet {
	return immSet_(boxAll_(b.c, b.items))
}

func (b *ImmutableMap_Builder[K, V]) buildErased() *ErasedImmutableMap {
	return immMap_(imMap_, boxAll_(b.kc, b.keys), boxAll_(b.vc, b.vals), nil, false)
}

func (b *ImmutableMap_Builder[K, V]) buildOrThrowErased() *ErasedImmutableMap { return b.buildErased() }

func (b *ImmutableMap_Builder[K, V]) buildKeepingLastErased() *ErasedImmutableMap {
	return immMap_(imMap_, boxAll_(b.kc, b.keys), boxAll_(b.vc, b.vals), nil, true)
}

// ---- what a typed declaration of the translator (j2go rule "typed collections") needs beside
// the above: copyOf of a typed collection, an erased immutable set or map entering a typed
// declaration, ImmutableMap.of with its arguments in Java's order, keySet() of an immutable map.

func classCastImm_(from, to string) {
	panic(new_ClassCastException_String("class " + from + " cannot be cast to class " + to).self())
}

// immutableListCopyOfT_ is ImmutableList.copyOf(l) of a typed list of the same element type:
// an immutable list is itself; any other is copied (a null element throws
// NullPointerException).
func immutableListCopyOfT_[T any](c *tconv_[T], l *TList[T]) *ImmutableList[T] {
	if l.imm && !l.unmod {
		return l
	}
	return immutableListCopyOfArr_(c, l.items)
}

// liveKV is the keys and values of the map in order (the map's own slices if nothing was
// removed).
func (m *TMap[K, V]) liveKV() ([]K, []V) {
	if m.x == nil || m.x.dead == nil {
		return m.keys, m.vals
	}
	keys, vals := make([]K, 0, m.live), make([]V, 0, m.live)
	for i := m.first(); i < len(m.keys); i++ {
		if !m.x.dead[i] {
			keys, vals = append(keys, m.keys[i]), append(vals, m.vals[i])
		}
	}
	return keys, vals
}

// immutableSetCopyOfT_ is ImmutableSet.copyOf(s) of a typed set of the same element type: an
// immutable set (not a sorted one) is itself.
func immutableSetCopyOfT_[K comparable](kc *tconv_[K], s *TSet[K]) *ImmutableSet[K] {
	if s.imm && !s.sorted() {
		return s
	}
	keys, _ := s.liveKV()
	return immutableSetOf_(kc, keys...)
}

// immutableMapCopyOfT_ is ImmutableMap.copyOf(m) of a typed map of the same types: an
// immutable map (not a sorted one) is itself.
func immutableMapCopyOfT_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], m *TMap[K, V]) *ImmutableMap[K, V] {
	if m.imm && !m.sorted() {
		return m
	}
	keys, vals := m.liveKV()
	return tmapMake_(kc, vc, keys, vals, false)
}

// tsetImm_ is an ImmutableSet that erased code made, entering a typed declaration: the typed
// set of its twin ic, made once (nil for null; tsetFrom_). An ImmutableSortedSet is a sorted
// typed set with its comparator, the elements as they are ordered.
func tsetImm_[K comparable](kc *tconv_[K], ic *ImmutableCollection) *ImmutableSet[K] {
	if ic == nil {
		return nil
	}
	if ic.kind == icList_ {
		classCastImm_("com.google.common.collect.ErasedImmutableList", "com.google.common.collect.ErasedImmutableSet")
	}
	if ic == emptyImmSet_ {
		return tsetEmptyImm_(kc)
	}
	if ic.kind != icSorted_ {
		return tsetFrom_(kc, Collection(ic), true)
	}
	if ic.typed_.conv == any(kc) {
		if s, ok := ic.typed_.v.(*TSet[K]); ok {
			return s
		}
	}
	s := newTSortedSetC_(kc, ic.cmp_())
	s.keys = unboxAll_(kc, ic.items)
	s.vals = make([]struct{}, len(s.keys))
	s.live = int32(len(s.keys))
	s.imm = true
	s.x.twin = ic
	ic.typed_ = immTyped_{kc, s}
	return s
}

// tmapImm_ is an ImmutableMap that erased code made, entering a typed declaration (see
// tsetImm_; tmapFrom_). An ImmutableSortedMap is a sorted typed map with its comparator.
func tmapImm_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], im *ErasedImmutableMap) *ImmutableMap[K, V] {
	if im == nil {
		return nil
	}
	if im == emptyImmMap_ {
		return tmapEmptyImm_(kc, vc)
	}
	if im.kind != imSorted_ {
		return tmapFrom_(kc, vc, Map(im), true)
	}
	if im.typed_.conv == any(vc) {
		if m, ok := im.typed_.v.(*TMap[K, V]); ok && m.kc == kc {
			return m
		}
	}
	m := newTMap_(kc, vc)
	m.setSorted_(im.cmp)
	m.keys, m.vals = unboxAll_(kc, im.keys), unboxAll_(vc, im.vals)
	m.live = int32(len(m.keys))
	m.imm = true
	m.x.twin = im
	im.typed_ = immTyped_{vc, m}
	return m
}

// immutableMapOf1_, immutableMapOf3_ and immutableMapOf4_ are ImmutableMap.of(k1, v1, ...): the
// arguments in Java's order (immutableMapOf_ takes the keys, then the values).
func immutableMapOf1_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], k1 K, v1 V) *ImmutableMap[K, V] {
	return tmapMake_(kc, vc, []K{k1}, []V{v1}, false)
}

func immutableMapOf3_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], k1 K, v1 V, k2 K, v2 V, k3 K, v3 V) *ImmutableMap[K, V] {
	return tmapMake_(kc, vc, []K{k1, k2, k3}, []V{v1, v2, v3}, false)
}

func immutableMapOf4_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], k1 K, v1 V, k2 K, v2 V, k3 K, v3 V, k4 K, v4 V) *ImmutableMap[K, V] {
	return tmapMake_(kc, vc, []K{k1, k2, k3, k4}, []V{v1, v2, v3, v4}, false)
}

// keySetImm is keySet() of an immutable map (an ImmutableSet<K>) for a typed declaration: one
// typed set, the same object as the key set that erased code gets from the map's twin.
func (m *TMap[K, V]) keySetImm() *ImmutableSet[K] {
	if !m.imm {
		classCastImm_("java.util.LinkedHashMap", "com.google.common.collect.ErasedImmutableMap")
	}
	x := m.ext()
	if x.keySet == nil {
		if x.twin != nil {
			// the key set of the twin, as a typed set
			x.keySet = tsetImm_(m.kc, x.twin.(*ErasedImmutableMap).keySet().(*ImmutableCollection))
		} else {
			x.keySet = tsetMake_(m.kc, m.keys)
		}
	}
	return x.keySet
}

// ---- a typed collection through its erased form and back (post-flip step trips): what the
// pair of conversions gave, without the erased view or twin in between. Where the shortcut
// would not be what the conversions do (a list whose element conversion is another object
// than the caller's, or whose twin has been given another typed copy), the helper does the
// two conversions themselves.

// tlistOwnTwin_ reports whether sliceImm_ and tlistImm_ of l.asImm() at conversion c find l
// itself: l converts by c and its twin, if it has one yet, names l as its typed copy.
func tlistOwnTwin_[T any](c *tconv_[T], l *TList[T]) bool {
	if l.c != c {
		return false
	}
	if l.twin == nil {
		return true // asImm makes the twin of l, with l as its typed copy
	}
	t := l.twin
	return t.typed_.conv == any(c) && t.typed_.v == any(l)
}

// tlistSlice_ is sliceImm_(c, l.asImm()) of a typed list l: its elements as a slice that
// nothing changes (nil for a nil list, an empty slice for an empty one).
func tlistSlice_[T any](c *tconv_[T], l *TList[T]) []T {
	if l == nil {
		return nil
	}
	if len(l.items) == 0 {
		return []T{}
	}
	if !tlistOwnTwin_(c, l) {
		return sliceImm_(c, Collection(l.asImm()))
	}
	return l.items
}

// tlistImmCopy_ is ImmutableList.copyOf(l.asList()) of a typed list l: l itself when it is
// immutable, else a null-checked copy.
func tlistImmCopy_[T any](c *tconv_[T], l *TList[T]) *ImmutableList[T] {
	if l.imm {
		if len(l.items) == 0 {
			return tlistEmptyImm_(c)
		}
		if tlistOwnTwin_(c, l) {
			return l
		}
	} else if l.c == c {
		return immutableListCopyOfArr_(c, l.items)
	}
	return immutableListCopyOf_(c, Collection(l.asList()))
}

// tsetAddAllOf_ is s.asSet().addAll(c): the elements of c, unboxed, added to s in c's order.
func tsetAddAllOf_[K comparable](s *TSet[K], c Collection) bool {
	if s.imm {
		throwUOE_()
	}
	changed := false
	for _, x := range toSlice_(c) {
		if s.Add(s.kc.unbox(x)) {
			changed = true
		}
	}
	return changed
}

// tsetAddSet_ is s.asSet().addAll(t.asSet()) (or t.asImm()) of two typed sets.
func tsetAddSet_[K comparable](s, t *TSet[K]) bool {
	if s.imm {
		throwUOE_()
	}
	return s.AddAll(t)
}

// tsetAddList_ is s.asSet().addAll(t.asList()) (or t.asImm()) of a typed set and a typed list.
func tsetAddList_[K comparable](s *TSet[K], t *TList[K]) bool {
	if s.imm {
		throwUOE_()
	}
	changed := false
	for _, k := range t.items {
		if s.Add(k) {
			changed = true
		}
	}
	return changed
}

// sliceAddSlice_ is sliceAddAll_(c, p, sliceView_(c, xs)): the elements of xs appended to the
// list p points to; a nil xs is a null list (NullPointerException, as its nil view gave).
func sliceAddSlice_[T any](p *[]T, xs []T) bool {
	if xs == nil {
		panic(new_NullPointerException_0().self())
	}
	if len(xs) == 0 {
		return false
	}
	*p = append(*p, xs...)
	return true
}

// sliceAddList_ is sliceAddAll_(c, p, l.asList()) (or l.asImm()) of a typed list l.
func sliceAddList_[T any](p *[]T, l *TList[T]) bool {
	xs := l.items
	if len(xs) == 0 {
		return false
	}
	*p = append(*p, xs...)
	return true
}

// sliceSkip_ is ImmutableList.copyOf(Iterables.skip(list of s, n)) as a slice that nothing
// changes: s without its first n elements.
func sliceSkip_[T any](s []T, n int32) []T {
	if n < 0 {
		throwIAE_("number to skip cannot be negative")
	}
	if int(n) >= len(s) {
		return []T{}
	}
	return s[n:len(s):len(s)]
}
