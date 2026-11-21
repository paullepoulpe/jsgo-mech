package closure

// Typed maps (j2go rule "typed collections"): a LinkedHashMap<K, V> that the translator finds
// typable is a *TMap[K, V]: its keys and values in insertion order, as K and V values, not
// boxes. K is a type whose Go == is Java's equals for the map (a String, a handle, an enum, a
// class without equals of its own); V is any type with one Go type.
//
// Erased code sees the map through asMap(), its one view (see TList). Like the erased table
// (htable_) and the typed set, a small map is searched linearly and has no index, and a
// removed mapping leaves a dead position until the map is compacted, so that a cursor keeps
// its place.

type TMap[K comparable, V any] struct {
	keys []K
	vals []V         // beside keys
	gone []bool      // beside keys once a mapping has been removed: the dead positions
	idx  map[K]int32 // the position of each key; nil while the map is small
	live int32
	kc   *tconv_[K]
	vc   *tconv_[V]
	eq   *teq_           // of keys compared by equals and hashCode (kc.byEquals); else nil
	view tmapView_[K, V] // the map as erased code sees it (asMap): part of the map
	// imm: nil for a hash map that can change; an immutable map (Guava's ImmutableMap,
	// rt_timm.go) has here what it has beside its mappings: the ImmutableMap that erased code
	// sees it as and its typed key set, each once it is needed (immPendingMap_ until then). A
	// sorted map that can change (a TreeMap, rt_tsorted.go) has sortedMapMark_.
	imm *timmMap_
}

// newTMap_ is new LinkedHashMap<>().
func newTMap_[K comparable, V any](kc *tconv_[K], vc *tconv_[V]) *TMap[K, V] {
	m := &TMap[K, V]{kc: kc, vc: vc}
	m.view.m = m
	return m
}

// newTMapN_ is new LinkedHashMap<>(n): the capacity is not used.
func newTMapN_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], n int32) *TMap[K, V] {
	return newTMap_(kc, vc)
}

// tmapFrom_ is new LinkedHashMap<>(from).
func tmapFrom_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], from Map) *TMap[K, V] {
	m := newTMap_(kc, vc)
	if from == nil {
		panic(new_NullPointerException_0().self())
	}
	m.asMap().putAll(from)
	return m
}

// tmapCopy_ is new LinkedHashMap<>(from) of another typed map of the same types.
func tmapCopy_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], from *TMap[K, V]) *TMap[K, V] {
	m := newTMap_(kc, vc)
	m.putAllT(from)
	return m
}

func (m *TMap[K, V]) dead(i int) bool { return m.gone != nil && m.gone[i] }

// find returns the position of key k, or -1.
func (m *TMap[K, V]) find(k K) int32 {
	if m.eq != nil {
		// compared by equals and hashCode, or sorted
		if m.eq.sorted {
			if i, ok := tsortSearch_(m.eq, m.kc, m.keys, k); ok && !m.dead(i) {
				return int32(i)
			}
			return -1
		}
		return teqFind_(m.eq, m.keys, m.gone, m.kc, k, objhash_(m.kc.box(k)))
	}
	if m.idx != nil {
		if i, ok := m.idx[k]; ok {
			return i
		}
		return -1
	}
	for i, x := range m.keys {
		if x == k && !m.dead(i) {
			return int32(i)
		}
	}
	return -1
}

func (m *TMap[K, V]) size() int32   { return m.live }
func (m *TMap[K, V]) isEmpty() bool { return m.live == 0 }

// has is containsKey.
func (m *TMap[K, V]) containsKey(k K) bool { return m.find(k) >= 0 }

// get is the value of key k, or null.
func (m *TMap[K, V]) get(k K) V {
	if i := m.find(k); i >= 0 {
		return m.vals[i]
	}
	return m.vc.null
}

// getOr is getOrDefault.
func (m *TMap[K, V]) getOrDefault(k K, def V) V {
	if i := m.find(k); i >= 0 {
		return m.vals[i]
	}
	return def
}

