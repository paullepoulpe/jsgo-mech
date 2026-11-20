package closure

// Support for collections that are Go slices and maps (post-flip steps 1 and 2).

// sliceEach_ iterates over a slice the way each_ iterates over an ArrayList: it reads the slice
// anew at every step, so elements appended during the loop are visited.
type sliceEach_[T any] struct {
	src *[]T
	i   int
	v   T
}

func eachOf_[T any](s *[]T) sliceEach_[T] { return sliceEach_[T]{src: s} }

func (e *sliceEach_[T]) next() bool {
	if e.i < len(*e.src) {
		e.v = (*e.src)[e.i]
		e.i++
		return true
	}
	return false
}
// has_ reports whether m has key k (Map.containsKey, Set.contains).
func has_[K comparable, V any](m map[K]V, k K) bool {
	_, ok := m[k]
	return ok
}

// setAdd_ adds k to the set and reports whether it was new (the value of Set.add).
func setAdd_[K comparable](m map[K]struct{}, k K) bool {
	if _, ok := m[k]; ok {
		return false
	}
	m[k] = struct{}{}
	return true
}

// mapStr_ is Map.get for String values: an absent key gives Java's null string.
func mapStr_[K comparable](m map[K]string, k K) string {
	if v, ok := m[k]; ok {
		return v
	}
	return nullstr_
}
