package closure

import "sort"

// Typed sorted collections (j2go rule "typed collections"): a TreeSet<K> or a TreeMap<K, V> in
// a typed declaration is the typed set or map of rt_tset.go and rt_tmap.go in its sorted
// mode (teq_.sorted): the keys are kept in the order of the comparator (or their natural
// ordering) and found by binary search with it, as the erased TreeSet and TreeMap of
// rt_coll2.go find theirs. So a Set<K> declaration holds a LinkedHashSet or a TreeSet in one
// Go type, as it does in Java. Natural ordering compares two strings as they are; any other
// comparison is made on the boxes, as erased code makes it.
//
// A removed key leaves a dead position, as in the hash mode (a cursor keeps its place), but
// the key stays there until the collection is compacted: it orders the positions around it.
//
// Erased code sees a sorted collection through its sorted view (asSorted, which asSet and
// asMap return for it): the view of the hash mode with the methods of NavigableSet, and the
// class name TreeSet / TreeMap.

// sortedMark_ and sortedMapMark_ are the `twin` of a sorted set and the `imm` of a sorted map
// that can change (so that asSet and asMap take their slow path for them).
var sortedMark_ = &ImmutableCollection{}
var sortedMapMark_ = &timmMap_{}

// tsortCmp_ compares two keys of a sorted collection: with its comparator, or by natural
// ordering (a null key throws NullPointerException there, through compareNatural_).
func tsortCmp_[K any](x *teq_, c *jtconv_[K], a, b K) int32 {
	if x.cmp != nil {
		return x.cmp.compare(c.box(a), c.box(b))
	}
	if p, ok := any(&a).(*string); ok {
		if q := any(&b).(*string); *p != nullstr_ && *q != nullstr_ {
			return String_compareTo(*p, *q)
		}
	}
	return compareNatural_(c.box(a), c.box(b))
}

// tsortSearch_ returns the first position whose key is not less than k, and whether the key
// there equals k (TreeSet.search in rt_coll2.go; the position may be dead).
func tsortSearch_[K any](x *teq_, c *jtconv_[K], keys []K, k K) (int, bool) {
	i := sort.Search(len(keys), func(i int) bool { return tsortCmp_(x, c, keys[i], k) >= 0 })
	return i, i < len(keys) && tsortCmp_(x, c, keys[i], k) == 0
}

// ---- sets

// newTSortedSet_ is new TreeSet<>().
func newTSortedSet_[K comparable](c *jtconv_[K]) *JTSet[K] { return newTSortedSetC_(c, nil) }

// newTSortedSetC_ is new TreeSet<>(comparator).
func newTSortedSetC_[K comparable](c *jtconv_[K], cmp Comparator) *JTSet[K] {
	s := &JTSet[K]{c: c, eq: &teq_{sorted: true, cmp: cmp}, twin: sortedMark_}
	s.view.s = s
	return s
}

// tsortedSetFrom_ is new TreeSet<>(collection): its elements in their natural ordering.
func tsortedSetFrom_[K comparable](c *jtconv_[K], from Collection) *JTSet[K] {
	s := newTSortedSetC_(c, nil)
	for _, x := range toSlice_(from) {
		s.addSorted(c.unbox(x))
	}
	return s
}

// tsortedSetCopy_ is new TreeSet<>(collection) of a typed set of the same element type.
func tsortedSetCopy_[K comparable](c *jtconv_[K], from *JTSet[K]) *JTSet[K] {
	s := newTSortedSetC_(c, nil)
	for i, k := range from.keys {
		if !from.dead(i) {
			s.addSorted(k)
		}
	}
	return s
}

// addSorted is add of a sorted set.
func (s *JTSet[K]) addSorted(k K) bool {
	if n := len(s.keys) - int(s.live); n > hsmall_ && n > int(s.live) {
		s.compact()
	}
	i, ok := tsortSearch_(s.eq, s.c, s.keys, k)
	if ok {
		if !s.dead(i) {
			return false
		}
		// the position of an equal element that was removed
		s.gone[i] = false
		s.keys[i] = k
		s.live++
		return true
	}
	var z K
	s.keys = append(s.keys, z)
	copy(s.keys[i+1:], s.keys[i:])
	s.keys[i] = k
	if s.gone != nil {
		s.gone = append(s.gone, false)
		copy(s.gone[i+1:], s.gone[i:])
		s.gone[i] = false
	}
	s.live++
	return true
}