// locate returns the position of key k, or -1, and the key's hash code if the map compares its
// keys by equals and hashCode (for append).
func (m *TMap[K, V]) locate(k K) (i int32, h int32) {
	if m.eq != nil && m.eq.sorted {
		// h: the position at which append puts the key
		p, ok := tsortSearch_(m.eq, m.kc, m.keys, k)
		if ok && !m.dead(p) {
			return int32(p), 0
		}
		return -1, int32(p)
	}
	if m.kc.byEquals {
		h = objhash_(m.kc.box(k))
		return teqFind_(m.eq, m.keys, m.gone, m.kc, k, h), h
	}
	return m.find(k), 0
}

// put returns the value that key k had, or null.
func (m *TMap[K, V]) put(k K, v V) V {
	if m.imm != nil && m.imm != sortedMapMark_ {
		panic(frozen_{})
	}
	i, h := m.locate(k)
	if i >= 0 {
		old := m.vals[i]
		m.vals[i] = v
		return old
	}
	m.append(k, v, h)
	return m.vc.null
}

// append adds a mapping for a key that the map does not have (h: see locate).
func (m *TMap[K, V]) append(k K, v V, h int32) {
	if m.eq != nil && m.eq.sorted {
		m.insertSorted(k, v, int(h))
		return
	}
	if n := len(m.keys) - int(m.live); n > hsmall_ && n > int(m.live) {
		m.compact()
	}
	if m.keys == nil {
		m.keys = make([]K, 0, hfirst_)
		m.vals = make([]V, 0, hfirst_)
	}
	m.keys = append(m.keys, k)
	m.vals = append(m.vals, v)
	if m.gone != nil {
		m.gone = append(m.gone, false)
	}
	m.live++
	if m.kc.byEquals {
		if m.eq == nil {
			m.eq = &teq_{}
		}
		m.eq.add(h)
	} else if m.idx != nil {
		m.idx[k] = int32(len(m.keys) - 1)
	} else if len(m.keys) > hsmall_ {
		m.idx = make(map[K]int32, 2*len(m.keys))
		for i, x := range m.keys {
			if !m.dead(i) {
				m.idx[x] = int32(i)
			}
		}
	}
}

// isNull reports whether v is Java's null.
func (m *TMap[K, V]) isNull(v V) bool { return immNull_(m.vc, v) }

// putIfAbsent maps k to v unless k has a value that is not null, which it returns.
func (m *TMap[K, V]) putIfAbsent(k K, v V) V {
	if m.imm != nil && m.imm != sortedMapMark_ {
		panic(frozen_{})
	}
	i, h := m.locate(k)
	if i >= 0 {
		old := m.vals[i]
		if m.isNull(old) {
			m.vals[i] = v
		}
		return old
	}
	m.append(k, v, h)
	return m.vc.null
}

// del is remove(key): the value the key had, or null.
func (m *TMap[K, V]) remove(k K) V {
	if m.imm != nil && m.imm != sortedMapMark_ {
		panic(frozen_{})
	}
	i := m.find(k)
	if i < 0 {
		return m.vc.null
	}
	old := m.vals[i]
	m.delAt(int(i))
	return old
}

func (m *TMap[K, V]) delAt(i int) {
	if m.imm != nil && m.imm != sortedMapMark_ {
		// (through a view of an immutable map that typed code holds: valuesT)
		panic(frozen_{})
	}
	if m.idx != nil {
		delete(m.idx, m.keys[i])
	}
	if m.gone == nil {
		m.gone = make([]bool, len(m.keys), cap(m.keys))
	}
	m.gone[i] = true
	var zv V
	m.vals[i] = zv
	if m.eq == nil || !m.eq.sorted {
		// (a sorted map keeps the key at a dead position: the order of the positions)
		var zk K
		m.keys[i] = zk
	}
	m.live--
}

