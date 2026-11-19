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