// asSorted is a sorted set as erased code sees it (a place of type SortedSet or
// NavigableSet): nil for null; a set that is not sorted is not one (ClassCastException).
func (s *JTSet[K]) asSorted() NavigableSet {
	if s == nil {
		return nil
	}
	if s.eq == nil || !s.eq.sorted {
		classCastImm_("java.util.LinkedHashSet", "java.util.SortedSet")
	}
	if s.twin != sortedMark_ {
		// an immutable sorted set: its twin (an ImmutableSortedSet)
		return s.asImm()
	}
	return s.sortedView()
}

// sortedView is the one erased view of a sorted set that can change.
func (s *JTSet[K]) sortedView() *tsortedSetView_[K] {
	v, _ := s.eq.view.(*tsortedSetView_[K])
	if v == nil {
		v = &tsortedSetView_[K]{}
		v.s = s
		s.eq.view = v
	}
	return v
}

// tsortedSetView_ is a sorted JTSet seen as an erased TreeSet: the view of a set, with the
// methods of a sorted set that erased code calls.
type tsortedSetView_[K comparable] struct {
	jtsetView_[K]
}

func (v *tsortedSetView_[K]) self() any       { return v }
func (v *tsortedSetView_[K]) goName_() string { return "TreeSet" }

// contains and remove compare x with the elements: an object of another class throws
// ClassCastException there, as the comparison of a TreeSet does.
func (v *tsortedSetView_[K]) contains(x any) bool      { return v.s.find(v.s.c.unbox(x)) >= 0 }
func (v *tsortedSetView_[K]) remove_Object(x any) bool { return v.s.remove(v.s.c.unbox(x)) }
func (v *tsortedSetView_[K]) equals(o any) bool        { return setEquals_(v, o) }
func (v *tsortedSetView_[K]) hashCode() int32          { return setHash_(v) }
func (v *tsortedSetView_[K]) toString() string         { return collToString_(v) }
func (v *tsortedSetView_[K]) containsAll(c Collection) bool {
	for it := c.iterator(); it.hasNext(); {
		if !v.contains(it.next()) {
			return false
		}
	}
	return true
}

// descendingSet and tailSet are, as the erased TreeSet's, new sets of the elements as they
// are now.
func (v *tsortedSetView_[K]) descendingSet() NavigableSet {
	s := v.s
	out := &TreeSet{cmp: Comparator_reversed(&cmpFunc_{func(a, b any) int32 {
		return tsortCmp_(s.eq, s.c, s.c.unbox(a), s.c.unbox(b))
	}})}
	for i := len(s.keys) - 1; i >= 0; i-- {
		if !s.dead(i) {
			out.items = append(out.items, s.c.box(s.keys[i]))
		}
	}
	return out
}

func (v *tsortedSetView_[K]) tailSet(from any, inclusive bool) NavigableSet {
	s := v.s
	i, ok := tsortSearch_(s.eq, s.c, s.keys, s.c.unbox(from))
	if ok && !inclusive {
		i++
	}
	out := &TreeSet{cmp: s.eq.cmp}
	for ; i < len(s.keys); i++ {
		if !s.dead(i) {
			out.items = append(out.items, s.c.box(s.keys[i]))
		}
	}
	return out
}

// ---- maps

// newTSortedMap_ is new TreeMap<>().
func newTSortedMap_[K comparable, V any](kc *jtconv_[K], vc *jtconv_[V]) *JTMap[K, V] {
	return newTSortedMapC_(kc, vc, nil)
}

// newTSortedMapC_ is new TreeMap<>(comparator).
func newTSortedMapC_[K comparable, V any](kc *jtconv_[K], vc *jtconv_[V], cmp Comparator) *JTMap[K, V] {
	m := &JTMap[K, V]{kc: kc, vc: vc, eq: &teq_{sorted: true, cmp: cmp}, imm: sortedMapMark_}
	m.view.m = m
	return m
}

