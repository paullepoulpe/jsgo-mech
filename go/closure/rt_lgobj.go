package closure

import "unsafe"

// Rule "typed library interfaces", piece `objarg` (tools/j2go LibObj; rule candidate T3 of
// docs/javaisms6/JAVAISMS6.md): Map.get / containsKey / remove and Set.contains / remove, which
// Java declares with an Object parameter, called on a Map or Set receiver with an argument whose
// static type is the key or element type, take it at that type, where the translation has a
// typed map or set of those types (tools/j2go/objarg-pairs.txt). Where the receiver's object is
// the view of a typed map or set, the twin calls the typed map's or set's own lookup, which is
// what the view's erased method does after it has taken the key out of its box (tkeyOf_: the
// same find on the same key); else the erased method with the argument boxed as the erased code
// boxes a value of its type (boxT_). A null argument takes the erased method in every case (it
// is nil in both translations).
//
// The view is found through a method whose signature names no type parameter (tmap_ / tset_):
// a value of MapT[K, V] whose object is a typed map's view is the view of a JTMap[K, V] (its
// put_t takes a K and a V), so the pointer it gives is one. (A type assertion to the view's own
// type would make Go instantiate it with its methods; an interface with K in a method's
// signature would make Go keep that method for every instantiation of the view. The pair list
// keeps the twins to the K, V that have a typed map, so the twins compile no typed map that the
// translation does not have.)

func (v *tmapView_[K, V]) tmap_() unsafe.Pointer { return unsafe.Pointer(v.tm()) }
func (v *tsetView_[K]) tset_() unsafe.Pointer    { return unsafe.Pointer(v.ts()) }

// jtmapOf_ / jtsetOf_: the view of a typed map or set (also of a sorted one, which embeds it).
type jtmapOf_ interface{ tmap_() unsafe.Pointer }
type jtsetOf_ interface{ tset_() unsafe.Pointer }

// Map_getK_ is m.get(k) for a key of the map's key type (tmapView_.get_t).
func Map_getK_[K comparable, V any](m MapT[K, V], k K) V {
	if p, ok := m.(jtmapOf_); ok {
		if t := (*TMap[K, V])(p.tmap_()); k != t.kc.null {
			return t.Get(k)
		}
	}
	return m.get_t(boxT_(k))
}

// Map_containsKeyK_ is m.containsKey(k) for a key of the map's key type (tmapView_.containsKey).
func Map_containsKeyK_[K comparable, V any](m MapT[K, V], k K) bool {
	if p, ok := m.(jtmapOf_); ok {
		if t := (*TMap[K, V])(p.tmap_()); k != t.kc.null {
			return t.find(k) >= 0
		}
	}
	return m.containsKey(boxT_(k))
}

// Map_removeK_ is m.remove(k) for a key of the map's key type (tmapView_.remove_t).
func Map_removeK_[K comparable, V any](m MapT[K, V], k K) V {
	if p, ok := m.(jtmapOf_); ok {
		if t := (*TMap[K, V])(p.tmap_()); k != t.kc.null {
			return t.Delete(k)
		}
	}
	return m.remove_t(boxT_(k))
}

// Set_containsE_ is s.contains(x) for an element of the set's element type (tsetView_.contains).
func Set_containsE_[T comparable](s SetT[T], x T) bool {
	if p, ok := s.(jtsetOf_); ok {
		if t := (*TSet[T])(p.tset_()); x != t.kc.null {
			return t.find(x) >= 0
		}
	}
	return s.contains(boxT_(x))
}

// Set_removeE_ is s.remove(x) (remove(Object)) for an element of the set's element type
// (tsetView_.remove_Object).
func Set_removeE_[T comparable](s SetT[T], x T) bool {
	if p, ok := s.(jtsetOf_); ok {
		if t := (*TSet[T])(p.tset_()); x != t.kc.null {
			return t.Delete(x)
		}
	}
	return s.remove_Object(boxT_(x))
}
