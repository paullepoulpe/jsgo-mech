package closure

// Typed maps and sets (post-flip step 4): LinkedHashMap<K, V> and LinkedHashSet<K> with
// unboxed keys and values. Keys whose Java equality is Go's == (strings, handles, classes
// without equals of their own) are found by ==; keys of a class with its own equals and
// hashCode (the key conversion has hash and eq) as java.util.HashMap finds them: by the hash
// code each key had when it was put, then by identity or equals.
//
// Insertion order is kept as in LinkedHashMap: keys and values in slices, removed positions
// marked dead, a Go map from key to position once the table has more than tsmall_ entries.
// Code that still expects an erased Map or Set gets asMap() / asSet(), a view with the
// collection's identity. Behaviour mirrors rt_coll.go.

const tsmall_ = 8

type TMap[K comparable, V any] struct {
	keys []K
	vals []V
	kc   *tconv_[K]
	vc   *tconv_[V]
	live int32
	imm  bool
	x    *tmapExt_[K, V] // what most maps never need; nil until then
}

// tmapExt_ holds the parts of a typed map that a small map never needs, so that the many small
// maps and sets the compiler creates stay small (a TMap is 80 bytes).
type tmapExt_[K comparable, V any] struct {
	dead  []bool // beside keys once something was removed; else nil
	idx   map[K]int32
	ndead int32
	head  int32 // every position below head is a removed entry (with dead; a hint for first)
	view  *tmapView_[K, V]
	sview *tsetView_[K]
	// Keys compared by equals (kc.hash != nil): each key's hash code, beside keys, and once the
	// map has more than tsmall_ entries an open-addressed index of them (hindex_'s layout:
	// the mixed hash in the high 32 bits, 1 + the position in the low ones, 0 for free).
	hs    []int32
	slots []uint64
	// Of an immutable map or set: the ImmutableMap or ImmutableSet erased code sees (rt_timm.go).
	twin any
	// Of an immutable set: its hash code, once computed (tsetHash_; Guava computes it once).
	hash   int32
	hashed bool
	// Of a sorted map or set (a TreeMap, a TreeSet in a typed declaration: rt_tsorted.go): its
	// order. The keys are kept in that order and found by binary search; dead, idx, hs and slots
	// are not used.
	sort *tsort_[K]
	// Of an immutable map: its key set as typed code sees it, once that is needed (keySetImm).
	keySet *TSet[K]
}

func (m *TMap[K, V]) ext() *tmapExt_[K, V] {
	if m.x == nil {
		m.x = &tmapExt_[K, V]{}
	}
	return m.x
}

// isDead reports whether position i holds a removed entry.
func (m *TMap[K, V]) isDead(i int) bool { return m.x != nil && m.x.dead != nil && m.x.dead[i] }

// first is the position iteration starts at: every position below it is a removed entry.
// LinkedHashMap unlinks a removed entry, so its iteration starts at the first live one at once;
// here the removed entries stay in place until the table compacts, and a map used as a work
// queue (remove the first, add at the end) would skip a growing run of them on every
// getFirst. Entries are only appended and a removed one never comes back, so the hint stays
// right until keys is rebuilt (compact, clear), which drops dead and resets it.
func (m *TMap[K, V]) first() int {
	x := m.x
	if x == nil || x.dead == nil {
		return 0
	}
	i := int(x.head)
	for i < len(m.keys) && x.dead[i] {
		i++
	}
	x.head = int32(i)
	return i
}

func newTMap_[K comparable, V any](kc *tconv_[K], vc *tconv_[V]) *TMap[K, V] {
	return &TMap[K, V]{kc: kc, vc: vc}
}

// newTMapN_ is new LinkedHashMap<>(n): the capacity is not used (j2go).
func newTMapN_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], n int32) *TMap[K, V] {
	return &TMap[K, V]{kc: kc, vc: vc}
}

// tmapCopy_ is new LinkedHashMap<>(from) of another typed map of the same types (j2go).
func tmapCopy_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], from *TMap[K, V]) *TMap[K, V] {
	m := &TMap[K, V]{kc: kc, vc: vc}
	m.putAllT(from)
	return m
}

// putAllT is putAll of another typed map of the same types (j2go).
func (m *TMap[K, V]) putAllT(o *TMap[K, V]) {
	if m.imm {
		throwUOE_()
	}
	for i, k := range o.keys {
		if !o.isDead(i) {
			m.Put(k, o.vals[i])
		}
	}
}

// erasedMap_ is a map of the translated program as an erased Map, whether its declaration is a
// typed map or not (for handwritten runtime code that calls generated code).
func erasedMap_(m any) Map {
	switch x := m.(type) {
	case Map:
		return x
	case interface{ asMap() Map }:
		return x.asMap()
	}
	return nil
}