// tsortedMapFrom_ is new TreeMap<>(map): its mappings in the natural ordering of the keys.
func tsortedMapFrom_[K comparable, V any](kc *jtconv_[K], vc *jtconv_[V], from Map) *JTMap[K, V] {
	m := newTSortedMapC_(kc, vc, nil)
	if from == nil {
		panic(new_NullPointerException_0().self())
	}
	m.view.putAll(from)
	return m
}

// tsortedMapCopy_ is new TreeMap<>(map) of a typed map of the same types.
func tsortedMapCopy_[K comparable, V any](kc *jtconv_[K], vc *jtconv_[V], from *JTMap[K, V]) *JTMap[K, V] {
	m := newTSortedMapC_(kc, vc, nil)
	m.putAllT(from)
	return m
}

// insertSorted adds a mapping at position i, the place of its key in the order (locate).
func (m *JTMap[K, V]) insertSorted(k K, v V, i int) {
	if n := len(m.keys) - int(m.live); n > hsmall_ && n > int(m.live) {
		m.compact()
		i, _ = tsortSearch_(m.eq, m.kc, m.keys, k)
	}
	if i < len(m.keys) && m.dead(i) && tsortCmp_(m.eq, m.kc, m.keys[i], k) == 0 {
		// the position of an equal key that was removed
		m.gone[i] = false
		m.keys[i], m.vals[i] = k, v
		m.live++
		return
	}
	var zk K
	var zv V
	m.keys, m.vals = append(m.keys, zk), append(m.vals, zv)
	copy(m.keys[i+1:], m.keys[i:])
	copy(m.vals[i+1:], m.vals[i:])
	m.keys[i], m.vals[i] = k, v
	if m.gone != nil {
		m.gone = append(m.gone, false)
		copy(m.gone[i+1:], m.gone[i:])
		m.gone[i] = false
	}
	m.live++
}

// asSorted is a sorted map as erased code sees it (a place of type SortedMap or
// NavigableMap): nil for null; a map that is not sorted is not one (ClassCastException).
func (m *JTMap[K, V]) asSorted() NavigableMap {
	if m == nil {
		return nil
	}
	if m.eq == nil || !m.eq.sorted {
		classCastImm_("java.util.LinkedHashMap", "java.util.SortedMap")
	}
	if m.imm != sortedMapMark_ {
		// an immutable sorted map: its twin (an ImmutableSortedMap)
		return m.asImm()
	}
	return m.sortedView()
}

// sortedView is the one erased view of a sorted map that can change.
func (m *JTMap[K, V]) sortedView() *tsortedMapView_[K, V] {
	v, _ := m.eq.view.(*tsortedMapView_[K, V])
	if v == nil {
		v = &tsortedMapView_[K, V]{}
		v.m = m
		m.eq.view = v
	}
	return v
}

// tsortedMapView_ is a sorted JTMap seen as an erased TreeMap.
type tsortedMapView_[K comparable, V any] struct {
	jtmapView_[K, V]
}

func (v *tsortedMapView_[K, V]) self() any       { return v }
func (v *tsortedMapView_[K, V]) goName_() string { return "TreeMap" }

// get, containsKey and remove compare x with the keys: an object of another class throws
// ClassCastException there, as the comparison of a TreeMap does.
func (v *tsortedMapView_[K, V]) get(x any) any {
	if i := v.m.find(v.m.kc.unbox(x)); i >= 0 {
		return v.m.vc.box(v.m.vals[i])
	}
	return nil
}
func (v *tsortedMapView_[K, V]) containsKey(x any) bool { return v.m.find(v.m.kc.unbox(x)) >= 0 }
func (v *tsortedMapView_[K, V]) remove(x any) any       { return v.m.vc.box(v.m.remove(v.m.kc.unbox(x))) }
func (v *tsortedMapView_[K, V]) equals(o any) bool      { return mapEquals_(v, o) }
func (v *tsortedMapView_[K, V]) hashCode() int32        { return mapHash_(v) }
func (v *tsortedMapView_[K, V]) toString() string       { return mapToString_(v) }