// compact drops the dead positions; the order of the mappings is unchanged.
func (m *TMap[K, V]) compact() {
	n := 0
	for i := range m.keys {
		if !m.gone[i] {
			m.keys[n], m.vals[n] = m.keys[i], m.vals[i]
			n++
		}
	}
	clear(m.keys[n:])
	clear(m.vals[n:])
	m.keys, m.vals = m.keys[:n], m.vals[:n]
	if m.eq != nil {
		m.eq.compact(m.gone)
	}
	m.gone = nil
	if m.idx != nil {
		clear(m.idx)
		for i, x := range m.keys {
			m.idx[x] = int32(i)
		}
	}
}

func (m *TMap[K, V]) clear() {
	if m.imm != nil && m.imm != sortedMapMark_ {
		panic(frozen_{})
	}
	clear(m.keys)
	clear(m.vals)
	m.keys, m.vals = m.keys[:0], m.vals[:0]
	m.gone = nil
	m.idx = nil
	if m.eq != nil && !m.eq.sorted {
		m.eq = nil
	}
	m.live = 0
}

// putAllT is putAll of another typed map of the same types.
func (m *TMap[K, V]) putAllT(o *TMap[K, V]) {
	if m.imm != nil && m.imm != sortedMapMark_ {
		panic(frozen_{})
	}
	for i, k := range o.keys {
		if !o.dead(i) {
			m.put(k, o.vals[i])
		}
	}
}

// tmapKeys_ is the cursor of a for-each loop over the keys of a typed map, tmapVals_ over its
// values, tmapEntries_ over its entries: the mappings in insertion order, read position by
// position as the erased cursor reads its table.
type tmapKeys_[K comparable, V any] struct {
	m *TMap[K, V]
	i int
	v K
}

// eachKey is the cursor of a for-each loop over keySet(); a null map throws
// NullPointerException here, where the loop asked for the key set.
func (m *TMap[K, V]) eachKey() tmapKeys_[K, V] {
	_ = m.live
	return tmapKeys_[K, V]{m: m}
}

func (e *tmapKeys_[K, V]) next() bool {
	m := e.m
	for e.i < len(m.keys) {
		i := e.i
		e.i++
		if !m.dead(i) {
			e.v = m.keys[i]
			return true
		}
	}
	return false
}

type tmapVals_[K comparable, V any] struct {
	m *TMap[K, V]
	i int
	v V
}

// eachValue is the cursor of a for-each loop over values().
func (m *TMap[K, V]) eachValue() tmapVals_[K, V] {
	_ = m.live
	return tmapVals_[K, V]{m: m}
}

func (e *tmapVals_[K, V]) next() bool {
	m := e.m
	for e.i < len(m.keys) {
		i := e.i
		e.i++
		if !m.dead(i) {
			e.v = m.vals[i]
			return true
		}
	}
	return false
}

type tmapEntries_[K comparable, V any] struct {
	m *TMap[K, V]
	i int
}

// eachEntry is the cursor of a for-each loop over entrySet().
func (m *TMap[K, V]) eachEntry() tmapEntries_[K, V] {
	_ = m.live
	return tmapEntries_[K, V]{m: m}
}

func (e *tmapEntries_[K, V]) next() bool {
	m := e.m
	for e.i < len(m.keys) {
		e.i++
		if !m.dead(e.i - 1) {
			return true
		}
	}
	return false
}

// entry is the entry the cursor is at: an object of its own, as each entry of the Java map is.
func (e *tmapEntries_[K, V]) entry() *TEntry[K, V] {
	i := e.i - 1
	return &TEntry[K, V]{m: e.m, i: int32(i), k: e.m.keys[i], v: e.m.vals[i]}
}

// TEntry is an entry of a typed map (Map.Entry<K, V>) in a for-each loop over its entries. It
// reads and writes the value of its mapping while the map has it; after the mapping's removal
// it keeps the last value, as the entry of the erased map does.
type TEntry[K comparable, V any] struct {
	m *TMap[K, V]
	i int32
	k K
	v V
}

