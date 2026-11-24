package closure

// Typed collections held in typed containers (tools/postflip/colls, nested.go).
//
// A container that converts its elements (the *tconv_[V] of a typed map) boxes a typed
// collection as its view, the one object erased code sees it as (asList, asMap, asSet: a
// collection has one view, so its identity is kept), and unboxes a view to the typed
// collection behind it. Only collections of the class are ever stored there (colls proves
// it), so the view is always one of a typed collection of these types.

// tlistOf_ is the typed list whose view v is (null for null).
func tlistOf_[T any](v any) *TList[T] {
	if v == nil {
		return nil
	}
	return v.(*tlistView_[T]).l
}

// tmapOf_ is the typed map whose view v is.
func tmapOf_[K comparable, V any](v any) *TMap[K, V] {
	if v == nil {
		return nil
	}
	return v.(*tmapView_[K, V]).m
}

// tsetOf_ is the typed set whose view v is.
func tsetOf_[K comparable](v any) *TSet[K] {
	if v == nil {
		return nil
	}
	return v.(*tsetView_[K]).s
}

// tlistConv_ is the conversion of typed lists as elements of a typed container.
func tlistConv_[T any]() *tconv_[*TList[T]] {
	return &tconv_[*TList[T]]{
		box: func(l *TList[T]) any {
			if l == nil {
				return nil
			}
			return l.asList()
		},
		unbox: tlistOf_[T],
	}
}

// tmapConv_ is the conversion of typed maps as elements of a typed container.
func tmapConv_[K comparable, V any]() *tconv_[*TMap[K, V]] {
	return &tconv_[*TMap[K, V]]{
		box: func(m *TMap[K, V]) any {
			if m == nil {
				return nil
			}
			return m.asMap()
		},
		unbox: tmapOf_[K, V],
	}
}

// tsetConv_ is the conversion of typed sets as elements of a typed container.
func tsetConv_[K comparable]() *tconv_[*TSet[K]] {
	return &tconv_[*TSet[K]]{
		box: func(s *TSet[K]) any {
			if s == nil {
				return nil
			}
			return s.asSet()
		},
		unbox: tsetOf_[K],
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
