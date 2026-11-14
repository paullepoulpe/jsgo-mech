package closure

// Support for collections that are Go slices (post-flip step 1).

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