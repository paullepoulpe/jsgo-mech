package closure

// Sorted typed maps and sets in the types of the hash ones (j2go rule "typed collections"): a
// TreeSet<K> or a TreeMap<K, V> in a declaration that the translator types is a *TSet[K] or a
// *TMap[K, V] in its sorted mode, so that a Set<K> declaration holds a LinkedHashSet or a
// TreeSet in one Go type, as it does in Java. (The sorted collections that post-flip step
// colls -kind treeset / treemap types have types of their own: rt_ttree.go.)
//
// A sorted map keeps its keys in the order of its comparator (or their natural ordering) and
// finds them by binary search with it, as the erased TreeSet and TreeMap of rt_coll2.go and the
// TTreeSet and TTreeMap of rt_ttree.go do (the same comparison, ttreeCmp_, and the same search,
// ttreeSearch_): natural ordering compares two strings as they are; any other comparison is
// made on the boxes, as erased code makes it. A removed mapping leaves no dead position: it
// is taken out of the slices, as those collections take it out.
//
// Erased code sees a sorted collection that can change through its sorted view (asSet, asMap,
// asSorted): the view of the hash mode with the methods of NavigableSet, the class name
// TreeSet / TreeMap, and lookups that compare (an object of another class throws
// ClassCastException, as the comparison of a TreeSet does). An immutable sorted collection
// (an ImmutableSortedSet or ImmutableSortedMap that erased code made, entering a typed
// declaration: tsetImm_, tmapImm_) is seen as that collection, its twin.

// tsort_ is what a sorted typed map or set has beside its mappings (tmapExt_.sort).
type tsort_[K any] struct {
	ecmp Comparator_e       // the comparator given; nil: natural ordering
	cmp  func(a, b K) int32 // the comparison of two keys (ttreeCmp_)
	view any                // the collection as erased code sees it (sortedView), once that is needed
}

// sorted reports whether the map (or set) is in its sorted mode.
func (m *TMap[K, V]) sorted() bool { return m.x != nil && m.x.sort != nil }

// setSorted_ puts an empty map (or set) in its sorted mode.
func (m *TMap[K, V]) setSorted_(cmp Comparator_e) {
	m.ext().sort = &tsort_[K]{ecmp: cmp, cmp: ttreeCmp_(m.kc, cmp)}
}

// newTSortedSet_ is new TreeSet<>().
func newTSortedSet_[K comparable](kc *tconv_[K]) *TSet[K] { return newTSortedSetC_(kc, nil) }

// newTSortedSetC_ is new TreeSet<>(comparator).
func newTSortedSetC_[K comparable](kc *tconv_[K], cmp Comparator_e) *TSet[K] {
	s := newTSet_(kc)
	s.setSorted_(cmp)
	return s
}

// newTSortedMap_ is new TreeMap<>().
func newTSortedMap_[K comparable, V any](kc *tconv_[K], vc *tconv_[V]) *TMap[K, V] {
	m := newTMap_(kc, vc)
	m.setSorted_(nil)
	return m
}

// findSorted is find of a sorted map.
func (m *TMap[K, V]) findSorted(k K) int32 {
	if i, ok := ttreeSearch_(m.keys, m.x.sort.cmp, k); ok {
		return int32(i)
	}
	return -1
}

// putSorted is Put of a sorted map (that can change): a new key goes to its place in the order.
func (m *TMap[K, V]) putSorted(k K, v V) V {
	i, ok := ttreeSearch_(m.keys, m.x.sort.cmp, k)
	if ok {
		old := m.vals[i]
		m.vals[i] = v
		return old
	}
	var zk K
	var zv V
	m.keys, m.vals = append(m.keys, zk), append(m.vals, zv)
	copy(m.keys[i+1:], m.keys[i:])
	copy(m.vals[i+1:], m.vals[i:])
	m.keys[i], m.vals[i] = k, v
	m.live++
	return m.vc.null
}

