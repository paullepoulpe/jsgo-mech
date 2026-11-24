package closure

// Rule "typed library interfaces", piece `nbox` (tools/j2go LibNbox; J2GO_NOLIBGEN=nbox is the
// translation without it, which names nothing of this file): a field or parameter that Java
// declares as a boxed primitive (Boolean, Integer, ...; a value that may be null, as the
// program's tri-state flags), and the boxed type arguments of a function type given for one
// (Consumer[Nbox[bool]]), hold the primitive and a flag instead of an `any` with the box. The
// zero value is Java's null. Unboxing null throws NullPointerException as Java's does; the box_
// of a value is the object erased code knows for that box (the bool, the int32 ...).
type Nbox[T any] struct {
	v  T
	ok bool // not null
}

// nbox_ is a primitive value boxed (never null).
func nbox_[T any](v T) Nbox[T] { return Nbox[T]{v, true} }

// nboxOf_ is an Object (a box or nil) as an Nbox[T]: nil is null, a box of another type the
// unboxing cast's exception.
func nboxOf_[T any](v any) Nbox[T] {
	if v == nil {
		return Nbox[T]{}
	}
	return Nbox[T]{unbox_[T](v), true}
}

// get_ is the unboxing: NullPointerException for null.
func (n Nbox[T]) get_() T {
	if !n.ok {
		panic(new_NullPointerException_0().self())
	}
	return n.v
}

// null_ is `n == null`.
func (n Nbox[T]) null_() bool { return !n.ok }

// box_ is the value as an Object: nil for null, else the box.
func (n Nbox[T]) box_() any {
	if !n.ok {
		return nil
	}
	return any(n.v)
}
