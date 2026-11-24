//go:build !j2go_nofntypes

package closure

// Piece `nboxarg` (tools/j2go LibFns, LibNbox): boxed type arguments of function types at Nbox.

// Comparator_comparingN_ is Comparator.comparing(Function) with a key that is a boxed integral
// primitive (piece `nboxarg`: the function gives Nbox[P]): the keys compared as the erased
// Comparator.comparing compares their boxes (compareNatural_: Integer.compareTo, a null key
// throws there).
func Comparator_comparingN_[T any, P int8 | int16 | int32 | int64 | uint16](f Function[T, Nbox[P]]) Comparator[T] {
	return func(a, b T) int32 {
		ka, kb := f(a), f(b)
		if !ka.ok || !kb.ok {
			return compareNatural_(ka.box_(), kb.box_())
		}
		switch {
		case ka.v < kb.v:
			return -1
		case ka.v > kb.v:
			return 1
		}
		return 0
	}
}
