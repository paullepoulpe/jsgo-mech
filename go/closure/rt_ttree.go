package closure

// Typed sorted collections (post-flip step colls -kind treeset / treemap): TTreeSet[K] is a
// TreeSet and TTreeMap[K, V] a TreeMap whose elements are kept unboxed. Each is the runtime's
// sorted slice (rt_coll2.go) with the same comparator, searched the same way: compare is
// called with the same arguments in the same order, so a comparator sees what it saw, an
// element the comparator throws on (null under natural ordering) throws as it did, and an
// equal element is found where it was. asSet / asMap give erased code a view with the
// collection's identity.

import (
	"iter"
	"sort"
)

// ttreeCmp_ is the comparison of a sorted collection of K: the comparator given, on the boxed
// elements, or natural ordering (compareNatural_), for strings without boxing them; a null
// string goes through compareNatural_ as the boxed collection's did.
func ttreeCmp_[K any](kc *tconv_[K], c Comparator_e) func(a, b K) int32 {
	if c != nil {
		return func(a, b K) int32 { return c.compare(kc.box(a), kc.box(b)) }
	}
	if sc, ok := any(kc).(*tconv_[string]); ok {
		f := func(a, b string) int32 {
			if a == nullstr_ || b == nullstr_ {
				return compareNatural_(sc.box(a), sc.box(b))
			}
			return String_compareTo(a, b)
		}
		return any(f).(func(a, b K) int32)
	}
	return func(a, b K) int32 { return compareNatural_(kc.box(a), kc.box(b)) }
}

// ttreeSearch_ is the position of k in sorted keys and whether it is there (TreeSet.search).
func ttreeSearch_[K any](keys []K, cmp func(a, b K) int32, k K) (int, bool) {
	i := sort.Search(len(keys), func(i int) bool { return cmp(keys[i], k) >= 0 })
	return i, i < len(keys) && cmp(keys[i], k) == 0
}

// ---- TTreeSet

type TTreeSet[K any] struct {
	kc    *tconv_[K]
	ecmp  Comparator_e // the comparator given (comparator()); nil: natural ordering
	cmp   func(a, b K) int32
	items []K
	view  *ttreeSetView_[K]
}

func newTTreeSet_[K any](kc *tconv_[K]) *TTreeSet[K] {
	return &TTreeSet[K]{kc: kc, cmp: ttreeCmp_(kc, nil)}
}

func newTTreeSetC_[K any](kc *tconv_[K], c Comparator_e) *TTreeSet[K] {
	return &TTreeSet[K]{kc: kc, ecmp: c, cmp: ttreeCmp_(kc, c)}
}

func (s *TTreeSet[K]) size() int32   { return int32(len(s.items)) }
func (s *TTreeSet[K]) isEmpty() bool { return len(s.items) == 0 }
func (s *TTreeSet[K]) clear()        { s.items = nil }

func (s *TTreeSet[K]) contains(k K) bool { _, ok := ttreeSearch_(s.items, s.cmp, k); return ok }

func (s *TTreeSet[K]) add(k K) bool {
	i, ok := ttreeSearch_(s.items, s.cmp, k)
	if ok {
		return false
	}
	var zero K
	s.items = append(s.items, zero)
	copy(s.items[i+1:], s.items[i:])
	s.items[i] = k
	return true
}

func (s *TTreeSet[K]) remove(k K) bool {
	i, ok := ttreeSearch_(s.items, s.cmp, k)
	if !ok {
		return false
	}
	s.items = append(s.items[:i], s.items[i+1:]...)
	return true
}

// addAllT is addAll from another typed sorted set, in its order.
func (s *TTreeSet[K]) addAllT(o *TTreeSet[K]) bool {
	changed := false
	for i := 0; i < len(o.items); i++ {
		if s.add(o.items[i]) {
			changed = true
		}
	}
	return changed
}

// each is the cursor of a for-each loop: the k-th step reads item k as it is then, as the
// boxed set's iterator did.
func (s *TTreeSet[K]) each() ttreeSetEach_[K] { return ttreeSetEach_[K]{s: s} }

type ttreeSetEach_[K any] struct {
	s *TTreeSet[K]
	i int
	v K
}

func (e *ttreeSetEach_[K]) next() bool {
	if e.i < len(e.s.items) {
		e.v = e.s.items[e.i]
		e.i++
		return true
	}
	return false
}