// tmapFrom_ is new LinkedHashMap<>(from) (or an ImmutableMap, imm) as a typed map.
func tmapFrom_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], from Map, imm bool) *TMap[K, V] {
	if imm && isNil_(from) {
		return nil
	}
	var slot *immTyped_
	if imm {
		slot = immSlot_(from)
		if slot != nil && slot.conv == any(vc) && slot.v != nil {
			if m, ok := slot.v.(*TMap[K, V]); ok && m.kc == kc {
				return m
			}
		}
	}
	m := newTMap_(kc, vc)
	for it := from.entrySet().iterator(); it.hasNext(); {
		e := it.next().(Map_Entry)
		m.Put(kc.unbox(e.getKey()), vc.unbox(e.getValue()))
	}
	m.imm = imm
	if slot != nil {
		*slot = immTyped_{vc, m}
	}
	if im, ok := from.(*ErasedImmutableMap); ok && imm {
		m.ext().twin = im // the map is the immutable map itself for erased code (asMap)
	}
	return m
}

func (m *TMap[K, V]) find(k K) int32 {
	if m.kc.hash != nil {
		return m.findH(k)
	}
	if m.x != nil {
		if m.x.idx != nil {
			if i, ok := m.x.idx[k]; ok {
				return i
			}
			return -1
		}
		if m.x.sort != nil {
			return m.findSorted(k)
		}
		for i, x := range m.keys {
			if x == k && !m.isDead(i) {
				return int32(i)
			}
		}
		return -1
	}
	for i, x := range m.keys {
		if x == k {
			return int32(i)
		}
	}
	return -1
}

// hashOf is the hash code java.util.HashMap gives key k: 0 for null.
func (m *TMap[K, V]) hashOf(k K) int32 {
	var zk K
	if k == zk {
		return 0
	}
	return m.kc.hash(k)
}

// findH is find for keys compared by equals: a key is found at a live position whose key had
// the same hash code and is the same object, or equals it (k.equals(key), as HashMap.getNode).
func (m *TMap[K, V]) findH(k K) int32 {
	x := m.x
	if x != nil && x.sort != nil {
		return m.findSorted(k)
	}
	if x == nil || len(x.hs) == 0 {
		return -1
	}
	h := m.hashOf(k)
	var zk K
	match := func(i int32) bool {
		if m.isDead(int(i)) {
			return false
		}
		e := m.keys[i]
		return e == k || (k != zk && m.kc.eq(k, e))
	}
	if x.slots == nil {
		for i, eh := range x.hs {
			if eh == h && match(int32(i)) {
				return int32(i)
			}
		}
		return -1
	}
	hv := hmix_(h)
	mask := uint32(len(x.slots) - 1)
	for j := hv & mask; x.slots[j] != 0; j = (j + 1) & mask {
		if sl := x.slots[j]; uint32(sl>>32) == hv {
			if i := int32(uint32(sl)) - 1; match(i) {
				return i
			}
		}
	}
	return -1
}

// putH records the hash code of the key just appended at position n (keys compared by equals).
func (m *TMap[K, V]) putH(n int32) {
	x := m.ext()
	h := m.hashOf(m.keys[n])
	x.hs = append(x.hs, h)
	if x.slots == nil {
		if len(m.keys) > tsmall_ {
			m.reindexH()
		}
		return
	}
	if 2*len(m.keys) > len(x.slots) {
		m.reindexH()
		return
	}
	m.placeH(hmix_(h), n)
}

func (m *TMap[K, V]) placeH(hv uint32, i int32) {
	x := m.x
	mask := uint32(len(x.slots) - 1)
	j := hv & mask
	for x.slots[j] != 0 {
		j = (j + 1) & mask
	}
	x.slots[j] = uint64(hv)<<32 | uint64(uint32(i+1))
}

// reindexH builds the index of the live keys from their recorded hash codes.
func (m *TMap[K, V]) reindexH() {
	x := m.x
	n := 16
	for n < 4*len(m.keys) {
		n *= 2
	}
	x.slots = make([]uint64, n)
	for i, h := range x.hs {
		if !m.isDead(i) {
			m.placeH(hmix_(h), int32(i))
		}
	}
}

func (m *TMap[K, V]) Len() int32    { return m.live }
func (m *TMap[K, V]) IsEmpty() bool { return m.live == 0 }

func (m *TMap[K, V]) Get(k K) V {
	if i := m.find(k); i >= 0 {
		return m.vals[i]
	}
	return m.vc.null
}

func (m *TMap[K, V]) Has(k K) bool { return m.find(k) >= 0 }

func (m *TMap[K, V]) GetOr(k K, def V) V {
	if i := m.find(k); i >= 0 {
		return m.vals[i]
	}
	return def
}