// pos is the position of the entry's mapping in the map, or -1 if the map has it no more.
func (e *TEntry[K, V]) pos() int32 {
	m := e.m
	if i := int(e.i); i < len(m.keys) && m.keys[i] == e.k && !m.dead(i) {
		return e.i
	}
	return m.find(e.k)
}

func (e *TEntry[K, V]) getKey() K { return e.k }

func (e *TEntry[K, V]) getValue() V {
	if i := e.pos(); i >= 0 {
		e.v = e.m.vals[i]
	}
	return e.v
}

func (e *TEntry[K, V]) setValue(v V) V {
	if e.m.imm != nil && e.m.imm != sortedMapMark_ {
		panic(frozen_{})
	}
	old := e.getValue()
	e.v = v
	if i := e.pos(); i >= 0 {
		e.m.vals[i] = v
	}
	return old
}

// asEntry is the entry as an erased Map.Entry (nil for null).
func (e *TEntry[K, V]) asEntry() Map_Entry {
	if e == nil {
		return nil
	}
	return &tentryView_[K, V]{e: e}
}

// tentryView_ is a TEntry seen as an erased Map.Entry.
type tentryView_[K comparable, V any] struct {
	e   *TEntry[K, V]
	own TEntry[K, V] // the entry itself, where erased code asked the map for it (TMap.out)
}

func (v *tentryView_[K, V]) self() any     { return v }
func (v *tentryView_[K, V]) getKey() any   { return v.e.m.kc.box(v.e.k) }
func (v *tentryView_[K, V]) getValue() any { return v.e.m.vc.box(v.e.getValue()) }
func (v *tentryView_[K, V]) setValue(x any) any {
	return v.e.m.vc.box(v.e.setValue(v.e.m.vc.unbox(x)))
}
func (v *tentryView_[K, V]) equals(o any) bool {
	x, ok := o.(Map_Entry)
	return ok && x != nil && objequals_(v.getKey(), x.getKey()) && objequals_(v.getValue(), x.getValue())
}
func (v *tentryView_[K, V]) hashCode() int32 { return objhash_(v.getKey()) ^ objhash_(v.getValue()) }
func (v *tentryView_[K, V]) toString() string {
	return objstr_(v.getKey()) + "=" + objstr_(v.getValue())
}

// asMap is the map as an erased Map (nil for null): one view per map, its identity for erased
// code.
func (m *TMap[K, V]) asMap() Map {
	if m == nil || m.imm != nil {
		return m.asOtherMap()
	}
	return &m.view
}

// erasedMap_ is a map of the translated program as an erased Map, whether the translator made
// its declaration a typed map or not (for handwritten runtime code that calls generated code).
func erasedMap_(m any) Map {
	switch x := m.(type) {
	case Map:
		return x
	case interface{ asMap() Map }:
		return x.asMap()
	}
	return nil
}

// tmapView_ is a TMap seen as an erased Map. Its key set, values and entry set are parts of
// it: making one allocates nothing, as for the erased map.
type tmapView_[K comparable, V any] struct {
	m *TMap[K, V]
}

// key is x as a key of the map, if it can be one (a lookup of an object of another class finds
// nothing, as in Java).
func (v *tmapView_[K, V]) key(x any) (k K, ok bool) { return tkeyOf_(v.m.kc, x) }

func tkeyOf_[K any](c *tconv_[K], x any) (k K, ok bool) {
	if k, ok := x.(K); ok {
		return k, true
	}
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return c.unbox(x), true
}

// goName_ makes getClass() of the view java.util.LinkedHashMap, the class of the Java object.
func (v *tmapView_[K, V]) goName_() string { return "LinkedHashMap" }

func (v *tmapView_[K, V]) self() any     { return v }
func (v *tmapView_[K, V]) size() int32   { return v.m.live }
func (v *tmapView_[K, V]) isEmpty() bool { return v.m.live == 0 }
func (v *tmapView_[K, V]) clear()        { v.m.clear() }

