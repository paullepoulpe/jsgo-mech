package closure

// Typed collections held in typed containers (tools/postflip/colls, nested.go).
//
// A container that converts its elements (the *tconv_[V] of a typed map) boxes a typed
// collection as its view, the one object erased code sees it as (asList, asMap, asSet: a
// collection has one view, so its identity is kept), and unboxes a view to the typed
// collection behind it. Only collections of the class are ever stored there (colls proves
// it), so the view is always one of a typed collection of these types.

// An immutable typed collection (rt_timm.go) is seen by erased code as its twin, the
// runtime's immutable collection with the same elements (asList is asImm for it), not as a
// view: the twin keeps the typed collection it is the twin of (typed_), which is what it
// unboxes to. The empty twins (emptyImmList_, emptyImmSet_, emptyImmMap_: one object each,
// as in Guava) are shared by every element type, so a conversion remembers the empty
// immutable collection it boxed last and unboxes the empty twin to it.

// tlistOf_ is the typed list whose view v is (null for null), or whose twin v is.
func tlistOf_[T any](v any) *TList[T] {
	switch x := v.(type) {
	case nil:
		return nil
	case *ImmutableCollection:
		if l, ok := x.typed_.v.(*TList[T]); ok {
			return l
		}
	}
	return v.(*tlistView_[T]).tl()
}

// tmapOf_ is the typed map whose view v is, or whose twin v is.
func tmapOf_[K comparable, V any](v any) *TMap[K, V] {
	switch x := v.(type) {
	case nil:
		return nil
	case *ErasedImmutableMap:
		if m, ok := x.typed_.v.(*TMap[K, V]); ok {
			return m
		}
	}
	return v.(*tmapView_[K, V]).tm()
}

// tsetOf_ is the typed set whose view v is, or whose twin v is.
func tsetOf_[K comparable](v any) *TSet[K] {
	switch x := v.(type) {
	case nil:
		return nil
	case *ImmutableCollection:
		if s, ok := x.typed_.v.(*TSet[K]); ok {
			return s
		}
	}
	return v.(*tsetView_[K]).ts()
}

// tlistConv_ is the conversion of typed lists as elements of a typed container.
func tlistConv_[T any]() *tconv_[*TList[T]] {
	var empty *TList[T] // the empty immutable list boxed last
	return &tconv_[*TList[T]]{
		box: func(l *TList[T]) any {
			if l == nil {
				return nil
			}
			v := l.asList()
			if v == List(emptyImmList_) {
				empty = l
			}
			return v
		},
		unbox: func(v any) *TList[T] {
			if v == any(emptyImmList_) && empty != nil {
				return empty
			}
			return tlistOf_[T](v)
		},
	}
}

// tmapConv_ is the conversion of typed maps as elements of a typed container.
func tmapConv_[K comparable, V any]() *tconv_[*TMap[K, V]] {
	var empty *TMap[K, V] // the empty immutable map boxed last
	return &tconv_[*TMap[K, V]]{
		box: func(m *TMap[K, V]) any {
			if m == nil {
				return nil
			}
			v := m.asMap()
			if v == Map(emptyImmMap_) {
				empty = m
			}
			return v
		},
		unbox: func(v any) *TMap[K, V] {
			if v == any(emptyImmMap_) && empty != nil {
				return empty
			}
			return tmapOf_[K, V](v)
		},
	}
}

// tsetConv_ is the conversion of typed sets as elements of a typed container.
func tsetConv_[K comparable]() *tconv_[*TSet[K]] {
	var empty *TSet[K] // the empty immutable set boxed last
	return &tconv_[*TSet[K]]{
		box: func(s *TSet[K]) any {
			if s == nil {
				return nil
			}
			v := s.asSet()
			if v == Set(emptyImmSet_) {
				empty = s
			}
			return v
		},
		unbox: func(v any) *TSet[K] {
			if v == any(emptyImmSet_) && empty != nil {
				return empty
			}
			return tsetOf_[K](v)
		},
	}
}

// tdequeView_ is a deque that is a typed list (colls -kind deque), for a call that reads it
// from head to tail (a copy, an addAll, a join, a for-each): the list's view where the list
// keeps the deque's order, the view reversed where the list's end is the deque's head (rev).
// Null for a null deque. addAll through the first appends at the tail, as the deque's does.
func tdequeView_[T any](l *TList[T], rev bool) List {
	if l == nil {
		return nil
	}
	if rev {
		return Lists_reverse(l.asList())
	}
	return l.asList()
}
