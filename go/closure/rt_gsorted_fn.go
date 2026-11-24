//go:build !j2go_nofntypes

package closure

import "sort"

// Piece `gsorted` (rt_gsorted.go): ImmutableList.sortedCopyOf with a comparator at the element type
// (rule "function types": Comparator[T] is a Go func type only with the rule on).

// immutableListSortedCopyOfCT_ is ImmutableList.sortedCopyOf(cmp, from) with the comparator at
// the element type (ImmutableList_sortedCopyOf with its erased form: a null comparator is natural
// ordering there too).
func immutableListSortedCopyOfCT_[T any](c *tconv_[T], cmp Comparator[T], from Iterable) *TList[T] {
	src := toSlice_(from)
	checkNoNulls_(src)
	items := make([]T, len(src))
	for i, x := range src {
		items[i] = c.unbox(x)
	}
	if cmp == nil {
		return tlistSorted_(c, items)
	}
	sort.SliceStable(items, func(i, j int) bool { return cmp(items[i], items[j]) < 0 })
	if len(items) == 0 {
		return tlistEmptyImm_(c)
	}
	return tlistOwn_(c, items)
}

// immutableListSortedCopyOfCTIt_ is the same of a typed Iterable of the element type.
func immutableListSortedCopyOfCTIt_[T any](c *tconv_[T], cmp Comparator[T], from IterableT[T]) *TList[T] {
	var items []T
	for e := eachOf_(from); e.next(); {
		items = append(items, e.v)
	}
	for i, x := range items {
		if immNull_(c, x) {
			throwNPE_("at index " + istr_(int64(i)))
		}
	}
	if cmp == nil {
		return tlistSorted_(c, items)
	}
	sort.SliceStable(items, func(i, j int) bool { return cmp(items[i], items[j]) < 0 })
	if len(items) == 0 {
		return tlistEmptyImm_(c)
	}
	return tlistOwn_(c, items)
}
