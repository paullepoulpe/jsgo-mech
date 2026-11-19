package closure

// Typed sets (j2go rule "typed collections"): a LinkedHashSet<K> that the translator finds
// typable is a *JTSet[K]: its elements in insertion order, as K values, not boxes. K is a type whose Go == is
// Java's equals for the set: a String, a handle, an enum, a class without equals of its own.
//
// Erased code sees the set through asSet(), its one view (see JTList). Like the erased table
// (htable_), a small set is searched linearly and has no index, and a removed element leaves a
// dead position until the set is compacted, so that a cursor keeps its place.

type JTSet[K comparable] struct {
	keys []K
	gone []bool      // beside keys once an element has been removed: the dead positions
	idx  map[K]int32 // the position of each element; nil while the set is small
	live int32
	c    *jtconv_[K]
	eq   *teq_         // of elements compared by equals and hashCode (c.byEquals); else nil
	view jtsetView_[K] // the set as erased code sees it (asSet): part of the set
	// twin: nil for a hash set that can change. An immutable set (Guava's ImmutableSet,
	// rt_timm.go) has here the ImmutableSet that erased code sees it as, or immPending_ (see
	// JTList.twin); a sorted set that can change (a TreeSet, rt_tsorted.go) has sortedMark_.
	twin *ImmutableCollection
}

// jnewTSet_ is new LinkedHashSet<>().
func jnewTSet_[K comparable](c *jtconv_[K]) *JTSet[K] {
	s := &JTSet[K]{c: c}
	s.view.s = s
	return s
}

// newTSetN_ is new LinkedHashSet<>(n): the capacity is not used.
func newTSetN_[K comparable](c *jtconv_[K], n int32) *JTSet[K] { return jnewTSet_(c) }

// jtsetFrom_ is new LinkedHashSet<>(from).
func jtsetFrom_[K comparable](c *jtconv_[K], from Collection) *JTSet[K] {
	s := jnewTSet_(c)
	for _, x := range toSlice_(from) {
		s.add(c.unbox(x))
	}
	return s
}

// tsetCopy_ is new LinkedHashSet<>(from) of another typed set of the same element type.
func tsetCopy_[K comparable](c *jtconv_[K], from *JTSet[K]) *JTSet[K] {
	s := jnewTSet_(c)
	for i, k := range from.keys {
		if !from.dead(i) {
			s.add(k)
		}
	}
	return s
}

func (s *JTSet[K]) dead(i int) bool { return s.gone != nil && s.gone[i] }

// find returns the position of k, or -1.
func (s *JTSet[K]) find(k K) int32 {
	if s.eq != nil {
		// compared by equals and hashCode, or sorted
		if s.eq.sorted {
			if i, ok := tsortSearch_(s.eq, s.c, s.keys, k); ok && !s.dead(i) {
				return int32(i)
			}
			return -1
		}
		return teqFind_(s.eq, s.keys, s.gone, s.c, k, objhash_(s.c.box(k)))
	}
	if s.idx != nil {
		if i, ok := s.idx[k]; ok {
			return i
		}
		return -1
	}
	for i, x := range s.keys {
		if x == k && !s.dead(i) {
			return int32(i)
		}
	}
	return -1
}

func (s *JTSet[K]) size() int32   { return s.live }
func (s *JTSet[K]) isEmpty() bool { return s.live == 0 }

// has is contains.
func (s *JTSet[K]) contains(k K) bool { return s.find(k) >= 0 }

func (s *JTSet[K]) add(k K) bool {
	if s.twin != nil && s.twin != sortedMark_ {
		panic(frozen_{})
	}
	return s.add0(k)
}