// put stores v under k and returns the previous value (null if there was none).
func (m *TMap[K, V]) Put(k K, v V) V {
	if m.imm {
		throwUOE_()
	}
	if m.x != nil && m.x.sort != nil {
		return m.putSorted(k, v)
	}
	if i := m.find(k); i >= 0 {
		old := m.vals[i]
		m.vals[i] = v
		return old
	}
	if m.keys == nil {
		m.keys = make([]K, 0, 4) // as the runtime's tables: most stay this small
		m.vals = make([]V, 0, 4)
	}
	m.keys = append(m.keys, k)
	m.vals = append(m.vals, v)
	n := int32(len(m.keys) - 1)
	m.live++
	if m.kc.hash != nil {
		if x := m.x; x != nil && x.dead != nil {
			x.dead = append(x.dead, false)
		}
		m.putH(n)
		return m.vc.null
	}
	if x := m.x; x != nil {
		if x.dead != nil {
			x.dead = append(x.dead, false)
		}
		if x.idx != nil {
			x.idx[k] = n
			return m.vc.null
		}
	}
	if len(m.keys) > tsmall_ {
		m.reindex()
	}
	return m.vc.null
}

func (m *TMap[K, V]) reindex() {
	x := m.ext()
	x.idx = make(map[K]int32, len(m.keys))
	for i, k := range m.keys {
		if !m.isDead(i) {
			x.idx[k] = int32(i)
		}
	}
}

func (m *TMap[K, V]) PutIfAbsent(k K, v V) V {
	if m.imm {
		throwUOE_()
	}
	if i := m.find(k); i >= 0 {
		if old := m.vals[i]; !m.vc.isNull(old) {
			return old
		}
	}
	return m.Put(k, v)
}

// remove deletes the mapping of k and returns its value (null if there was none).
func (m *TMap[K, V]) Delete(k K) V {
	if m.imm {
		throwUOE_()
	}
	i := m.find(k)
	if i < 0 {
		return m.vc.null
	}
	return m.removeAt(i)
}

func (m *TMap[K, V]) removeAt(i int32) V {
	if m.imm {
		// (through a view of an immutable map that typed code holds: valuesT, rt_lg.go)
		throwUOE_()
	}
	old := m.vals[i]
	var zk K
	var zv V
	x := m.ext()
	if x.sort != nil {
		m.removeSorted(int(i))
		return old
	}
	if x.idx != nil {
		delete(x.idx, m.keys[i])
	}
	if x.dead == nil {
		x.dead = make([]bool, len(m.keys), cap(m.keys))
		x.head = 0
	}
	x.dead[i] = true
	m.keys[i] = zk
	m.vals[i] = zv
	m.live--
	x.ndead++
	if x.ndead > tsmall_ && x.ndead > m.live {
		m.compact()
	}
	return old
}

func (m *TMap[K, V]) compact() {
	j := 0
	hs := m.x.hs
	for i := range m.keys {
		if !m.isDead(i) {
			m.keys[j], m.vals[j] = m.keys[i], m.vals[i]
			if hs != nil {
				hs[j] = hs[i]
			}
			j++
		}
	}
	if hs != nil {
		m.x.hs = hs[:j]
	}
	var zk K
	var zv V
	for i := j; i < len(m.keys); i++ {
		m.keys[i], m.vals[i] = zk, zv
	}
	m.keys, m.vals = m.keys[:j], m.vals[:j]
	m.x.dead, m.x.ndead, m.x.head = nil, 0, 0
	if m.x.idx != nil {
		m.reindex()
	}
	if m.x.slots != nil {
		m.reindexH()
	}
}

func (m *TMap[K, V]) Clear() {
	if m.imm {
		throwUOE_()
	}
	clear(m.keys)
	clear(m.vals)
	m.keys, m.vals = m.keys[:0], m.vals[:0]
	m.live = 0
	if m.x != nil {
		m.x.dead, m.x.idx, m.x.ndead, m.x.head = nil, nil, 0, 0
		m.x.hs, m.x.slots = m.x.hs[:0], nil
	}
}

// computeIfAbsent is Map.computeIfAbsent with an erased function.
func (m *TMap[K, V]) computeIfAbsent(k K, f Function_e) V {
	if m.imm {
		throwUOE_()
	}
	if i := m.find(k); i >= 0 && !m.vc.isNull(m.vals[i]) {
		return m.vals[i]
	}
	r := f.apply(m.kc.box(k))
	if r == nil {
		return m.vc.null
	}
	v := m.vc.unbox(r)
	m.Put(k, v)
	return v
}

// computeIfAbsent_f is computeIfAbsent with the function as a Go func value, which the caller's
// closure can stay on the stack for (post-flip step funcparams calls it for lambdas).
func (m *TMap[K, V]) computeIfAbsent_f(k K, f func(any) any) V {
	if i := m.find(k); i >= 0 && !m.vc.isNull(m.vals[i]) {
		return m.vals[i]
	}
	r := f(m.kc.box(k))
	if r == nil {
		return m.vc.null
	}
	v := m.vc.unbox(r)
	m.Put(k, v)
	return v
}

