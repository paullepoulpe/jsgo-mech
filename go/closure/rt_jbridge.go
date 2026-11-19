package closure

// What the typed collections the translator writes use and the post-flip typed collections
// (rt_tlist.go, rt_tmap.go, rt_timm.go) did not have, for post-flip tool jfam: with these a
// *JTList[T] is a *TList[T]. Each is the translator's function on the post-flip representation:
// the same results, exceptions and iteration order.

// immutableListCopyOfTP_ is ImmutableList.copyOf(l) of a typed list of the same element type:
// an immutable list is itself; any other is copied (a null element throws
// NullPointerException). (The translator's immutableListCopyOfT_.)
func immutableListCopyOfTP_[T any](c *tconv_[T], l *TList[T]) *TList[T] {
	if l.imm && !l.unmod {
		return l
	}
	return immutableListCopyOfArr_(c, l.items)
}

// splitToListTP_ is Splitter.splitToList for a typed declaration: the parts as they are, in a
// list that cannot be changed and is no ImmutableList, as Guava's. (The translator's
// splitToListT_.)
func splitToListTP_(c *tconv_[string], sp *Splitter, seq any) *TList[string] {
	return &TList[string]{c: c, items: sp.splitStrs_(csString_(seq)), imm: true, unmod: true}
}

// newTSetCap_ is new LinkedHashSet<>(n), newTMapCap_ new LinkedHashMap<>(n): n is evaluated and
// not used, as in the translator's newTSetN_ and newTMapN_.
func newTSetCap_[K comparable](c *tconv_[K], n int32) *TSet[K] { return newTSet_(c) }

func newTMapCap_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], n int32) *TMap[K, V] {
	return newTMap_(kc, vc)
}

// tmapFromMap_ is new LinkedHashMap<>(from): NullPointerException for a null map.
func tmapFromMap_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], from Map) *TMap[K, V] {
	m := newTMap_(kc, vc)
	if from == nil {
		panic(new_NullPointerException_0().self())
	}
	m.asMap().putAll(from)
	return m
}

// iterator is the set's Iterator (of boxed elements, with remove), as its view's; a null set
// throws NullPointerException.
func (s *TSet[K]) iterator() Iterator {
	if s == nil {
		panic(new_NullPointerException_0().self())
	}
	return s.asSet().iterator()
}

// putAllT is putAll of another typed map of the same types.
func (m *TMap[K, V]) putAllT(o *TMap[K, V]) {
	if m.imm { // ImmutableMap.putAll throws whatever its argument
		throwUOE_()
	}
	for i, k := range o.keys {
		if !o.isDead(i) {
			m.Put(k, o.vals[i])
		}
	}
}

// tmapKeysP_ is the cursor of a for-each loop over the keys of a typed map, tmapValsP_ over its
// values, tmapEntriesP_ over its entries: the mappings in insertion order, read position by
// position as the erased cursor reads its table.
type tmapKeysP_[K comparable, V any] struct {
	m *TMap[K, V]
	i int
	v K
}

// eachKey is the cursor of a for-each loop over keySet(); a null map throws
// NullPointerException here, where the loop asked for the key set.
func (m *TMap[K, V]) eachKey() tmapKeysP_[K, V] {
	_ = m.live
	return tmapKeysP_[K, V]{m: m}
}

func (e *tmapKeysP_[K, V]) next() bool {
	m := e.m
	for e.i < len(m.keys) {
		i := e.i
		e.i++
		if !m.isDead(i) {
			e.v = m.keys[i]
			return true
		}
	}
	return false
}

type tmapValsP_[K comparable, V any] struct {
	m *TMap[K, V]
	i int
	v V
}

// eachValue is the cursor of a for-each loop over values().
func (m *TMap[K, V]) eachValue() tmapValsP_[K, V] {
	_ = m.live
	return tmapValsP_[K, V]{m: m}
}

func (e *tmapValsP_[K, V]) next() bool {
	m := e.m
	for e.i < len(m.keys) {
		i := e.i
		e.i++
		if !m.isDead(i) {
			e.v = m.vals[i]
			return true
		}
	}
	return false
}

type tmapEntriesP_[K comparable, V any] struct {
	m *TMap[K, V]
	i int
}

// eachEntry is the cursor of a for-each loop over entrySet().
func (m *TMap[K, V]) eachEntry() tmapEntriesP_[K, V] {
	_ = m.live
	return tmapEntriesP_[K, V]{m: m}
}

func (e *tmapEntriesP_[K, V]) next() bool {
	m := e.m
	for e.i < len(m.keys) {
		e.i++
		if !m.isDead(e.i - 1) {
			return true
		}
	}
	return false
}

// entry is the entry the cursor is at: an object of its own, as each entry of the Java map is.
func (e *tmapEntriesP_[K, V]) entry() *TEntryP[K, V] {
	i := e.i - 1
	return &TEntryP[K, V]{m: e.m, i: int32(i), k: e.m.keys[i], v: e.m.vals[i]}
}

// TEntryP is an entry of a typed map (Map.Entry<K, V>) in a for-each loop over its entries (the
// translator's TEntry on the post-flip map). It reads and writes the value of its mapping while
// the map has it; after the mapping's removal it keeps the last value, as the entry of the
// erased map does.
type TEntryP[K comparable, V any] struct {
	m *TMap[K, V]
	i int32
	k K
	v V
}

