package closure

// Helpers of the post-flip step eqhash: equals and hashCode calls whose static types decide
// what they do. Each is what Object_equals, Objects_equals or Objects_hashCode does for a value
// of that type, without the dynamic dispatch on the boxed value.

// equalsId_ is a.equals(b) for a class that does not override equals: identity, and a
// NullPointerException when a is null.
func equalsId_[T comparable](a, b T) bool {
	var zero T
	if a == zero {
		panic(new_NullPointerException_0().self())
	}
	return a == b
}

// strequals_ is a.equals(b) for two strings: a NullPointerException when a is null.
func strequals_(a, b string) bool {
	if a == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	return a == b
}

// equalsBy_ is a.equals(b) for a class whose equals method is eq: a NullPointerException when
// a is null.
func equalsBy_[T comparable](a T, b any, eq func(T, any) bool) bool {
	var zero T
	if a == zero {
		panic(new_NullPointerException_0().self())
	}
	return eq(a, b)
}

// objequalsBy_ is Objects.equals(a, b) for a class whose equals method is eq.
func objequalsBy_[T comparable](a T, b any, eq func(T, any) bool) bool {
	var zero T
	if a == zero || b == nil {
		return a == zero && b == nil
	}
	return eq(a, b)
}

// objhashBy_ is Objects.hashCode(a) for a class whose hashCode method is h.
func objhashBy_[T comparable](a T, h func(T) int32) int32 {
	var zero T
	if a == zero {
		return 0
	}
	return h(a)
}

// objhashId_ is Objects.hashCode(a) for a class without a hashCode method: the identity hash.
func objhashId_[T comparable](a T) int32 {
	var zero T
	if a == zero {
		return 0
	}
	return identityHash_(a)
}

// objhashStr_ is Objects.hashCode(s) for a string.
func objhashStr_(s string) int32 {
	if s == nullstr_ {
		return 0
	}
	return String_hashCode(s)
}

// Boolean_hashCode is Boolean.hashCode(boolean).
func Boolean_hashCode(b bool) int32 {
	if b {
		return 1231
	}
	return 1237
}

// equaler_ and hasher_ are the dynamic types that have equals or hashCode.
type equaler_ interface{ equals(any) bool }
type hasher_ interface{ hashCode() int32 }

// equalsVia_ is a.equals(b) for a value of an interface type every implementation of which has
// an equals method that the interface does not declare: a NullPointerException when a is null.
func equalsVia_[T comparable](a T, b any) bool {
	var zero T
	if a == zero {
		panic(new_NullPointerException_0().self())
	}
	return any(a).(equaler_).equals(b)
}

// objequalsVia_ is Objects.equals(a, b) for such a value.
func objequalsVia_[T comparable](a T, b any) bool {
	var zero T
	if a == zero || b == nil {
		return a == zero && b == nil
	}
	return any(a).(equaler_).equals(b)
}

// objhashVia_ is Objects.hashCode(a) for a value of an interface type every implementation of
// which has a hashCode method that the interface does not declare.
func objhashVia_[T comparable](a T) int32 {
	var zero T
	if a == zero {
		return 0
	}
	return any(a).(hasher_).hashCode()
}

// equalsIdAny_ is a.equals(b) for a class that does not override equals, against a value of
// type any: the boxes compared, as Object_equals compares them; a NullPointerException when a
// is null.
func equalsIdAny_[T comparable](a T, b any) bool {
	var zero T
	if a == zero {
		panic(new_NullPointerException_0().self())
	}
	return any(a) == b
}

// optequals_ is Objects.equals(a, b) for two Optionals, without boxing them.
func optequals_[T any](a, b Optional[T]) bool {
	if !a.set || !b.set {
		return a.set == b.set
	}
	return objequals_(a.content_(), b.content_())
}

// hashBy_ is a.hashCode() for a class whose hashCode method is h: a NullPointerException when
// a is null.
func hashBy_[T comparable](a T, h func(T) int32) int32 {
	var zero T
	if a == zero {
		panic(new_NullPointerException_0().self())
	}
	return h(a)
}

// hashId_ is a.hashCode() for a class without a hashCode method: the identity hash, and a
// NullPointerException when a is null.
func hashId_[T comparable](a T) int32 {
	var zero T
	if a == zero {
		panic(new_NullPointerException_0().self())
	}
	return identityHash_(a)
}