// ComputeIfAbsent is Map.computeIfAbsent with a typed function (tools/postflip/tnames): the
// value of k if it has one that is not null; else f(k), stored unless it is null.
// An immutable map throws also where nothing would change, as the erased one does (it is the
// translator's typed computeIfAbsent of rule "function types": step 402 of the fifth flip).
func (m *TMap[K, V]) ComputeIfAbsent(k K, f func(K) V) V {
	if m.imm {
		throwUOE_()
	}
	if i := m.find(k); i >= 0 && !m.vc.isNull(m.vals[i]) {
		return m.vals[i]
	}
	v := f(k)
	if m.vc.isNull(v) {
		return m.vc.null
	}
	m.Put(k, v)
	return v
}

// ComputeIfAbsentObj is ComputeIfAbsent with a function whose result is an Object (a typed
// lambda adapter X_tfn_K_any held it; tools/postflip/adapters): f(k) is unboxed as
// computeIfAbsent unboxes what the adapter returns, and stored unless it is null.
func (m *TMap[K, V]) ComputeIfAbsentObj(k K, f func(K) any) V {
	if i := m.find(k); i >= 0 && !m.vc.isNull(m.vals[i]) {
		return m.vals[i]
	}
	r := f(k)
	if r == nil {
		return m.vc.null
	}
	v := m.vc.unbox(r)
	m.Put(k, v)
	return v
}

// each is the cursor of a for-each loop over the entries (keys, values) of the map.
func (m *TMap[K, V]) each() tmapEach_[K, V] { return tmapEach_[K, V]{m: m, i: m.first()} }

type tmapEach_[K comparable, V any] struct {
	m *TMap[K, V]
	i int
	k K
	v V
}

func (e *tmapEach_[K, V]) next() bool {
	m := e.m
	for e.i < len(m.keys) {
		i := e.i
		e.i++
		if !m.isDead(i) {
			e.k, e.v = m.keys[i], m.vals[i]
			return true
		}
	}
	return false
}

// tmapEntries_ is the cursor of a for-each loop over the entries of a typed map as objects
// (j2go: a loop whose entry variable is used as a Map.Entry).
type tmapEntries_[K comparable, V any] struct {
	m *TMap[K, V]
	i int
}

// eachEntry is the cursor of a for-each loop over entrySet().
func (m *TMap[K, V]) eachEntry() tmapEntries_[K, V] { return tmapEntries_[K, V]{m: m, i: m.first()} }

