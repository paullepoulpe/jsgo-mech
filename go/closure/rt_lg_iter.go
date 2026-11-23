package closure

import "iter"

// elemsT_ yields the elements of a typed Iterable as the for-each cursor jeachOf_ does (post-flip
// step ranges on the loops of rule "typed library interfaces"): it makes that cursor when the
// loop starts and steps it, so a typed collection of the runtime is read in place, a wrapped
// erased collection as foreach_ reads it, and anything else through its iterator_t().
func elemsT_[T any](x IterableT[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for e := jeachOf_(x); e.next(); {
			if !yield(e.v) {
				return
			}
		}
	}
}
