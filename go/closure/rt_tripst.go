package closure

// The round trips of trips (rt_timm.go) through the views of a typed set for typed code (rule
// "typed library interfaces"): S.asSetT().addAll(c), also S.asCollT().addAll(c). For a set that
// can change, asSetT() is the view asSet() is (the sorted view for a sorted set), so the tool's
// helpers are the call. For an immutable set asSet() is the erased twin, whose addAll throws
// UnsupportedOperationException always, and asSetT() the typed view, which throws when it adds
// the first element: there the call itself is made.

// tsetAddAllOfT_ is s.asSetT().addAll(c).
func tsetAddAllOfT_[K comparable](s *TSet[K], c Collection) bool {
	if s.imm {
		return s.asSetT().addAll(c)
	}
	return tsetAddAllOf_(s, c)
}

// tsetAddSetT_ is s.asSetT().addAll(t.asSet()) (or a view of t for typed code) of two typed sets.
func tsetAddSetT_[K comparable](s, t *TSet[K]) bool {
	if s.imm {
		return s.asSetT().addAll(Collection(t.asSet()))
	}
	return tsetAddSet_(s, t)
}

// tsetAddListT_ is s.asSetT().addAll(t.asList()) (or a view of t for typed code) of a typed set
// and a typed list.
func tsetAddListT_[K comparable](s *TSet[K], t *TList[K]) bool {
	if s.imm {
		return s.asSetT().addAll(Collection(t.asList()))
	}
	return tsetAddList_(s, t)
}