func (v *tmapView_[K, V]) get(x any) any {
	if k, ok := v.key(x); ok {
		if i := v.m.find(k); i >= 0 {
			return v.m.vc.box(v.m.vals[i])
		}
	}
	return nil
}

func (v *tmapView_[K, V]) containsKey(x any) bool {
	k, ok := v.key(x)
	return ok && v.m.find(k) >= 0
}

func (v *tmapView_[K, V]) containsValue(x any) bool {
	m := v.m
	for i := range m.keys {
		if !m.dead(i) && objequals_(x, m.vc.box(m.vals[i])) {
			return true
		}
	}
	return false
}

func (v *tmapView_[K, V]) put(k any, x any) any {
	return v.m.vc.box(v.m.put(v.m.kc.unbox(k), v.m.vc.unbox(x)))
}

func (v *tmapView_[K, V]) putAll(o Map) {
	if t, ok := o.(*tmapView_[K, V]); ok && t != nil {
		v.m.putAllT(t.m)
		return
	}
	if h := hashMapOf_(o); h != nil {
		for i, k := range h.t.keys {
			if k != dead_ {
				v.m.put(v.m.kc.unbox(k), v.m.vc.unbox(h.t.vals[i]))
			}
		}
		return
	}
	for it := o.entrySet().iterator(); it.hasNext(); {
		e := it.next().(Map_Entry)
		v.m.put(v.m.kc.unbox(e.getKey()), v.m.vc.unbox(e.getValue()))
	}
}

func (v *tmapView_[K, V]) remove(x any) any {
	if k, ok := v.key(x); ok {
		return v.m.vc.box(v.m.remove(k))
	}
	return nil
}

// get_s, containsKey_s, put_s and remove_s are the operations on a key that is a String and
// not null, without its box (Map_get_s...).
func (v *tmapView_[K, V]) get_s(s string) any {
	if k, ok := any(s).(K); ok {
		if i := v.m.find(k); i >= 0 {
			return v.m.vc.box(v.m.vals[i])
		}
		return nil
	}
	return v.get(strobj_(s))
}

func (v *tmapView_[K, V]) containsKey_s(s string) bool {
	if k, ok := any(s).(K); ok {
		return v.m.find(k) >= 0
	}
	return v.containsKey(strobj_(s))
}

func (v *tmapView_[K, V]) put_s(s string, x any) any {
	if k, ok := any(s).(K); ok {
		return v.m.vc.box(v.m.put(k, v.m.vc.unbox(x)))
	}
	return v.put(strobj_(s), x)
}

func (v *tmapView_[K, V]) remove_s(s string) any {
	if k, ok := any(s).(K); ok {
		return v.m.vc.box(v.m.remove(k))
	}
	return v.remove(strobj_(s))
}

func (v *tmapView_[K, V]) keySet() Set        { return (*tmapKeySet_[K, V])(v) }
func (v *tmapView_[K, V]) values() Collection { return (*tmapValues_[K, V])(v) }
func (v *tmapView_[K, V]) entrySet() Set      { return (*tmapEntrySet_[K, V])(v) }
func (v *tmapView_[K, V]) equals(o any) bool  { return mapEquals_(v, o) }
func (v *tmapView_[K, V]) hashCode() int32    { return mapHash_(v) }
func (v *tmapView_[K, V]) toString() string   { return mapToString_(v) }

// What an iterator or a for-each cursor of erased code reads from a typed map.
const (
	tkeys_ int8 = iota
	tvals_
	tentries_
)

// out is the key, the value or the entry at position i, boxed.
func (m *TMap[K, V]) out(i int, mode int8) any {
	switch mode {
	case tkeys_:
		return m.kc.box(m.keys[i])
	case tvals_:
		return m.vc.box(m.vals[i])
	}
	// one object: the view holds the entry it shows
	return m.entryAt(i)
}

