package closure

// Rule "typed library interfaces", piece `objarg` (tools/j2go LibObj; rule candidate T3 of
// docs/javaisms6/JAVAISMS6.md): the collection methods that Java declares with an Object
// parameter (Map.get / containsKey / remove, Collection.contains / remove, List.indexOf /
// lastIndexOf), called with an argument whose static type is the key or element type, take it
// at that type. Each twin does what the erased call does: where the receiver's object is the view
// of a typed map or set, the lookup that the view's erased method makes after it has taken the key
// out of its box (tkeyOf_: the same find on the same key); else the erased method with the
// argument boxed as the erased code boxes a value of its type (boxT_). A null argument takes the
// erased method in every case (it is nil in both translations). The key type is comparable (the
// translator refuses one that Go cannot compare), so the test of the view is a type assertion
// to the view's own type, and a twin is instantiated for the types of its calls only.

// Map_getK_ is m.get(k) for a key of the map's key type (tmapView_.get_t).
func Map_getK_[K comparable, V any](m MapT[K, V], k K) V {
	if v, ok := m.(*tmapView_[K, V]); ok && k != v.m.kc.null {
		return v.m.get(k)
	}
	return m.get_t(boxT_(k))
}

// Map_containsKeyK_ is m.containsKey(k) for a key of the map's key type (tmapView_.containsKey).
func Map_containsKeyK_[K comparable, V any](m MapT[K, V], k K) bool {
	if v, ok := m.(*tmapView_[K, V]); ok && k != v.m.kc.null {
		return v.m.find(k) >= 0
	}
	return m.containsKey(boxT_(k))
}

// Map_removeK_ is m.remove(k) for a key of the map's key type (tmapView_.remove_t).
func Map_removeK_[K comparable, V any](m MapT[K, V], k K) V {
	if v, ok := m.(*tmapView_[K, V]); ok && k != v.m.kc.null {
		return v.m.remove(k)
	}
	return m.remove_t(boxT_(k))
}

// Collection_containsE_ is c.contains(x) for an element of the collection's element type
// (tsetView_.contains).
func Collection_containsE_[T comparable](c CollectionT[T], x T) bool {
	if v, ok := c.(*tsetView_[T]); ok && x != v.s.c.null {
		return v.s.find(x) >= 0
	}
	return c.contains(boxT_(x))
}

// Collection_removeE_ is c.remove(x) (remove(Object)) for an element of the collection's element
// type (tsetView_.remove_Object).
func Collection_removeE_[T comparable](c CollectionT[T], x T) bool {
	if v, ok := c.(*tsetView_[T]); ok && x != v.s.c.null {
		return v.s.remove(x)
	}
	return c.remove_Object(boxT_(x))
}

// List_indexOfE_ / List_lastIndexOfE_ are l.indexOf(x) / l.lastIndexOf(x) for an element of the
// list's element type. Java compares by the argument's equals: the erased method, with the box.
func List_indexOfE_[T any](l ListT[T], x T) int32     { return l.indexOf(boxT_(x)) }
func List_lastIndexOfE_[T any](l ListT[T], x T) int32 { return l.lastIndexOf(boxT_(x)) }
