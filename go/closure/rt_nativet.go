package closure

// The typed views of rule "typed library interfaces" for lists that tools/postflip/native makes
// slices (toolfix.sh, fix 44).

// sliceOfT_ is tlistOfT_(c, x), the immutable typed list of a place of class ImmutableList, as a
// slice that nothing changes: the list's items (an empty slice for an empty list), nil for null,
// and the same exception for a value that is no ImmutableList.
func sliceOfT_[T any](c *tconv_[T], x Collection) []T {
	l := tlistOfT_(c, x)
	if l == nil {
		return nil
	}
	if len(l.items) == 0 {
		return []T{}
	}
	return l.items
}

// slicePtrOfT_ is sliceOfT_ as a pointer to the slice; nil for null.
func slicePtrOfT_[T any](c *tconv_[T], x Collection) *[]T {
	s := sliceOfT_(c, x)
	if s == nil {
		return nil
	}
	return &s
}

// slicePtrViewT_ is p as the typed List of a call that only reads it (the list's asListT(),
// asCollT() or asIterable() given to a parameter of a typed library interface): slicePtrView_
// with the typed methods, reading the slice the pointer holds at every operation as the typed
// list's view read its items; nil for a null list.
func slicePtrViewT_[T any](c *tconv_[T], p *[]T) ListT[T] {
	if p == nil {
		return nil
	}
	return &slicePtrListT_[T]{slicePtrList_[T]{p: p, c: c}}
}

type slicePtrListT_[T any] struct{ slicePtrList_[T] }

func (v *slicePtrListT_[T]) iterator_t() IteratorT[T] {
	return &slicePtrIterTT_[T]{slicePtrIterT_[T]{p: v.p, c: v.c}}
}
func (v *slicePtrListT_[T]) get_t(i int32) T      { return sliceAt_(*v.p, i) }
func (v *slicePtrListT_[T]) add_t(x T) bool       { return v.add(v.c.box(x)) }
func (v *slicePtrListT_[T]) set_t(i int32, x T) T { return v.c.unbox(v.set(i, v.c.box(x))) }
func (v *slicePtrListT_[T]) remove_t(i int32) T   { return v.c.unbox(v.remove(i)) }

// slicePtrIterTT_ is slicePtrIterT_ with the typed next.
type slicePtrIterTT_[T any] struct{ slicePtrIterT_[T] }

func (it *slicePtrIterTT_[T]) next_t() T { return sliceNext_(*it.p, &it.i) }