// All yields the items as each did (tools/postflip/ranges -loops).
func (s *TTreeSet[K]) All() iter.Seq[K] { return sliceAll_(&s.items) }

func (s *TTreeSet[K]) asSet() NavigableSet {
	if s == nil {
		return nil
	}
	if s.view == nil {
		s.view = &ttreeSetView_[K]{s: s}
	}
	return s.view
}

// ttreeSetView_ is the erased face of a TTreeSet: TreeSet's methods on boxed elements.
type ttreeSetView_[K any] struct{ s *TTreeSet[K] }

func (v *ttreeSetView_[K]) self() any                     { return v }
func (v *ttreeSetView_[K]) comparator() Comparator_e        { return v.s.ecmp }
func (v *ttreeSetView_[K]) size() int32                   { return v.s.size() }
func (v *ttreeSetView_[K]) isEmpty() bool                 { return v.s.isEmpty() }
func (v *ttreeSetView_[K]) clear()                        { v.s.clear() }
func (v *ttreeSetView_[K]) contains(x any) bool           { return v.s.contains(v.s.kc.unbox(x)) }
func (v *ttreeSetView_[K]) add(x any) bool                { return v.s.add(v.s.kc.unbox(x)) }
func (v *ttreeSetView_[K]) remove_Object(x any) bool      { return v.s.remove(v.s.kc.unbox(x)) }
func (v *ttreeSetView_[K]) containsAll(c Collection) bool { return containsAll_(v, c) }
func (v *ttreeSetView_[K]) removeAll(c Collection) bool   { return removeAll_(v, c) }
func (v *ttreeSetView_[K]) equals(o any) bool             { return setEquals_(v, o) }
func (v *ttreeSetView_[K]) hashCode() int32               { return setHash_(v) }
func (v *ttreeSetView_[K]) toString() string              { return collToString_(v) }
func (v *ttreeSetView_[K]) toArray(a []any) []any         { return fillArray_(v.toArray_0(), a) }

func (v *ttreeSetView_[K]) addAll(c Collection) bool {
	changed := false
	for it := c.iterator(); it.hasNext(); {
		if v.add(it.next()) {
			changed = true
		}
	}
	return changed
}

func (v *ttreeSetView_[K]) toArray_0() []any {
	out := make([]any, len(v.s.items))
	for i, k := range v.s.items {
		out[i] = v.s.kc.box(k)
	}
	return out
}

func (v *ttreeSetView_[K]) iterator() Iterator { return &ttreeSetIter_[K]{s: v.s} }

// descendingSet and tailSet are copies, as TreeSet's are.
func (v *ttreeSetView_[K]) descendingSet() NavigableSet {
	out := &TreeSet{cmp: cmpReversed_(Comparator_of_(func(a, b any) int32 { return v.s.cmp(v.s.kc.unbox(a), v.s.kc.unbox(b)) }))}
	for i := len(v.s.items) - 1; i >= 0; i-- {
		out.items = append(out.items, v.s.kc.box(v.s.items[i]))
	}
	return out
}

func (v *ttreeSetView_[K]) tailSet(from any, inclusive bool) NavigableSet {
	i, ok := ttreeSearch_(v.s.items, v.s.cmp, v.s.kc.unbox(from))
	if ok && !inclusive {
		i++
	}
	out := &TreeSet{cmp: v.s.ecmp}
	for _, k := range v.s.items[i:] {
		out.items = append(out.items, v.s.kc.box(k))
	}
	return out
}

// ttreeSetIter_ is TreeSet's iterator (sliceIter_ over the items, removing in place).
type ttreeSetIter_[K any] struct {
	s *TTreeSet[K]
	i int
}

func (it *ttreeSetIter_[K]) hasNext() bool { return it.i < len(it.s.items) }

func (it *ttreeSetIter_[K]) next() any {
	if it.i >= len(it.s.items) {
		panic(new_NoSuchElementException_0().self())
	}
	k := it.s.items[it.i]
	it.i++
	return it.s.kc.box(k)
}

func (it *ttreeSetIter_[K]) remove() {
	it.i--
	it.s.items = append(it.s.items[:it.i], it.s.items[it.i+1:]...)
}

// ---- TTreeMap