// pos is the position of the entry's mapping in the map, or -1 if the map has it no more.
func (e *TEntryP[K, V]) pos() int32 {
	m := e.m
	if i := int(e.i); i < len(m.keys) && m.keys[i] == e.k && !m.isDead(i) {
		return e.i
	}
	return m.find(e.k)
}

func (e *TEntryP[K, V]) getKey() K { return e.k }

func (e *TEntryP[K, V]) getValue() V {
	if i := e.pos(); i >= 0 {
		e.v = e.m.vals[i]
	}
	return e.v
}

func (e *TEntryP[K, V]) setValue(v V) V {
	old := e.getValue()
	e.v = v
	if i := e.pos(); i >= 0 {
		e.m.vals[i] = v
	}
	return old
}

// asEntry is the entry as an erased Map.Entry (nil for null).
func (e *TEntryP[K, V]) asEntry() Map_Entry {
	if e == nil {
		return nil
	}
	return &tentryViewP_[K, V]{e: e}
}

// tentryViewP_ is a TEntryP seen as an erased Map.Entry.
type tentryViewP_[K comparable, V any] struct {
	e *TEntryP[K, V]
}

func (v *tentryViewP_[K, V]) self() any     { return v }
func (v *tentryViewP_[K, V]) getKey() any   { return v.e.m.kc.box(v.e.k) }
func (v *tentryViewP_[K, V]) getValue() any { return v.e.m.vc.box(v.e.getValue()) }
func (v *tentryViewP_[K, V]) setValue(x any) any {
	return v.e.m.vc.box(v.e.setValue(v.e.m.vc.unbox(x)))
}
func (v *tentryViewP_[K, V]) equals(o any) bool {
	x, ok := o.(Map_Entry)
	return ok && x != nil && objequals_(v.getKey(), x.getKey()) && objequals_(v.getValue(), x.getValue())
}
func (v *tentryViewP_[K, V]) hashCode() int32 { return objhash_(v.getKey()) ^ objhash_(v.getValue()) }
func (v *tentryViewP_[K, V]) toString() string {
	return objstr_(v.getKey()) + "=" + objstr_(v.getValue())
}

// immutableSetCopyOfTP_ is ImmutableSet.copyOf(s) of a typed set of the same element type: an
// immutable set is itself; any other is copied. (The translator's immutableSetCopyOfT_.)
func immutableSetCopyOfTP_[K comparable](c *tconv_[K], s *TSet[K]) *TSet[K] {
	if s.imm {
		return s
	}
	return immutableSetCopyOf_(c, Iterable(s.asSet()))
}

// immutableMapCopyOfTP_ is ImmutableMap.copyOf(m) of a typed map of the same types. (The
// translator's immutableMapCopyOfT_.)
func immutableMapCopyOfTP_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], m *TMap[K, V]) *TMap[K, V] {
	if m.imm {
		return m
	}
	return immutableMapCopyOf_(kc, vc, m.asMap())
}

// immutableMapOf1P_ ... immutableMapOf5P_ are ImmutableMap.of(k1, v1, ...): the translator's
// immutableMapOf1_ ... immutableMapOf5_ (duplicate keys throw IllegalArgumentException).
func immutableMapOf1P_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], k1 K, v1 V) *TMap[K, V] {
	return immutableMapOf_(kc, vc, []K{k1}, []V{v1})
}
func immutableMapOf2P_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], k1 K, v1 V, k2 K, v2 V) *TMap[K, V] {
	return immutableMapOf_(kc, vc, []K{k1, k2}, []V{v1, v2})
}
func immutableMapOf3P_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], k1 K, v1 V, k2 K, v2 V, k3 K, v3 V) *TMap[K, V] {
	return immutableMapOf_(kc, vc, []K{k1, k2, k3}, []V{v1, v2, v3})
}
func immutableMapOf4P_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], k1 K, v1 V, k2 K, v2 V, k3 K, v3 V, k4 K, v4 V) *TMap[K, V] {
	return immutableMapOf_(kc, vc, []K{k1, k2, k3, k4}, []V{v1, v2, v3, v4})
}
func immutableMapOf5P_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], k1 K, v1 V, k2 K, v2 V, k3 K, v3 V, k4 K, v4 V, k5 K, v5 V) *TMap[K, V] {
	return immutableMapOf_(kc, vc, []K{k1, k2, k3, k4, k5}, []V{v1, v2, v3, v4, v5})
}

// forEachT is Map.forEach of a typed map with the action as a Go function of its key and value
// types (post-flip tool bulkops): the mappings in iteration order, each read when it is reached,
// as Map_forEach reads them through the entry set's iterator.
func (m *TMap[K, V]) forEachT(f func(K, V)) {
	for i := m.first(); i < len(m.keys); i++ {
		if !m.isDead(i) {
			f(m.keys[i], m.vals[i])
		}
	}
}

// forEachT is Iterable.forEach of a typed list with the action as a Go function of its element
// type: element i is read when it is reached, as the list's iterator reads it.
func (l *TList[T]) forEachT(f func(T)) {
	for i := 0; i < len(l.items); i++ {
		f(l.items[i])
	}
}
