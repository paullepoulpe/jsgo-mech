package closure

import "sort"

// Rule "typed library interfaces", piece `freshloop` (tools/j2go StmtEmitter.freshLoop; RESIDUE8
// T2.1 and T1.8, docs/libgen/COLLECTIONS-PLAN.md section 16): a for-each loop over a fresh
// immutable copy (`ImmutableList.copyOf(c)`, `ImmutableList.of(a, b)`, `sortedCopyOf(c)`), which
// nothing but the loop reads, makes the typed copy at the loop's element type and runs on its
// cursor; a loop over `Lists.reverse(l)` of an ImmutableList runs on a typed cursor backwards.
// The erased loop made the erased copy of the boxes and cast each element back.

// immutableListCopyOfIt_ is ImmutableList.copyOf(from) of a typed Iterable of the list's element
// type, read through its typed cursor (eachOf_), where immutableListCopyOf_ reads the erased
// collection and unboxes each element; a null element throws NullPointerException as
// checkNoNulls_ does (ImmutableList_copyOf).
func immutableListCopyOfIt_[T any](c *tconv_[T], from IterableT[T]) *TList[T] {
	if v, ok := from.(*tlistView_[T]); ok {
		return immutableListCopyOfT_(c, v.l)
	}
	var items []T
	for e := eachOf_(from); e.next(); {
		items = append(items, e.v)
	}
	for i, x := range items {
		if immNull_(c, x) {
			throwNPE_("at index " + istr_(int64(i)))
		}
	}
	if len(items) == 0 {
		return tlistEmptyImm_(c)
	}
	return tlistOwn_(c, items)
}

// immutableListSortedCopyOfIt_ is ImmutableList.sortedCopyOf(from) (natural ordering) of a typed
// Iterable: the elements in the order sortSlice_ gives their boxes (the same stable sort, the
// same comparisons: tsortCmp_ of natural ordering compares two strings as compareNatural_ does,
// any other pair through compareTo_t or their boxes).
func immutableListSortedCopyOfIt_[T any](c *tconv_[T], from IterableT[T]) *TList[T] {
	var items []T
	for e := eachOf_(from); e.next(); {
		items = append(items, e.v)
	}
	return tlistSorted_(c, items)
}

// immutableListSortedCopyOf_ is the same of a collection as erased code sees it.
func immutableListSortedCopyOf_[T any](c *tconv_[T], from Iterable) *TList[T] {
	src := toSlice_(from)
	checkNoNulls_(src)
	items := make([]T, len(src))
	for i, x := range src {
		items[i] = c.unbox(x)
	}
	return tlistSorted_(c, items)
}

func tlistSorted_[T any](c *tconv_[T], items []T) *TList[T] {
	for i, x := range items {
		if immNull_(c, x) {
			throwNPE_("at index " + istr_(int64(i)))
		}
	}
	nat := &teq_{sorted: true}
	sort.SliceStable(items, func(i, j int) bool { return tsortCmp_(nat, c, items[i], items[j]) < 0 })
	if len(items) == 0 {
		return tlistEmptyImm_(c)
	}
	return tlistOwn_(c, items)
}

// sliceRevEach_ is the cursor of a for-each loop over Lists.reverse(l) of an immutable list l:
// its elements from the last (the list cannot change while the loop runs).
type sliceRevEach_[T any] struct {
	src []T
	i   int
	v   T
}

func (e *sliceRevEach_[T]) next() bool {
	if e.i > 0 {
		e.i--
		e.v = e.src[e.i]
		return true
	}
	return false
}

// tlistRevEach_ is that cursor for typed list l; a null list throws NullPointerException here,
// where Lists.reverse read it.
func tlistRevEach_[T any](l *TList[T]) sliceRevEach_[T] {
	if l == nil {
		panic(new_NullPointerException_0().self())
	}
	return sliceRevEach_[T]{src: l.items, i: len(l.items)}
}