// add0 is add for a set that can change, or that is being made.
func (s *JTSet[K]) add0(k K) bool {
	if s.eq != nil && s.eq.sorted {
		return s.addSorted(k)
	}
	var h int32
	if s.c.byEquals {
		h = objhash_(s.c.box(k))
		if teqFind_(s.eq, s.keys, s.gone, s.c, k, h) >= 0 {
			return false
		}
	} else if s.find(k) >= 0 {
		return false
	}
	if n := len(s.keys) - int(s.live); n > hsmall_ && n > int(s.live) {
		s.compact()
	}
	if s.keys == nil {
		s.keys = make([]K, 0, hfirst_)
	}
	s.keys = append(s.keys, k)
	if s.gone != nil {
		s.gone = append(s.gone, false)
	}
	s.live++
	if s.c.byEquals {
		if s.eq == nil {
			s.eq = &teq_{}
		}
		s.eq.add(h)
	} else if s.idx != nil {
		s.idx[k] = int32(len(s.keys) - 1)
	} else if len(s.keys) > hsmall_ {
		s.idx = make(map[K]int32, 2*len(s.keys))
		for i, x := range s.keys {
			if !s.dead(i) {
				s.idx[x] = int32(i)
			}
		}
	}
	return true
}

// compact drops the dead positions; the order of the elements is unchanged.
func (s *JTSet[K]) compact() {
	out := s.keys[:0]
	for i, x := range s.keys {
		if !s.gone[i] {
			out = append(out, x)
		}
	}
	clear(s.keys[len(out):])
	s.keys = out
	if s.eq != nil {
		s.eq.compact(s.gone)
	}
	s.gone = nil
	if s.idx != nil {
		clear(s.idx)
		for i, x := range s.keys {
			s.idx[x] = int32(i)
		}
	}
}

// del is remove(Object).
func (s *JTSet[K]) remove(k K) bool {
	if s.twin != nil && s.twin != sortedMark_ {
		panic(frozen_{})
	}
	i := s.find(k)
	if i < 0 {
		return false
	}
	s.delAt(int(i))
	return true
}

func (s *JTSet[K]) delAt(i int) {
	if s.idx != nil {
		delete(s.idx, s.keys[i])
	}
	if s.gone == nil {
		s.gone = make([]bool, len(s.keys), cap(s.keys))
	}
	s.gone[i] = true
	if s.eq == nil || !s.eq.sorted {
		// (a sorted set keeps the element at a dead position: the order of the positions)
		var z K
		s.keys[i] = z
	}
	s.live--
}

func (s *JTSet[K]) clear() {
	if s.twin != nil && s.twin != sortedMark_ {
		panic(frozen_{})
	}
	clear(s.keys)
	s.keys = s.keys[:0]
	s.gone = nil
	s.idx = nil
	if s.eq != nil && !s.eq.sorted {
		s.eq = nil
	}
	s.live = 0
}

// addAllT is addAll of another typed set of the same element type.
func (s *JTSet[K]) addAllT(o *JTSet[K]) bool {
	if s.twin != nil && s.twin != sortedMark_ {
		panic(frozen_{})
	}
	changed := false
	for i, k := range o.keys {
		if !o.dead(i) && s.add(k) {
			changed = true
		}
	}
	return changed
}

// jtsetEach_ is the cursor of a for-each loop over a typed set: its elements in insertion
// order, read position by position as the erased cursor reads its table.
type jtsetEach_[K comparable] struct {
	s *JTSet[K]
	i int
	v K
}

// each is the cursor of a for-each loop over the set; a null set throws NullPointerException
// here, where the loop asked for its iterator.
func (s *JTSet[K]) each() jtsetEach_[K] {
	_ = s.live
	return jtsetEach_[K]{s: s}
}

func (e *jtsetEach_[K]) next() bool {
	s := e.s
	for e.i < len(s.keys) {
		i := e.i
		e.i++
		if !s.dead(i) {
			e.v = s.keys[i]
			return true
		}
	}
	return false
}

// iterator is the set's Iterator (of boxed elements, with remove), as its view's.
func (s *JTSet[K]) iterator() Iterator {
	if s == nil {
		panic(new_NullPointerException_0().self())
	}
	return &tsetIter_[K]{s: s, last: -1}
}

