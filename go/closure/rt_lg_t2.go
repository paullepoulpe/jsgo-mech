package closure

// Rule "typed library interfaces", the kinds of rule candidate T2 (docs/javaisms6/JAVAISMS6.md;
// tools/j2go LibGen.CLASSES_T2, LibGen.MAKERS_T2): the typed makers of the library types that
// the earlier kinds did not reach. Each makes the typed collection of the same kind as the
// runtime's erased one, for a place that holds the typed interface.

// ---- kind `enumset`: EnumSet is the runtime's TreeSet ordered by ordinal (rt_coll2.go); its
// typed twin is the typed set in its sorted mode with the same comparator (rt_tsorted.go).

// tenumSetNoneOf_ is EnumSet.noneOf(c).
func tenumSetNoneOf_[K comparable](c *tconv_[K], cl *Class) *TSet[K] {
	return newTSortedSetC_(c, ordinalCmp_())
}

// tenumSetAllOf_ is EnumSet.allOf(c): the constants in their declaration order.
func tenumSetAllOf_[K comparable](c *tconv_[K], cl *Class) *TSet[K] {
	s := tenumSetNoneOf_(c, cl)
	for _, x := range enumRegistry_[cl.name_] {
		s.addSorted(c.unbox(x))
	}
	return s
}

// tenumSetOf_ is EnumSet.of(e).
func tenumSetOf_[K comparable](c *tconv_[K], e *Enum) *TSet[K] {
	s := tenumSetNoneOf_(c, nil)
	s.addSorted(c.unbox(e.self()))
	return s
}

// tenumSetOfArr_ is EnumSet.of(e, rest...).
func tenumSetOfArr_[K comparable](c *tconv_[K], e *Enum, rest []*Enum) *TSet[K] {
	s := tenumSetOf_(c, e)
	for _, x := range rest {
		s.addSorted(c.unbox(x.self()))
	}
	return s
}

// tenumSetCopyOf_ is EnumSet.copyOf(collection).
func tenumSetCopyOf_[K comparable](c *tconv_[K], from Collection) *TSet[K] {
	s := tenumSetNoneOf_(c, nil)
	for _, x := range toSlice_(from) {
		s.addSorted(c.unbox(x))
	}
	return s
}