type TTreeMap[K, V any] struct {
	kc   *tconv_[K]
	vc   *tconv_[V]
	ecmp Comparator_e
	cmp  func(a, b K) int32
	keys []K
	vals []V
	view *ttreeMapView_[K, V]
}

func newTTreeMap_[K, V any](kc *tconv_[K], vc *tconv_[V]) *TTreeMap[K, V] {
	return &TTreeMap[K, V]{kc: kc, vc: vc, cmp: ttreeCmp_(kc, nil)}
}

func newTTreeMapC_[K, V any](kc *tconv_[K], vc *tconv_[V], c Comparator_e) *TTreeMap[K, V] {
	return &TTreeMap[K, V]{kc: kc, vc: vc, ecmp: c, cmp: ttreeCmp_(kc, c)}
}

func (m *TTreeMap[K, V]) search(k K) (int, bool) { return ttreeSearch_(m.keys, m.cmp, k) }

func (m *TTreeMap[K, V]) size() int32   { return int32(len(m.keys)) }
func (m *TTreeMap[K, V]) isEmpty() bool { return len(m.keys) == 0 }
func (m *TTreeMap[K, V]) clear()        { m.keys, m.vals = nil, nil }

func (m *TTreeMap[K, V]) get(k K) V {
	if i, ok := m.search(k); ok {
		return m.vals[i]
	}
	return m.vc.null
}

func (m *TTreeMap[K, V]) containsKey(k K) bool { _, ok := m.search(k); return ok }

func (m *TTreeMap[K, V]) put(k K, v V) V {
	i, ok := m.search(k)
	if ok {
		old := m.vals[i]
		m.vals[i] = v
		return old
	}
	var zk K
	var zv V
	m.keys = append(m.keys, zk)
	copy(m.keys[i+1:], m.keys[i:])
	m.keys[i] = k
	m.vals = append(m.vals, zv)
	copy(m.vals[i+1:], m.vals[i:])
	m.vals[i] = v
	return m.vc.null
}

func (m *TTreeMap[K, V]) remove(k K) V {
	i, ok := m.search(k)
	if !ok {
		return m.vc.null
	}
	old := m.vals[i]
	m.keys = append(m.keys[:i], m.keys[i+1:]...)
	m.vals = append(m.vals[:i], m.vals[i+1:]...)
	return old
}

// getOrDefault, putIfAbsent and computeIfAbsent are Map's default methods (Map_getOrDefault...).
func (m *TTreeMap[K, V]) getOrDefault(k K, def V) V {
	if i, ok := m.search(k); ok {
		return m.vals[i]
	}
	return def
}

func (m *TTreeMap[K, V]) putIfAbsent(k K, v V) V {
	if old := m.get(k); !m.vc.isNull(old) {
		return old
	}
	m.put(k, v)
	return m.vc.null
}

func (m *TTreeMap[K, V]) computeIfAbsent(k K, f Function_e) V {
	if old := m.get(k); !m.vc.isNull(old) {
		return old
	}
	r := f.apply(m.kc.box(k))
	if r == nil {
		return m.vc.null
	}
	v := m.vc.unbox(r)
	m.put(k, v)
	return v
}

// each is the cursor of a for-each loop over the entries in key order (keySet(), values()).
func (m *TTreeMap[K, V]) each() ttreeMapEach_[K, V] { return ttreeMapEach_[K, V]{m: m} }

type ttreeMapEach_[K, V any] struct {
	m *TTreeMap[K, V]
	i int
	k K
	v V
}

func (e *ttreeMapEach_[K, V]) next() bool {
	if e.i < len(e.m.keys) {
		e.k, e.v = e.m.keys[e.i], e.m.vals[e.i]
		e.i++
		return true
	}
	return false
}

// All, Keys and Values walk the map as each did (tools/postflip/ranges -loops).
func (m *TTreeMap[K, V]) All() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for i := 0; i < len(m.keys); i++ {
			if !yield(m.keys[i], m.vals[i]) {
				return
			}
		}
	}
}

func (m *TTreeMap[K, V]) Keys() iter.Seq[K] { return sliceAll_(&m.keys) }

func (m *TTreeMap[K, V]) Values() iter.Seq[V] { return sliceAll_(&m.vals) }

func (m *TTreeMap[K, V]) asMap() NavigableMap {
	if m == nil {
		return nil
	}
	if m.view == nil {
		m.view = &ttreeMapView_[K, V]{m: m}
	}
	return m.view
}

