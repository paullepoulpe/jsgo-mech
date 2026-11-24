package closure

import "unsafe"

// Piece `cmp` of j2go rule "typed library interfaces" (tools/j2go/src/j2go/LibCmp.java): a class
// of the program that implements Comparable<C> of its own type has compareTo_t(C) at its Java
// types, and the bridge compareTo(any), which casts its argument and calls it (javac's bridge).
// ComparableT is that typed form of java.lang.Comparable.
type ComparableT[T any] interface {
	compareTo_t(T) int32
}

// cmpTypedT_ compares a and b by a's compareTo_t when a is a non-null value of such a class
// (ok), as compareNatural_ of the boxes would through the bridge: the bridge's cast of b is the
// identity there, since b is already a T. A null a (which compareNatural_ reports itself) and a
// value of any other type answer !ok, and the caller compares the boxes as before.
func cmpTypedT_[T any](a, b T) (r int32, ok bool) {
	x := any(a)
	if x == nil || (*eface_)(unsafe.Pointer(&x)).data == nil {
		return 0, false
	}
	if c, ok := x.(ComparableT[T]); ok {
		return c.compareTo_t(b), true
	}
	return 0, false
}