// asSet is the set as an erased Set (nil for null): the set's one view, its identity for erased
// code, a field of the set (as JTList.asList).
func (s *JTSet[K]) asSet() Set {
	if s == nil || s.twin != nil {
		return s.asOtherSet()
	}
	return &s.view
}

// asSet_ is a set of the translated program as an erased Set, whether the translator made its
// declaration a typed set or not (for handwritten runtime code that calls generated code).
func asSet_(s any) Set {
	switch x := s.(type) {
	case Set:
		return x
	case interface{ asSet() Set }:
		return x.asSet()
	}
	return nil
}

// jtsetView_ is a JTSet seen as an erased Set.
type jtsetView_[K comparable] struct {
	s *JTSet[K]
}

// key is x as an element of the set, if it can be one (a lookup of an object of another class
// finds nothing, as in Java).
func (v *jtsetView_[K]) key(x any) (k K, ok bool) {
	if k, ok := x.(K); ok {
		return k, true
	}
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return v.s.c.unbox(x), true
}

// goName_ makes getClass() of the view java.util.LinkedHashSet, the class of the Java object.
func (v *jtsetView_[K]) goName_() string { return "LinkedHashSet" }

func (v *jtsetView_[K]) self() any     { return v }
func (v *jtsetView_[K]) size() int32   { return v.s.live }
func (v *jtsetView_[K]) isEmpty() bool { return v.s.live == 0 }
func (v *jtsetView_[K]) clear()        { v.s.clear() }
func (v *jtsetView_[K]) add(x any) bool {
	return v.s.add(v.s.c.unbox(x))
}

func (v *jtsetView_[K]) addAll(c Collection) bool {
	if o, ok := c.(*jtsetView_[K]); ok && o != nil {
		return v.s.addAllT(o.s)
	}
	changed := false
	for _, x := range toSlice_(c) {
		if v.add(x) {
			changed = true
		}
	}
	return changed
}

func (v *jtsetView_[K]) contains(x any) bool {
	k, ok := v.key(x)
	return ok && v.s.find(k) >= 0
}

func (v *jtsetView_[K]) remove_Object(x any) bool {
	k, ok := v.key(x)
	return ok && v.s.remove(k)
}

// strs is the set as a set of strings, if it is one (a String is looked up without its box).
func (v *jtsetView_[K]) strs() *JTSet[string] {
	s, _ := any(v.s).(*JTSet[string])
	return s
}

// contains_s, add_s and remove_s are the operations on a String that is not null, without its
// box (Collection_contains_s...).
func (v *jtsetView_[K]) contains_s(k string) bool {
	if s := v.strs(); s != nil {
		return s.find(k) >= 0
	}
	return v.contains(strobj_(k))
}

func (v *jtsetView_[K]) add_s(k string) bool {
	if s := v.strs(); s != nil {
		return s.add(k)
	}
	return v.add(strobj_(k))
}

func (v *jtsetView_[K]) remove_s(k string) bool {
	if s := v.strs(); s != nil {
		return s.remove(k)
	}
	return v.remove_Object(strobj_(k))
}

func (v *jtsetView_[K]) containsAll(c Collection) bool {
	for it := c.iterator(); it.hasNext(); {
		if !v.contains(it.next()) {
			return false
		}
	}
	return true
}

func (v *jtsetView_[K]) removeAll(c Collection) bool {
	changed := false
	for i := range v.s.keys {
		if !v.s.dead(i) && c.contains(v.s.c.box(v.s.keys[i])) {
			v.s.delAt(i)
			changed = true
		}
	}
	return changed
}

func (v *jtsetView_[K]) iterator() Iterator { return &tsetIter_[K]{s: v.s, last: -1} }

func (v *jtsetView_[K]) toArray_0() []any {
	out := make([]any, 0, v.s.live)
	for i, k := range v.s.keys {
		if !v.s.dead(i) {
			out = append(out, v.s.c.box(k))
		}
	}
	return out
}

