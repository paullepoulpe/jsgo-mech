package closure

import "sort"

// Rule "typed library interfaces", piece `gsorted` (tools/j2go ExprEmitter.gsortedOf,
// StmtEmitter.freshLoop; RESIDUE8 T2.2, T1.14, T2.19; docs/libgen/COLLECTIONS-PLAN.md section 16):
// Guava's sorted immutables made for a typed place are typed: an ImmutableSortedSet is the typed
// set in its sorted mode (rt_tsorted.go) whose twin, made when erased code first needs it, is the
// runtime's ImmutableSortedSet of the same elements and comparator (TSet.asImm); an
// ImmutableSortedMap the same of the typed map; ImmutableList.sortedCopyOf the typed list. Each
// maker sorts and checks as the runtime's erased maker does (immSorted_, immMap_ of imSorted_,
// ImmutableList_sortedCopyOf): nulls first with the same messages, the same stable sort with the
// same comparisons (tsortCmp_: the comparator at the element type where the place has one, else
// its erased form, else natural ordering, which compares two strings as compareNatural_ does and
// anything else through compareTo_t or the boxes), the first of equal elements kept.

// tsortedSetMake_ is the immutable sorted set of items (which it owns) in the order of cmp
// (cmpT: the same at the element type, or nil; both nil: natural ordering), as immSorted_.
func tsortedSetMake_[K comparable](c *tconv_[K], cmp Comparator_e, cmpT any, items []K) *TSet[K] {
	for i, x := range items {
		if immNull_(c, x) {
			throwNPE_("at index " + istr_(int64(i)))
		}
	}
	eq := &teq_{sorted: true, cmp: cmp, cmpT: cmpT}
	sort.SliceStable(items, func(i, j int) bool { return tsortCmp_(eq, c, items[i], items[j]) < 0 })
	out := items[:0]
	for _, v := range items {
		if len(out) == 0 || tsortCmp_(eq, c, out[len(out)-1], v) != 0 {
			out = append(out, v)
		}
	}
	s := &TSet[K]{c: c, eq: eq, keys: out, live: int32(len(out)), twin: immPending_}
	s.view.s = s
	return s
}

// tsortedSetCopyOf_ is ImmutableSortedSet.copyOf(from) (natural ordering) of a collection as
// erased code sees it: an ImmutableSortedSet of natural ordering is itself (its typed set).
func tsortedSetCopyOf_[K comparable](c *tconv_[K], from Collection) *TSet[K] {
	if ic, ok := from.(*ImmutableCollection); ok && ic.kind == icSorted_ && ic.cmp == nil {
		return tsetImm_(c, ic)
	}
	src := toSlice_(from)
	checkNoNulls_(src)
	items := make([]K, len(src))
	for i, x := range src {
		items[i] = c.unbox(x)
	}
	return tsortedSetMake_(c, nil, nil, items)
}

// tsortedSetCopyOfIt_ is the same of a typed Iterable of the element type.
func tsortedSetCopyOfIt_[K comparable](c *tconv_[K], from IterableT[K]) *TSet[K] {
	if v, ok := from.(*tsetView_[K]); ok && v.s.twin != nil && v.s.twin != sortedMark_ && v.s.eq != nil && v.s.eq.sorted && v.s.eq.cmp == nil && v.s.eq.cmpT == nil {
		return v.s
	}
	var items []K
	for e := eachOf_(from); e.next(); {
		items = append(items, e.v)
	}
	return tsortedSetMake_(c, nil, nil, items)
}

// tsortedSetCopyOfSorted_ is ImmutableSortedSet.copyOfSorted(s) of a typed sorted set: an
// immutable one is itself; any other is copied with its comparator.
func tsortedSetCopyOfSorted_[K comparable](c *tconv_[K], s *TSet[K]) *TSet[K] {
	if s.eq == nil || !s.eq.sorted {
		// (not sorted: asSorted's ClassCastException, where erased code cast it)
		s.asSorted()
	}
	if s.twin != sortedMark_ {
		return s
	}
	return tsortedSetMake_(c, s.eq.cmp, s.eq.cmpT, append([]K(nil), s.liveKeys()...))
}

// tsortedMapMake_ is the immutable sorted map of parallel keys and values in the order of cmp
// (nil: natural ordering), as immMap_ makes an ImmutableSortedMap: nulls are checked first, a
// key equal to another by the comparator throws IllegalArgumentException.
func tsortedMapMake_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], cmp Comparator_e, keys []K, vals []V) *TMap[K, V] {
	for i := range keys {
		if immNull_(kc, keys[i]) {
			throwNPE_("null key in entry: null=" + objstr_(vc.box(vals[i])))
		}
		if immNull_(vc, vals[i]) {
			throwNPE_("null value in entry: " + objstr_(kc.box(keys[i])) + "=null")
		}
	}
	eq := &teq_{sorted: true, cmp: cmp}
	order := make([]int, len(keys))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return tsortCmp_(eq, kc, keys[order[a]], keys[order[b]]) < 0 })
	m := &TMap[K, V]{kc: kc, vc: vc, eq: eq, imm: immPendingMap_}
	m.view.m = m
	for _, i := range order {
		n := len(m.keys)
		if n > 0 && tsortCmp_(eq, kc, m.keys[n-1], keys[i]) == 0 {
			dupKey_(kc.box(keys[i]), vc.box(m.vals[n-1]), vc.box(vals[i]))
		}
		m.keys = append(m.keys, keys[i])
		m.vals = append(m.vals, vals[i])
	}
	m.live = int32(len(m.keys))
	return m
}

// tsortedMapCopyOf_ is ImmutableSortedMap.copyOf(from) (natural ordering): an
// ImmutableSortedMap of natural ordering is itself (its typed map); a typed map's mappings are
// read as they are, any other map's through its entries (mapKV_).
func tsortedMapCopyOf_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], from Map) *TMap[K, V] {
	switch x := from.(type) {
	case *ImmutableMap:
		if x.kind == imSorted_ && x.cmp == nil {
			return tmapImm_(kc, vc, x)
		}
	case *tmapView_[K, V]:
		t := x.m
		if t.imm != nil && t.imm != sortedMapMark_ && t.eq != nil && t.eq.sorted && t.eq.cmp == nil && t.eq.cmpT == nil {
			return t
		}
		keys, vals := t.liveKV()
		return tsortedMapMake_(kc, vc, nil, append([]K(nil), keys...), append([]V(nil), vals...))
	}
	bk, bv := mapKV_(from)
	keys, vals := make([]K, len(bk)), make([]V, len(bv))
	for i := range bk {
		if bk[i] != nil {
			keys[i] = kc.unbox(bk[i])
		} else {
			keys[i] = kc.null
		}
		if bv[i] != nil {
			vals[i] = vc.unbox(bv[i])
		} else {
			vals[i] = vc.null
		}
	}
	return tsortedMapMake_(kc, vc, nil, keys, vals)
}