func (e *tmapEntries_[K, V]) next() bool {
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
	if i := int(e.i); i < len(m.keys) && m.keys[i] == e.k && !m.isDead(i) {
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
	if e.m.imm {
		throwUOE_()
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
	own TEntry[K, V] // of a view made with its entry (entryAt, rt_lg.go): e is &own
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

// ---- sets

type TSet[K comparable] struct {
	TMap[K, struct{}]
}

var tunit_ = &tconv_[struct{}]{box: func(struct{}) any { return nil }, unbox: func(any) struct{} { return struct{}{} }}

func newTSet_[K comparable](kc *tconv_[K]) *TSet[K] {
	return &TSet[K]{TMap: TMap[K, struct{}]{kc: kc, vc: tunit_}}
}

// newTSetN_ is new LinkedHashSet<>(n): the capacity is not used (j2go).
func newTSetN_[K comparable](kc *tconv_[K], n int32) *TSet[K] { return newTSet_(kc) }

// tsetCopy_ is new LinkedHashSet<>(from) of another typed set of the same element type (j2go).
func tsetCopy_[K comparable](kc *tconv_[K], from *TSet[K]) *TSet[K] {
	s := newTSet_(kc)
	for i, k := range from.keys {
		if !from.isDead(i) {
			s.Add(k)
		}
	}
	return s
}

// iterator is the set's Iterator (of boxed elements, with remove), as its view's (j2go).
func (s *TSet[K]) iterator() Iterator { return s.cursor() }

// asSet_ is a set of the translated program as an erased Set, whether its declaration is a
// typed set or not (for handwritten runtime code that calls generated code).
func asSet_(s any) Set {
	switch x := s.(type) {
	case Set:
		return x
	case interface{ asSet() Set }:
		return x.asSet()
	}
	return nil
}

// tsetFrom_ is new LinkedHashSet<>(from) (or an ImmutableSet, imm) as a typed set.
func tsetFrom_[K comparable](kc *tconv_[K], from Collection, imm bool) *TSet[K] {
	if imm && isNil_(from) {
		return nil
	}
	var slot *immTyped_
	if imm {
		slot = immSlot_(from)
		if slot != nil && slot.conv == any(kc) {
			if s, ok := slot.v.(*TSet[K]); ok {
				return s
			}
		}
	}
	s := newTSet_(kc)
	for _, x := range toSlice_(from) {
		s.Add(kc.unbox(x))
	}
	s.imm = imm
	if slot != nil {
		*slot = immTyped_{kc, s}
	}
	if ic, ok := from.(*ImmutableCollection); ok && imm && ic.kind == icSet_ {
		s.ext().twin = ic // the set is the immutable set itself for erased code (asSet)
	}
	return s
}

func (s *TSet[K]) Add(k K) bool {
	if s.imm {
		throwUOE_()
	}
	if s.find(k) >= 0 {
		return false
	}
	s.Put(k, struct{}{})
	return true
}

func (s *TSet[K]) Has(k K) bool { return s.find(k) >= 0 }

func (s *TSet[K]) Delete(k K) bool {
	if s.imm {
		throwUOE_()
	}
	i := s.find(k)
	if i < 0 {
		return false
	}
	s.removeAt(i)
	return true
}

func (s *TSet[K]) each() tsetEach_[K] { return tsetEach_[K]{m: &s.TMap, i: s.first()} }

type tsetEach_[K comparable] struct {
	m *TMap[K, struct{}]
	i int
	v K
}

func (e *tsetEach_[K]) next() bool {
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

// ---- the erased faces

// try unboxes v, or reports that v is not a T (a lookup with a key of another class).
func (c *tconv_[T]) try(v any) (r T, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return c.unbox(v), true
}

func (c *tconv_[T]) isNull(v T) bool {
	if c.isnull != nil {
		return c.isnull(v)
	}
	return any(v) == any(c.null)
}

func (m *TMap[K, V]) asMap() Map {
	if m == nil {
		return nil
	}
	if m.imm {
		return m.asImm()
	}
	x := m.ext()
	if x.sort != nil {
		return m.sortedView()
	}
	if x.view == nil {
		x.view = &tmapView_[K, V]{m: m}
	}
	return x.view
}

func (s *TSet[K]) asSet() Set {
	if s == nil {
		return nil
	}
	if s.imm {
		return s.asImm()
	}
	x := s.ext()
	if x.sort != nil {
		return s.sortedView()
	}
	if x.sview == nil {
		x.sview = &tsetView_[K]{m: &s.TMap, s: s}
	}
	return x.sview
}

type tmapView_[K comparable, V any] struct{ m *TMap[K, V] }

// goName_ makes getClass() of the views java.util.LinkedHashMap and LinkedHashSet, the classes
// of the Java objects.
func (v *tmapView_[K, V]) goName_() string { return "LinkedHashMap" }
func (v *tsetView_[K]) goName_() string    { return "LinkedHashSet" }

// boxedKV_ is the keys and values of the map, boxed, in the order its entry set's iterator
// gives them (from the first live position, skipping removed ones), without the iterator and
// an entry object per element (mapKV_: ImmutableMap.copyOf of a typed map's view).
func (v *tmapView_[K, V]) boxedKV_() (keys, vals []any) {
	m := v.m
	for i := m.first(); i < len(m.keys); i++ {
		if !m.isDead(i) {
			keys = append(keys, m.kc.box(m.keys[i]))
			vals = append(vals, m.vc.box(m.vals[i]))
		}
	}
	return keys, vals
}

func (v *tmapView_[K, V]) self() any     { return v }
func (v *tmapView_[K, V]) size() int32   { return v.m.live }
func (v *tmapView_[K, V]) isEmpty() bool { return v.m.live == 0 }
func (v *tmapView_[K, V]) clear()        { v.m.Clear() }
func (v *tmapView_[K, V]) get(k any) any {
	if kk, ok := v.m.kc.try(k); ok {
		if i := v.m.find(kk); i >= 0 {
			return v.m.vc.box(v.m.vals[i])
		}
	}
	return nil
}
func (v *tmapView_[K, V]) containsKey(k any) bool {
	kk, ok := v.m.kc.try(k)
	return ok && v.m.find(kk) >= 0
}
func (v *tmapView_[K, V]) containsValue(x any) bool {
	for i := range v.m.keys {
		if (!v.m.isDead(i)) && objequals_(x, v.m.vc.box(v.m.vals[i])) {
			return true
		}
	}
	return false
}
func (v *tmapView_[K, V]) put(k any, x any) any {
	return v.m.vc.box(v.m.Put(v.m.kc.unbox(k), v.m.vc.unbox(x)))
}
func (v *tmapView_[K, V]) putAll(o Map) {
	for it := o.entrySet().iterator(); it.hasNext(); {
		e := it.next().(Map_Entry)
		v.put(e.getKey(), e.getValue())
	}
}
func (v *tmapView_[K, V]) remove(k any) any {
	if v.m.imm {
		throwUOE_()
	}
	if kk, ok := v.m.kc.try(k); ok {
		if i := v.m.find(kk); i >= 0 {
			return v.m.vc.box(v.m.removeAt(i))
		}
	}
	return nil
}

// (the views of the map are its view under another type: one object, as Java's are, and what
// typed code holds as keySetT, valuesT and entrySetT: rt_lg.go)
func (v *tmapView_[K, V]) keySet() Set        { return (*tkeysView_[K, V])(v) }
func (v *tmapView_[K, V]) values() Collection { return (*tvalsView_[K, V])(v) }
func (v *tmapView_[K, V]) entrySet() Set      { return (*tentriesView_[K, V])(v) }
func (v *tmapView_[K, V]) equals(o any) bool  { return mapEquals_(v, o) }
func (v *tmapView_[K, V]) hashCode() int32    { return mapHash_(v) }
func (v *tmapView_[K, V]) toString() string   { return mapToString_(v) }

// tmapIter_ walks the live positions of a typed map for erased code; out picks what it yields.
type tmapIter_[K comparable, V any] struct {
	m    *TMap[K, V]
	i    int
	last int32
	out  func(m *TMap[K, V], i int32) any
}

func (it *tmapIter_[K, V]) hasNext() bool {
	for it.i < len(it.m.keys) && it.m.isDead(it.i) {
		it.i++
	}
	return it.i < len(it.m.keys)
}

func (it *tmapIter_[K, V]) next() any {
	if !it.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	it.last = int32(it.i)
	it.i++
	return it.out(it.m, it.last)
}

// cursor is s.asSet().iterator() as a typed pointer, without the views (post-flip step iters):
// the same iterator over the keys, from the first live position. A null set throws
// NullPointerException here, as asSet().iterator() did.
func (s *TSet[K]) cursor() *tmapIter_[K, struct{}] {
	if s == nil {
		panic(new_NullPointerException_0().self())
	}
	return &tmapIter_[K, struct{}]{m: &s.TMap, i: s.first(), out: keyOut_[K, struct{}]}
}

// nextKey is next of a key iterator without boxing the key (the code unboxed it).
func (it *tmapIter_[K, V]) nextKey() K {
	if !it.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	it.last = int32(it.i)
	it.i++
	return it.m.keys[it.last]
}

func (it *tmapIter_[K, V]) remove() {
	if it.m.imm {
		throwUOE_()
	}
	// Live positions before the removed one: where the next element is if the table compacts.
	live := 0
	for j := it.m.first(); j < int(it.last); j++ {
		if !it.m.isDead(j) {
			live++
		}
	}
	it.m.removeAt(it.last)
	if it.m.x.dead == nil { // compacted: the live positions moved down
		it.i = live
	}
}

func keyOut_[K comparable, V any](m *TMap[K, V], i int32) any { return m.kc.box(m.keys[i]) }
func valOut_[K comparable, V any](m *TMap[K, V], i int32) any { return m.vc.box(m.vals[i]) }
func entryOut_[K comparable, V any](m *TMap[K, V], i int32) any {
	return &tmapEntry_[K, V]{m: m, k: m.keys[i], v: m.vals[i]}
}

type tmapEntry_[K comparable, V any] struct {
	m *TMap[K, V]
	k K
	v V
}

func (e *tmapEntry_[K, V]) self() any     { return e }
func (e *tmapEntry_[K, V]) getKey() any   { return e.m.kc.box(e.k) }
func (e *tmapEntry_[K, V]) getValue() any { return e.m.vc.box(e.v) }
func (e *tmapEntry_[K, V]) setValue(x any) any {
	if e.m.imm {
		throwUOE_()
	}
	old := e.getValue()
	e.v = e.m.vc.unbox(x)
	if i := e.m.find(e.k); i >= 0 {
		e.m.vals[i] = e.v
	}
	return old
}
func (e *tmapEntry_[K, V]) equals(o any) bool {
	x, ok := o.(Map_Entry)
	return ok && x != nil && objequals_(e.getKey(), x.getKey()) && objequals_(e.getValue(), x.getValue())
}
func (e *tmapEntry_[K, V]) hashCode() int32 { return objhash_(e.getKey()) ^ objhash_(e.getValue()) }
func (e *tmapEntry_[K, V]) toString() string {
	return objstr_(e.getKey()) + "=" + objstr_(e.getValue())
}

// tcollBase_ holds what the key, value and entry views share.
type tcollBase_[K comparable, V any] struct {
	m   *TMap[K, V]
	out func(m *TMap[K, V], i int32) any
}

func (v *tcollBase_[K, V]) size() int32   { return v.m.live }
func (v *tcollBase_[K, V]) isEmpty() bool { return v.m.live == 0 }
func (v *tcollBase_[K, V]) clear()        { v.m.Clear() }
func (v *tcollBase_[K, V]) iterator() Iterator {
	return &tmapIter_[K, V]{m: v.m, i: v.m.first(), out: v.out}
}
func (v *tcollBase_[K, V]) toArray_0() []any {
	out := make([]any, 0, v.m.live)
	for i := range v.m.keys {
		if !v.m.isDead(i) {
			out = append(out, v.out(v.m, int32(i)))
		}
	}
	return out
}
func (v *tcollBase_[K, V]) toArray(a []any) []any { return fillArray_(v.toArray_0(), a) }
func (v *tcollBase_[K, V]) add(x any) bool        { panic(new_UnsupportedOperationException_0().self()) }
func (v *tcollBase_[K, V]) addAll(c Collection) bool {
	panic(new_UnsupportedOperationException_0().self())
}

type tkeysView_[K comparable, V any] struct{ m *TMap[K, V] }

func (v *tkeysView_[K, V]) base() *tcollBase_[K, V] {
	return &tcollBase_[K, V]{m: v.m, out: keyOut_[K, V]}
}
func (v *tkeysView_[K, V]) self() any                { return v }
func (v *tkeysView_[K, V]) size() int32              { return v.m.live }
func (v *tkeysView_[K, V]) isEmpty() bool            { return v.m.live == 0 }
func (v *tkeysView_[K, V]) clear()                   { v.m.Clear() }
func (v *tkeysView_[K, V]) iterator() Iterator       { return v.base().iterator() }
func (v *tkeysView_[K, V]) toArray_0() []any         { return v.base().toArray_0() }
func (v *tkeysView_[K, V]) toArray(a []any) []any    { return v.base().toArray(a) }
func (v *tkeysView_[K, V]) add(x any) bool           { return v.base().add(x) }
func (v *tkeysView_[K, V]) addAll(c Collection) bool { return v.base().addAll(c) }
func (v *tkeysView_[K, V]) contains(x any) bool {
	k, ok := v.m.kc.try(x)
	return ok && v.m.find(k) >= 0
}
func (v *tkeysView_[K, V]) remove_Object(x any) bool {
	if v.m.imm {
		throwUOE_()
	}
	if k, ok := v.m.kc.try(x); ok {
		if i := v.m.find(k); i >= 0 {
			v.m.removeAt(i)
			return true
		}
	}
	return false
}
func (v *tkeysView_[K, V]) containsAll(c Collection) bool { return containsAll_(v, c) }
func (v *tkeysView_[K, V]) removeAll(c Collection) bool   { return removeAll_(v, c) }
func (v *tkeysView_[K, V]) equals(o any) bool             { return setEquals_(v, o) }
func (v *tkeysView_[K, V]) hashCode() int32               { return setHash_(v) }
func (v *tkeysView_[K, V]) toString() string              { return collToString_(v) }

type tvalsView_[K comparable, V any] struct{ m *TMap[K, V] }

func (v *tvalsView_[K, V]) base() *tcollBase_[K, V] {
	return &tcollBase_[K, V]{m: v.m, out: valOut_[K, V]}
}
func (v *tvalsView_[K, V]) self() any                { return v }
func (v *tvalsView_[K, V]) size() int32              { return v.m.live }
func (v *tvalsView_[K, V]) isEmpty() bool            { return v.m.live == 0 }
func (v *tvalsView_[K, V]) clear()                   { v.m.Clear() }
func (v *tvalsView_[K, V]) iterator() Iterator       { return v.base().iterator() }
func (v *tvalsView_[K, V]) toArray_0() []any         { return v.base().toArray_0() }
func (v *tvalsView_[K, V]) toArray(a []any) []any    { return v.base().toArray(a) }
func (v *tvalsView_[K, V]) add(x any) bool           { return v.base().add(x) }
func (v *tvalsView_[K, V]) addAll(c Collection) bool { return v.base().addAll(c) }
func (v *tvalsView_[K, V]) contains(x any) bool {
	for i := range v.m.keys {
		if (!v.m.isDead(i)) && objequals_(x, v.m.vc.box(v.m.vals[i])) {
			return true
		}
	}
	return false
}
func (v *tvalsView_[K, V]) remove_Object(x any) bool {
	for i := range v.m.keys {
		if (!v.m.isDead(i)) && objequals_(x, v.m.vc.box(v.m.vals[i])) {
			if v.m.imm {
				throwUOE_()
			}
			v.m.removeAt(int32(i))
			return true
		}
	}
	return false
}
func (v *tvalsView_[K, V]) containsAll(c Collection) bool { return containsAll_(v, c) }
func (v *tvalsView_[K, V]) removeAll(c Collection) bool   { return removeAll_(v, c) }
func (v *tvalsView_[K, V]) toString() string              { return collToString_(v) }

type tentriesView_[K comparable, V any] struct{ m *TMap[K, V] }

func (v *tentriesView_[K, V]) base() *tcollBase_[K, V] {
	return &tcollBase_[K, V]{m: v.m, out: entryOut_[K, V]}
}
func (v *tentriesView_[K, V]) self() any                { return v }
func (v *tentriesView_[K, V]) size() int32              { return v.m.live }
func (v *tentriesView_[K, V]) isEmpty() bool            { return v.m.live == 0 }
func (v *tentriesView_[K, V]) clear()                   { v.m.Clear() }
func (v *tentriesView_[K, V]) iterator() Iterator       { return v.base().iterator() }
func (v *tentriesView_[K, V]) toArray_0() []any         { return v.base().toArray_0() }
func (v *tentriesView_[K, V]) toArray(a []any) []any    { return v.base().toArray(a) }
func (v *tentriesView_[K, V]) add(x any) bool           { return v.base().add(x) }
func (v *tentriesView_[K, V]) addAll(c Collection) bool { return v.base().addAll(c) }
func (v *tentriesView_[K, V]) contains(x any) bool {
	e, ok := x.(Map_Entry)
	if !ok || e == nil {
		return false
	}
	k, ok := v.m.kc.try(e.getKey())
	if !ok {
		return false
	}
	i := v.m.find(k)
	return i >= 0 && objequals_(v.m.vc.box(v.m.vals[i]), e.getValue())
}
func (v *tentriesView_[K, V]) remove_Object(x any) bool {
	if !v.contains(x) {
		return false
	}
	k, _ := v.m.kc.try(x.(Map_Entry).getKey())
	v.m.Delete(k)
	return true
}
func (v *tentriesView_[K, V]) containsAll(c Collection) bool { return containsAll_(v, c) }
func (v *tentriesView_[K, V]) removeAll(c Collection) bool   { return removeAll_(v, c) }
func (v *tentriesView_[K, V]) equals(o any) bool             { return setEquals_(v, o) }
func (v *tentriesView_[K, V]) hashCode() int32               { return setHash_(v) }
func (v *tentriesView_[K, V]) toString() string              { return collToString_(v) }

// tsetView_ is a TSet seen as an erased Set.
type tsetView_[K comparable] struct {
	m *TMap[K, struct{}]
	s *TSet[K] // the set itself (m is its map)
}

func (v *tsetView_[K]) keys() *tkeysView_[K, struct{}] { return &tkeysView_[K, struct{}]{m: v.m} }
func (v *tsetView_[K]) self() any                      { return v }
func (v *tsetView_[K]) size() int32                    { return v.m.live }
func (v *tsetView_[K]) isEmpty() bool                  { return v.m.live == 0 }
func (v *tsetView_[K]) clear()                         { v.m.Clear() }
func (v *tsetView_[K]) iterator() Iterator             { return v.keys().iterator() }
func (v *tsetView_[K]) toArray_0() []any               { return v.keys().toArray_0() }
func (v *tsetView_[K]) toArray(a []any) []any          { return v.keys().toArray(a) }
func (v *tsetView_[K]) contains(x any) bool            { return v.keys().contains(x) }
func (v *tsetView_[K]) remove_Object(x any) bool       { return v.keys().remove_Object(x) }
func (v *tsetView_[K]) containsAll(c Collection) bool  { return containsAll_(v, c) }
func (v *tsetView_[K]) removeAll(c Collection) bool    { return removeAll_(v, c) }
func (v *tsetView_[K]) equals(o any) bool              { return setEquals_(v, o) }
func (v *tsetView_[K]) hashCode() int32                { return setHash_(v) }
func (v *tsetView_[K]) toString() string               { return collToString_(v) }
func (v *tsetView_[K]) add(x any) bool {
	if v.m.imm {
		throwUOE_()
	}
	k := v.m.kc.unbox(x)
	if v.m.find(k) >= 0 {
		return false
	}
	v.m.Put(k, struct{}{})
	return true
}
func (v *tsetView_[K]) addAll(c Collection) bool {
	changed := false
	for _, x := range toSlice_(c) {
		if v.add(x) {
			changed = true
		}
	}
	return changed
}

var (
	_ Map        = (*tmapView_[string, int32])(nil)
	_ Set        = (*tsetView_[string])(nil)
	_ Set        = (*tkeysView_[string, int32])(nil)
	_ Set        = (*tentriesView_[string, int32])(nil)
	_ Collection = (*tvalsView_[string, int32])(nil)
	_ List       = (*tlistView_[string])(nil)
)

// addAllT is addAll from another typed set.
func (s *TSet[K]) AddAll(o *TSet[K]) bool {
	if s.imm {
		throwUOE_()
	}
	changed := false
	for i, k := range o.keys {
		if !o.isDead(i) && s.Add(k) {
			changed = true
		}
	}
	return changed
}
