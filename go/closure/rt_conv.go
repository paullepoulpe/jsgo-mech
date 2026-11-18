package closure

import "reflect"

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
		nullFail_(objstr_(canon_(msg)))
	}
	return v
}

func Preconditions_checkNotNull_Object_Object_Str(v string, msg any) string {
	if v == nullstr_ {
		nullFail_(objstr_(canon_(msg)))
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

// ---- Message arguments of checks. The translated code passes an Object argument that
// Preconditions only formats (and only when the check fails) at its own type: a string or a
// pointer to a class. canon_ makes it the object that Java sees: null for the null string and
// for a nil pointer, the most derived object for a pointer to a class with subclasses. It is
// the identity on a value that already is such an object.
func canon_(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		return strobj_(x)
	case interface{ self() any }:
		if rv := reflect.ValueOf(v); rv.Kind() == reflect.Pointer && rv.IsNil() {
			return nil
		}
		return x.self()
	}
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Pointer && rv.IsNil() {
		return nil
	}
	return v
}

// handle_ is a handle class (a Node): its zero value is null, box_ gives the object.
type handle_ interface {
	comparable
	box_() any
}

// A handle as the one message argument of a check is boxed only when the check fails.

func Preconditions_checkState_boolean_Object_H[M handle_](b bool, msg M) {
	if !b {
		stateFail_(objstr_(msg.box_()))
	}
}

func Preconditions_checkArgument_boolean_Object_H[M handle_](b bool, msg M) {
	if !b {
		argFail_(objstr_(msg.box_()))
	}
}

func Preconditions_checkState_boolean_String_Object_H[M handle_](b bool, t string, p1 M) {
	if !b {
		stateFail_(Strings_lenientFormat(t, []any{p1.box_()}))
	}
}

func Preconditions_checkArgument_boolean_String_Object_H[M handle_](b bool, t string, p1 M) {
	if !b {
		argFail_(Strings_lenientFormat(t, []any{p1.box_()}))
	}
}
