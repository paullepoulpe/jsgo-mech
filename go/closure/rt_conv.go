package closure

// The runtime side of the translator's conversions that need no box (tools/j2go, Conv.java).

// ---- Typed non-null checks: <T> T checkNotNull(T, ...) of Preconditions and <T> T
// requireNonNull(T, ...) of Objects at the argument's own type. _T is for a pointer, an
// interface or a handle (null is the zero value of the type), _Str for a String (null is the
// null string). They throw what the functions taking an object throw.

func Preconditions_checkNotNull_T[T comparable](v T) T {
	var null T
	if v == null {
		panic(new_NullPointerException_0().self())
	}
	return v
}

func Preconditions_checkNotNull_Str(v string) string {
	if v == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	return v
}

func Preconditions_checkNotNull_Object_Object_T[T comparable](v T, msg any) T {
	var null T
	if v == null {
		nullFail_(objstr_(msg))
	}
	return v
}

func Preconditions_checkNotNull_Object_Object_Str(v string, msg any) string {
	if v == nullstr_ {
		nullFail_(objstr_(msg))
	}
	return v
}

func Preconditions_checkNotNull_Object_String_Object_T[T comparable](v T, t string, p1 any) T {
	var null T
	if v == null {
		nullFail_(Strings_lenientFormat(t, []any{p1}))
	}
	return v
}

func Preconditions_checkNotNull_Object_String_Object_Str(v string, t string, p1 any) string {
	if v == nullstr_ {
		nullFail_(Strings_lenientFormat(t, []any{p1}))
	}
	return v
}

func Preconditions_checkNotNull_Object_String_Object_Object_T[T comparable](v T, t string, p1 any, p2 any) T {
	var null T
	if v == null {
		nullFail_(Strings_lenientFormat(t, []any{p1, p2}))
	}
	return v
}

func Preconditions_checkNotNull_Object_String_Object_Object_Str(v string, t string, p1 any, p2 any) string {
	if v == nullstr_ {
		nullFail_(Strings_lenientFormat(t, []any{p1, p2}))
	}
	return v
}

func Objects_requireNonNull_T[T comparable](v T, msg string) T {
	var null T
	if v == null {
		panic(new_NullPointerException_String(msg).self())
	}
	return v
}

func Objects_requireNonNull_Str(v string, msg string) string {
	if v == nullstr_ {
		panic(new_NullPointerException_String(msg).self())
	}
	return v
}

func Objects_requireNonNull_Object_T[T comparable](v T) T {
	var null T
	if v == null {
		panic(new_NullPointerException_0().self())
	}
	return v
}

func Objects_requireNonNull_Object_Str(v string) string {
	if v == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	return v
}