func (v *jtsetView_[K]) toArray(a []any) []any { return fillArray_(v.toArray_0(), a) }
func (v *jtsetView_[K]) equals(o any) bool     { return setEquals_(v, o) }
func (v *jtsetView_[K]) hashCode() int32       { return setHash_(v) }
func (v *jtsetView_[K]) toString() string      { return collToString_(v) }

// step_ is for the cursor of a for-each loop in erased code (foreach_).
func (v *jtsetView_[K]) step_(i int) (any, int) {
	s := v.s
	for ; i < len(s.keys); i++ {
		if !s.dead(i) {
			return s.c.box(s.keys[i]), i + 1
		}
	}
	return nil, -1
}

// tsetIter_ iterates over a JTSet for erased code (Iterator, with remove).
type tsetIter_[K comparable] struct {
	s    *JTSet[K]
	i    int
	last int
}

func (it *tsetIter_[K]) hasNext() bool {
	s := it.s
	for it.i < len(s.keys) && s.dead(it.i) {
		it.i++
	}
	return it.i < len(s.keys)
}

func (it *tsetIter_[K]) next() any {
	if !it.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	it.last = it.i
	it.i++
	return it.s.c.box(it.s.keys[it.last])
}

func (it *tsetIter_[K]) remove() {
	if it.last < 0 {
		panic(new_IllegalStateException_0().self())
	}
	it.s.delAt(it.last)
	it.last = -1
}

// teq_ is what a typed set or map keeps for keys that are compared by equals and hashCode (a
// class with its own equals, an interface, a library class: jtconv_.byEquals), so that a key is
// found exactly when java.util.HashMap finds it: by the hash code it had when it was added,
// then by identity or equals. hs holds the hash codes beside the keys; once the table is not
// small, first and next chain the positions of equal hash codes, as the erased table does.
type teq_ struct {
	hs    []int32
	first map[int32]int32 // hash code -> 1 + the last position added with it
	next  []int32         // beside hs: 1 + the previous position with the same hash code, or 0
	// sorted: the set or map is a sorted one (a TreeSet, a TreeMap: rt_tsorted.go), its keys
	// in the order of cmp (nil: their natural ordering); hs, first and next are not used.
	sorted bool
	cmp    Comparator
	view   any // the sorted collection as erased code sees it, once that is needed
}

// teqFind_ returns the position of key k with hash code h among keys, or -1.
func teqFind_[K comparable](x *teq_, keys []K, gone []bool, c *jtconv_[K], k K, h int32) int32 {
	if x == nil {
		return -1
	}
	if x.first == nil {
		for i, kh := range x.hs {
			if kh == h && (gone == nil || !gone[i]) && (keys[i] == k || objequals_(c.box(k), c.box(keys[i]))) {
				return int32(i)
			}
		}
		return -1
	}
	for p := x.first[h]; p != 0; p = x.next[p-1] {
		i := p - 1
		if (gone == nil || !gone[i]) && (keys[i] == k || objequals_(c.box(k), c.box(keys[i]))) {
			return i
		}
	}
	return -1
}

// add records hash code h for the key just appended.
func (x *teq_) add(h int32) {
	if x.hs == nil {
		x.hs = make([]int32, 0, hfirst_)
	}
	x.hs = append(x.hs, h)
	if x.first != nil {
		x.next = append(x.next, x.first[h])
		x.first[h] = int32(len(x.hs))
	} else if len(x.hs) > hsmall_ {
		x.index()
	}
}

func (x *teq_) index() {
	x.first = make(map[int32]int32, 2*len(x.hs))
	x.next = make([]int32, len(x.hs), cap(x.hs))
	for i, h := range x.hs {
		x.next[i] = x.first[h]
		x.first[h] = int32(i + 1)
	}
}

// compact drops the hash codes of the dead positions.
func (x *teq_) compact(gone []bool) {
	n := 0
	for i, h := range x.hs {
		if !gone[i] {
			x.hs[n] = h
			n++
		}
	}
	x.hs = x.hs[:n]
	if x.first != nil {
		x.index()
	}
}