// hashOf_ is Objects.hash of values whose hash codes are hs (Arrays.hashCode).
func hashOf_(hs ...int32) int32 {
	var h int32 = 1
	for _, x := range hs {
		h = 31*h + x
	}
	return h
}

// requireNonNull_ is Objects.requireNonNull(v, msg) for a value whose zero value is null.
func requireNonNull_[T comparable](v T, msg string) T {
	var zero T
	if v == zero {
		panic(new_NullPointerException_String(msg).self())
	}
	return v
}

// requireNonNull0_ is Objects.requireNonNull(v).
func requireNonNull0_[T comparable](v T) T {
	var zero T
	if v == zero {
		panic(new_NullPointerException_0().self())
	}
	return v
}

// hashVia_ is a.hashCode() for such a value: a NullPointerException when a is null.
func hashVia_[T comparable](a T) int32 {
	var zero T
	if a == zero {
		panic(new_NullPointerException_0().self())
	}
	return any(a).(hasher_).hashCode()
}

// equalsDyn_ is a.equals(b) for a value of an interface type some implementations of which
// have an equals method and the others none (all comparable): the method, else identity, as
// Object_equals decides; a NullPointerException when a is null.
func equalsDyn_[T comparable](a T, b any) bool {
	var zero T
	if a == zero {
		panic(new_NullPointerException_0().self())
	}
	if e, ok := any(a).(equaler_); ok {
		return e.equals(b)
	}
	return any(a) == b
}

// objequalsDyn_ is Objects.equals(a, b) for such a value.
func objequalsDyn_[T comparable](a T, b any) bool {
	var zero T
	if a == zero || b == nil {
		return a == zero && b == nil
	}
	if e, ok := any(a).(equaler_); ok {
		return e.equals(b)
	}
	return any(a) == b
}

// objhashDyn_ is Objects.hashCode(a) for a value of an interface type some implementations of
// which have a hashCode method and the others none: the method, else the identity hash.
func objhashDyn_[T comparable](a T) int32 {
	var zero T
	if a == zero {
		return 0
	}
	if h, ok := any(a).(hasher_); ok {
		return h.hashCode()
	}
	return identityHash_(a)
}

// hashDyn_ is a.hashCode() for such a value: a NullPointerException when a is null.
func hashDyn_[T comparable](a T) int32 {
	var zero T
	if a == zero {
		panic(new_NullPointerException_0().self())
	}
	if h, ok := any(a).(hasher_); ok {
		return h.hashCode()
	}
	return identityHash_(a)
}

// optequalsOn_ is a.equals(b) for two Optionals: a NullPointerException when a is null.
func optequalsOn_[T any](a, b Optional[T]) bool {
	a.nn_()
	return b.set && objequals_(a.content_(), b.content_())
}

// objhashHandle_ is Objects.hashCode(a) for a handle (Node): the identity hash of its number,
// as identityHash_ takes it from the box, without boxing it.
func objhashHandle_[T interface {
	comparable
	handle_() uint32
}](a T) int32 {
	var zero T
	if a == zero {
		return 0
	}
	return handleHash_(a.handle_())
}

// hashHandle_ is a.hashCode() for a handle: a NullPointerException when a is null.
func hashHandle_[T interface {
	comparable
	handle_() uint32
}](a T) int32 {
	var zero T
	if a == zero {
		panic(new_NullPointerException_0().self())
	}
	return handleHash_(a.handle_())
}

// optOfEquals_ is Optional.of(s).equals(o) for a String s and an Optional<String> o, without
// boxing either: a NullPointerException when s is null (Optional.of), false when o is null or
// empty, else String.equals of the contents (an Optional never holds a null String).
func optOfEquals_(s string, o Optional[string]) bool {
	if s == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	return o.set && o.present && o.value == s
}

// strNonNull_ is the null check of Optional.of(s) for a String s, made where Java makes it:
// as the first argument of optOfEquals_ it throws before the second argument is computed (Go
// evaluates the calls of an argument list from left to right).
func strNonNull_(s string) string {
	if s == nullstr_ {
		panic(new_NullPointerException_0())
	}
	return s
}

// optEqualsOf_ is o.equals(Optional.of(s)): the NullPointerException of Optional.of when s is
// null, then that of the call on o when o is null.
func optEqualsOf_(o Optional[string], s string) bool {
	if s == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	o.nn_()
	return o.present && o.value == s
}
