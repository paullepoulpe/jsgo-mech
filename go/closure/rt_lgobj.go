package closure

// Rule "typed library interfaces", piece `objarg` (tools/j2go LibObj; rule candidate T3 of
// docs/javaisms6/JAVAISMS6.md): the collection methods that Java declares with an Object
// parameter (Map.get / containsKey / remove, Collection.contains / remove, List.indexOf /
// lastIndexOf), called with an argument whose static type is the key or element type, take it
// at that type. Each twin does what the erased call does: where the receiver's object is the view
// of a typed collection, the lookup that its erased method makes after it has taken the key out
// of its box (the view's method *_k below: the same find on the same key); else the erased method
// with the argument boxed as the erased code boxes a value of its type (boxT_). A null argument
// takes the erased method in every case (it is nil in both translations).

// Map_getK_ is m.get(k) for a key of the map's key type.
func Map_getK_[K, V any](m MapT[K, V], k K) V {
	if t, ok := m.(interface{ get_k(K) V }); ok {
		return t.get_k(k)
	}
	return m.get_t(boxT_(k))
}

// Map_containsKeyK_ is m.containsKey(k) for a key of the map's key type.
func Map_containsKeyK_[K, V any](m MapT[K, V], k K) bool {
	if t, ok := m.(interface{ containsKey_k(K) bool }); ok {
		return t.containsKey_k(k)
	}
	return m.containsKey(boxT_(k))
}

// Map_removeK_ is m.remove(k) for a key of the map's key type.
func Map_removeK_[K, V any](m MapT[K, V], k K) V {
	if t, ok := m.(interface{ remove_k(K) V }); ok {
		return t.remove_k(k)
	}
	return m.remove_t(boxT_(k))
}

// Collection_containsE_ is c.contains(x) for an element of the collection's element type.
func Collection_containsE_[T any](c CollectionT[T], x T) bool {
	if t, ok := c.(interface{ contains_k(T) bool }); ok {
		return t.contains_k(x)
	}
	return c.contains(boxT_(x))
}

// Collection_removeE_ is c.remove(x) (remove(Object)) for an element of the collection's element type.
func Collection_removeE_[T any](c CollectionT[T], x T) bool {
	if t, ok := c.(interface{ remove_k(T) bool }); ok {
		return t.remove_k(x)
	}
	return c.remove_Object(boxT_(x))
}

// List_indexOfE_ / List_lastIndexOfE_ are l.indexOf(x) / l.lastIndexOf(x) for an element of the
// list's element type. Java compares by the argument's equals: the erased method, with the box.
func List_indexOfE_[T any](l ListT[T], x T) int32     { return l.indexOf(boxT_(x)) }
func List_lastIndexOfE_[T any](l ListT[T], x T) int32 { return l.lastIndexOf(boxT_(x)) }

// The typed lookups of the views of the typed collections: what get_t / containsKey / remove_t /
// contains / remove_Object do with the key they take out of its box (tkeyOf_), for that key.

func (v *tmapView_[K, V]) get_k(k K) V {
	if k == v.m.kc.null {
		return v.get_t(nil)
	}
	if i := v.m.find(k); i >= 0 {
		return v.m.vals[i]
	}
	return v.m.vc.null
}

func (v *tmapView_[K, V]) containsKey_k(k K) bool {
	if k == v.m.kc.null {
		return v.containsKey(nil)
	}
	return v.m.find(k) >= 0
}

func (v *tmapView_[K, V]) remove_k(k K) V {
	if k == v.m.kc.null {
		return v.remove_t(nil)
	}
	return v.m.remove(k)
}

func (v *tsetView_[K]) contains_k(k K) bool {
	if k == v.s.c.null {
		return v.contains(nil)
	}
	return v.s.find(k) >= 0
}

func (v *tsetView_[K]) remove_k(k K) bool {
	if k == v.s.c.null {
		return v.remove_Object(nil)
	}
	return v.s.remove(k)
}

func (v *tmapKeySet_[K, V]) contains_k(k K) bool {
	if k == v.m.kc.null {
		return v.contains(nil)
	}
	return v.m.find(k) >= 0
}

func (v *tmapKeySet_[K, V]) remove_k(k K) bool {
	if k == v.m.kc.null {
		return v.remove_Object(nil)
	}
	i := v.m.find(k)
	if i < 0 {
		return false
	}
	v.m.delAt(int(i))
	return true
}