// removeSorted is removeAt of a sorted map: the mapping at position i is taken out.
func (m *TMap[K, V]) removeSorted(i int) {
	var zk K
	var zv V
	n := len(m.keys) - 1
	copy(m.keys[i:], m.keys[i+1:])
	copy(m.vals[i:], m.vals[i+1:])
	m.keys[n], m.vals[n] = zk, zv
	m.keys, m.vals = m.keys[:n], m.vals[:n]
	m.live--
}

// ---- sets

// asSorted is a sorted set as erased code sees it (a place of type SortedSet or
// NavigableSet): nil for null; a set that is not sorted is not one (ClassCastException).
func (s *TSet[K]) asSorted() NavigableSet {
	if s == nil {
		return nil
	}
	if !s.sorted() {
		classCastImm_("java.util.LinkedHashSet", "java.util.SortedSet")
	}
	if s.imm {
		// an immutable sorted set: its twin (an ImmutableSortedSet)
		return s.asImm()
	}
	return s.sortedView()
}

// sortedView is the one erased view of a sorted set that can change.
func (s *TSet[K]) sortedView() *tsortedSetView_[K] {
	v, _ := s.x.sort.view.(*tsortedSetView_[K])
	if v == nil {
		v = &tsortedSetView_[K]{tsetView_[K]{m: &s.TMap, s: s}}
		s.x.sort.view = v
	}
	return v
}

// tsortedSetView_ is a sorted TSet seen as an erased TreeSet: the view of a set, with the
// methods of a sorted set that erased code calls.
type tsortedSetView_[K comparable] struct {
	tsetView_[K]
}

func (v *tsortedSetView_[K]) self() any       { return v }
func (v *tsortedSetView_[K]) goName_() string { return "TreeSet" }

// contains and remove compare x with the elements: an object of another class throws
// ClassCastException there, as the comparison of a TreeSet does.
func (v *tsortedSetView_[K]) contains(x any) bool      { return v.m.find(v.m.kc.unbox(x)) >= 0 }
func (v *tsortedSetView_[K]) remove_Object(x any) bool { return v.s.Delete(v.m.kc.unbox(x)) }
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
	m := v.m
	out := &TreeSet{cmp: cmpReversed_(Comparator_of_(func(a, b any) int32 {
		return m.x.sort.cmp(m.kc.unbox(a), m.kc.unbox(b))
	}))}
	for i := len(m.keys) - 1; i >= 0; i-- {
		out.items = append(out.items, m.kc.box(m.keys[i]))
	}
	return out
}

func (v *tsortedSetView_[K]) tailSet(from any, inclusive bool) NavigableSet {
	m := v.m
	i, ok := ttreeSearch_(m.keys, m.x.sort.cmp, m.kc.unbox(from))
	if ok && !inclusive {
		i++
	}
	out := &TreeSet{cmp: m.x.sort.ecmp}
	for ; i < len(m.keys); i++ {
		out.items = append(out.items, m.kc.box(m.keys[i]))
	}
	return out
}

// ---- maps

// sortedView is the one erased view of a sorted map that can change.
func (m *TMap[K, V]) sortedView() *tsortedMapView_[K, V] {
	v, _ := m.x.sort.view.(*tsortedMapView_[K, V])
	if v == nil {
		v = &tsortedMapView_[K, V]{tmapView_[K, V]{m: m}}
		m.x.sort.view = v
	}
	return v
}

// tsortedMapView_ is a sorted TMap seen as an erased TreeMap.
type tsortedMapView_[K comparable, V any] struct {
	tmapView_[K, V]
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
func (v *tsortedMapView_[K, V]) remove(x any) any       { return v.m.vc.box(v.m.Delete(v.m.kc.unbox(x))) }
func (v *tsortedMapView_[K, V]) equals(o any) bool      { return mapEquals_(v, o) }
func (v *tsortedMapView_[K, V]) hashCode() int32        { return mapHash_(v) }
func (v *tsortedMapView_[K, V]) toString() string       { return mapToString_(v) }

var (
	_ NavigableSet = (*tsortedSetView_[string])(nil)
	_ NavigableMap = (*tsortedMapView_[string, int32])(nil)
)