func (m *TMap[K, V]) step(i int, mode int8) (any, int) {
	for ; i < len(m.keys); i++ {
		if !m.dead(i) {
			return m.out(i, mode), i + 1
		}
	}
	return nil, -1
}

func (m *TMap[K, V]) all(mode int8) []any {
	out := make([]any, 0, m.live)
	for i := range m.keys {
		if !m.dead(i) {
			out = append(out, m.out(i, mode))
		}
	}
	return out
}

// tmapIter_ iterates over the keys, the values or the entries of a TMap for erased code
// (Iterator, with remove).
type tmapIter_[K comparable, V any] struct {
	m    *TMap[K, V]
	i    int
	last int
	mode int8
}

func (it *tmapIter_[K, V]) hasNext() bool {
	m := it.m
	for it.i < len(m.keys) && m.dead(it.i) {
		it.i++
	}
	return it.i < len(m.keys)
}

func (it *tmapIter_[K, V]) next() any {
	if !it.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	it.last = it.i
	it.i++
	return it.m.out(it.last, it.mode)
}

func (it *tmapIter_[K, V]) remove() {
	if it.last < 0 {
		panic(new_IllegalStateException_0().self())
	}
	it.m.delAt(it.last)
	it.last = -1
}

// tmapKeySet_ is keySet() of the view of a typed map.
type tmapKeySet_[K comparable, V any] struct{ m *TMap[K, V] }

func (v *tmapKeySet_[K, V]) self() any { return v }
func (v *tmapKeySet_[K, V]) iterator() Iterator {
	return &tmapIter_[K, V]{m: v.m, last: -1, mode: tkeys_}
}
func (v *tmapKeySet_[K, V]) size() int32   { return v.m.live }
func (v *tmapKeySet_[K, V]) isEmpty() bool { return v.m.live == 0 }
func (v *tmapKeySet_[K, V]) clear()        { v.m.clear() }
func (v *tmapKeySet_[K, V]) contains(x any) bool {
	k, ok := tkeyOf_(v.m.kc, x)
	return ok && v.m.find(k) >= 0
}
func (v *tmapKeySet_[K, V]) contains_s(s string) bool {
	if k, ok := any(s).(K); ok {
		return v.m.find(k) >= 0
	}
	return v.contains(strobj_(s))
}
func (v *tmapKeySet_[K, V]) add(x any) bool { panic(new_UnsupportedOperationException_0().self()) }
func (v *tmapKeySet_[K, V]) addAll(c Collection) bool {
	panic(new_UnsupportedOperationException_0().self())
}
func (v *tmapKeySet_[K, V]) remove_Object(x any) bool {
	k, ok := tkeyOf_(v.m.kc, x)
	if !ok {
		return false
	}
	i := v.m.find(k)
	if i < 0 {
		return false
	}
	v.m.delAt(int(i))
	return true
}
func (v *tmapKeySet_[K, V]) containsAll(c Collection) bool { return containsAll_(v, c) }
func (v *tmapKeySet_[K, V]) removeAll(c Collection) bool   { return removeAll_(v, c) }
func (v *tmapKeySet_[K, V]) toArray_0() []any              { return v.m.all(tkeys_) }
func (v *tmapKeySet_[K, V]) toArray(a []any) []any         { return fillArray_(v.m.all(tkeys_), a) }
func (v *tmapKeySet_[K, V]) equals(o any) bool             { return setEquals_(v, o) }
func (v *tmapKeySet_[K, V]) hashCode() int32               { return setHash_(v) }
func (v *tmapKeySet_[K, V]) toString() string              { return collToString_(v) }
func (v *tmapKeySet_[K, V]) step_(i int) (any, int)        { return v.m.step(i, tkeys_) }

// tmapValues_ is values() of the view of a typed map.
type tmapValues_[K comparable, V any] struct{ m *TMap[K, V] }