// ttreeMapView_ is the erased face of a TTreeMap: TreeMap's methods on boxed keys and values;
// keySet, values and entrySet are copies, as TreeMap's are.
type ttreeMapView_[K, V any] struct{ m *TTreeMap[K, V] }

func (v *ttreeMapView_[K, V]) self() any              { return v }
func (v *ttreeMapView_[K, V]) comparator() Comparator_e { return v.m.ecmp }
func (v *ttreeMapView_[K, V]) size() int32            { return v.m.size() }
func (v *ttreeMapView_[K, V]) isEmpty() bool          { return v.m.isEmpty() }
func (v *ttreeMapView_[K, V]) clear()                 { v.m.clear() }
func (v *ttreeMapView_[K, V]) get(k any) any          { return v.m.vc.box(v.m.get(v.m.kc.unbox(k))) }
func (v *ttreeMapView_[K, V]) containsKey(k any) bool { return v.m.containsKey(v.m.kc.unbox(k)) }
func (v *ttreeMapView_[K, V]) put(k, x any) any {
	return v.m.vc.box(v.m.put(v.m.kc.unbox(k), v.m.vc.unbox(x)))
}
func (v *ttreeMapView_[K, V]) remove(k any) any  { return v.m.vc.box(v.m.remove(v.m.kc.unbox(k))) }
func (v *ttreeMapView_[K, V]) equals(o any) bool { return mapEquals_(v, o) }
func (v *ttreeMapView_[K, V]) hashCode() int32   { return mapHash_(v) }
func (v *ttreeMapView_[K, V]) toString() string  { return mapToString_(v) }

func (v *ttreeMapView_[K, V]) containsValue(x any) bool {
	for _, y := range v.m.vals {
		if objequals_(x, v.m.vc.box(y)) {
			return true
		}
	}
	return false
}

func (v *ttreeMapView_[K, V]) putAll(o Map) {
	for it := o.entrySet().iterator(); it.hasNext(); {
		e := it.next().(Map_Entry)
		v.put(e.getKey(), e.getValue())
	}
}

func (v *ttreeMapView_[K, V]) keySet() Set {
	s := new_LinkedHashSet()
	for _, k := range v.m.keys {
		s.add(v.m.kc.box(k))
	}
	return s
}

func (v *ttreeMapView_[K, V]) values() Collection {
	out := make([]any, len(v.m.vals))
	for i, x := range v.m.vals {
		out[i] = v.m.vc.box(x)
	}
	return listOf_(out)
}

func (v *ttreeMapView_[K, V]) entrySet() Set {
	s := new_LinkedHashSet()
	s.identity = true
	for i, k := range v.m.keys {
		s.add(&ttreeEntry_[K, V]{m: v.m, k: k, v: v.m.vals[i]})
	}
	return s
}

// ttreeEntry_ is an entry of entrySet: setValue writes the map, as TreeMap's shared entry did.
type ttreeEntry_[K, V any] struct {
	m *TTreeMap[K, V]
	k K
	v V
}

func (e *ttreeEntry_[K, V]) self() any     { return e }
func (e *ttreeEntry_[K, V]) getKey() any   { return e.m.kc.box(e.k) }
func (e *ttreeEntry_[K, V]) getValue() any { return e.m.vc.box(e.v) }
func (e *ttreeEntry_[K, V]) setValue(x any) any {
	old := e.getValue()
	e.v = e.m.vc.unbox(x)
	if i, ok := e.m.search(e.k); ok {
		e.m.vals[i] = e.v
	}
	return old
}
func (e *ttreeEntry_[K, V]) equals(o any) bool {
	x, ok := o.(Map_Entry)
	return ok && x != nil && objequals_(e.getKey(), x.getKey()) && objequals_(e.getValue(), x.getValue())
}
func (e *ttreeEntry_[K, V]) hashCode() int32 { return objhash_(e.getKey()) ^ objhash_(e.getValue()) }
func (e *ttreeEntry_[K, V]) toString() string {
	return objstr_(e.getKey()) + "=" + objstr_(e.getValue())
}

var (
	_ NavigableSet = (*ttreeSetView_[string])(nil)
	_ NavigableMap = (*ttreeMapView_[string, int32])(nil)
)