func (v *tmapValues_[K, V]) self() any { return v }
func (v *tmapValues_[K, V]) iterator() Iterator {
	return &tmapIter_[K, V]{m: v.m, last: -1, mode: tvals_}
}
func (v *tmapValues_[K, V]) size() int32   { return v.m.live }
func (v *tmapValues_[K, V]) isEmpty() bool { return v.m.live == 0 }
func (v *tmapValues_[K, V]) clear()        { v.m.clear() }
func (v *tmapValues_[K, V]) contains(x any) bool {
	return v.m.asMap().containsValue(x)
}
func (v *tmapValues_[K, V]) add(x any) bool { panic(new_UnsupportedOperationException_0().self()) }
func (v *tmapValues_[K, V]) addAll(c Collection) bool {
	panic(new_UnsupportedOperationException_0().self())
}
func (v *tmapValues_[K, V]) remove_Object(x any) bool {
	m := v.m
	for i := range m.keys {
		if !m.dead(i) && objequals_(x, m.vc.box(m.vals[i])) {
			m.delAt(i)
			return true
		}
	}
	return false
}
func (v *tmapValues_[K, V]) containsAll(c Collection) bool { return containsAll_(v, c) }
func (v *tmapValues_[K, V]) removeAll(c Collection) bool   { return removeAll_(v, c) }
func (v *tmapValues_[K, V]) toArray_0() []any              { return v.m.all(tvals_) }
func (v *tmapValues_[K, V]) toArray(a []any) []any         { return fillArray_(v.m.all(tvals_), a) }
func (v *tmapValues_[K, V]) toString() string              { return collToString_(v) }
func (v *tmapValues_[K, V]) step_(i int) (any, int)        { return v.m.step(i, tvals_) }

// tmapEntrySet_ is entrySet() of the view of a typed map.
type tmapEntrySet_[K comparable, V any] struct{ m *TMap[K, V] }

func (v *tmapEntrySet_[K, V]) self() any { return v }
func (v *tmapEntrySet_[K, V]) iterator() Iterator {
	return &tmapIter_[K, V]{m: v.m, last: -1, mode: tentries_}
}
func (v *tmapEntrySet_[K, V]) size() int32   { return v.m.live }
func (v *tmapEntrySet_[K, V]) isEmpty() bool { return v.m.live == 0 }
func (v *tmapEntrySet_[K, V]) clear()        { v.m.clear() }
func (v *tmapEntrySet_[K, V]) contains(x any) bool {
	e, ok := x.(Map_Entry)
	if !ok || e == nil {
		return false
	}
	mv := v.m.asMap()
	return mv.containsKey(e.getKey()) && objequals_(mv.get(e.getKey()), e.getValue())
}
func (v *tmapEntrySet_[K, V]) add(x any) bool { panic(new_UnsupportedOperationException_0().self()) }
func (v *tmapEntrySet_[K, V]) addAll(c Collection) bool {
	panic(new_UnsupportedOperationException_0().self())
}
func (v *tmapEntrySet_[K, V]) remove_Object(x any) bool {
	if !v.contains(x) {
		return false
	}
	v.m.asMap().remove(x.(Map_Entry).getKey())
	return true
}
func (v *tmapEntrySet_[K, V]) containsAll(c Collection) bool { return containsAll_(v, c) }
func (v *tmapEntrySet_[K, V]) removeAll(c Collection) bool   { return removeAll_(v, c) }
func (v *tmapEntrySet_[K, V]) toArray_0() []any              { return v.m.all(tentries_) }
func (v *tmapEntrySet_[K, V]) toArray(a []any) []any         { return fillArray_(v.m.all(tentries_), a) }
func (v *tmapEntrySet_[K, V]) equals(o any) bool             { return setEquals_(v, o) }
func (v *tmapEntrySet_[K, V]) hashCode() int32               { return setHash_(v) }
func (v *tmapEntrySet_[K, V]) toString() string              { return collToString_(v) }
func (v *tmapEntrySet_[K, V]) step_(i int) (any, int)        { return v.m.step(i, tentries_) }
